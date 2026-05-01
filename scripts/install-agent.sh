#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "${SCRIPT_DIR}/.." && pwd)

DIST_DIR=${DIST_DIR:-"${REPO_ROOT}/dist"}
SERVICE_TEMPLATE=${SERVICE_TEMPLATE:-"${REPO_ROOT}/packaging/systemd/nvimon-agent.service"}

INSTALL_BIN_DIR=${INSTALL_BIN_DIR:-/usr/local/bin}
INSTALL_ETC_DIR=${INSTALL_ETC_DIR:-/etc/nvimon}
INSTALL_STATE_DIR=${INSTALL_STATE_DIR:-/var/lib/nvimon}
SYSTEMD_UNIT_DIR=${SYSTEMD_UNIT_DIR:-/etc/systemd/system}
SERVICE_NAME=${SERVICE_NAME:-nvimon-agent}
PROMPT_FOR_BIND_ADDRESS=${PROMPT_FOR_BIND_ADDRESS:-1}

TARGET_BIN="${INSTALL_BIN_DIR}/nvimon-agent"
TARGET_CONFIG="${INSTALL_ETC_DIR}/config.yaml"
TARGET_UNIT="${SYSTEMD_UNIT_DIR}/${SERVICE_NAME}.service"

log() {
  printf '[nvimon-agent-install] %s\n' "$*"
}

fail() {
  printf '[nvimon-agent-install] error: %s\n' "$*" >&2
  exit 1
}

have_command() {
  command -v "$1" >/dev/null 2>&1
}

have_tty() {
  [[ -t 0 ]] || [[ -r /dev/tty ]]
}

files_differ() {
  local left=$1
  local right=$2
  if [[ ! -f "${left}" || ! -f "${right}" ]]; then
    return 0
  fi
  ! cmp -s "${left}" "${right}"
}

run_as_root() {
  if [[ ${EUID} -eq 0 ]]; then
    "$@"
    return
  fi

  if have_command sudo; then
    sudo "$@"
    return
  fi

  fail "this installer needs root privileges; rerun as root or install sudo"
}

can_execute_binary() {
  local binary_path=$1
  [[ -x "${binary_path}" ]] || return 1
  "${binary_path}" --help >/dev/null 2>&1
}

select_agent_artifact() {
  local nvml_path="${DIST_DIR}/local-nvml/nvimon-agent"
  local portable_path="${DIST_DIR}/portable/nvimon-agent"

  if [[ -x "${nvml_path}" ]] && can_execute_binary "${nvml_path}"; then
    printf '%s\n' "${nvml_path}"
    return
  fi

  if [[ -x "${portable_path}" ]] && can_execute_binary "${portable_path}"; then
    printf '%s\n' "${portable_path}"
    return
  fi

  if [[ -x "${portable_path}" ]]; then
    fail "portable artifact exists but could not execute; verify dist artifacts were built for this host"
  fi

  fail "no compatible nvimon-agent artifact found under ${DIST_DIR}"
}

select_config_source() {
  local dist_config="${DIST_DIR}/nvimon.config.yaml"
  local example_config="${REPO_ROOT}/config.example.yaml"

  if [[ -f "${dist_config}" ]]; then
    printf '%s\n' "${dist_config}"
    return
  fi

  if [[ -f "${example_config}" ]]; then
    printf '%s\n' "${example_config}"
    return
  fi

  fail "no config template found in dist/ or repo root"
}

read_config_bind_address() {
  local config_path=$1
  if [[ ! -f "${config_path}" ]]; then
    return 1
  fi

  sed -n 's/^[[:space:]]*bind_address:[[:space:]]*//p' "${config_path}" | head -n 1
}

install_binary() {
  local source_binary=$1
  run_as_root install -d "${INSTALL_BIN_DIR}"
  run_as_root install -m 0755 "${source_binary}" "${TARGET_BIN}"
}

install_config_if_missing() {
  local source_config=$1
  run_as_root install -d "${INSTALL_ETC_DIR}"

  if run_as_root test -f "${TARGET_CONFIG}"; then
    log "keeping existing config at ${TARGET_CONFIG}"
    return 1
  fi

  run_as_root install -m 0644 "${source_config}" "${TARGET_CONFIG}"
  log "installed default config to ${TARGET_CONFIG}"
  return 0
}

list_bind_ip_candidates() {
  local current_host=$1

  {
    printf '127.0.0.1\n'
    printf '0.0.0.0\n'
    if [[ -n "${current_host}" ]]; then
      printf '%s\n' "${current_host}"
    fi
    if have_command hostname; then
      hostname -I 2>/dev/null | tr ' ' '\n'
    fi
    if have_command ip; then
      ip -o -4 addr show scope global 2>/dev/null | awk '{print $4}' | cut -d/ -f1
    fi
  } | awk 'NF { if (!seen[$0]++) print $0 }'
}

prompt_for_bind_address() {
  local current_bind=$1
  local bind_host=${current_bind%:*}
  local bind_port=${current_bind##*:}

  if [[ -n "${AGENT_BIND_ADDRESS:-}" ]]; then
    printf '%s\n' "${AGENT_BIND_ADDRESS}"
    return
  fi

  if [[ "${PROMPT_FOR_BIND_ADDRESS}" != "1" ]] || ! have_tty; then
    printf '%s\n' "${current_bind}"
    return
  fi

  local tty_path=/dev/tty
  local selected_host=""
  local selected_port=""
  local -a candidates=()
  local idx=1
  local line=""

  while IFS= read -r line; do
    candidates+=("${line}")
  done < <(list_bind_ip_candidates "${bind_host}")

  printf '\n[nvimon-agent-install] available listen IPs:\n' >"${tty_path}"
  for line in "${candidates[@]}"; do
    printf '  %d) %s\n' "${idx}" "${line}" >"${tty_path}"
    idx=$((idx + 1))
  done
  printf '[nvimon-agent-install] current bind address: %s\n' "${current_bind}" >"${tty_path}"

  while true; do
    printf '[nvimon-agent-install] select listen IP or enter a custom value [%s]: ' "${bind_host}" >"${tty_path}"
    IFS= read -r selected_host <"${tty_path}" || selected_host=""
    if [[ -z "${selected_host}" ]]; then
      selected_host=${bind_host}
    elif [[ "${selected_host}" =~ ^[0-9]+$ ]] && (( selected_host >= 1 && selected_host <= ${#candidates[@]} )); then
      selected_host=${candidates[$((selected_host - 1))]}
    fi
    if [[ -n "${selected_host}" ]]; then
      break
    fi
  done

  while true; do
    printf '[nvimon-agent-install] listen port [%s]: ' "${bind_port}" >"${tty_path}"
    IFS= read -r selected_port <"${tty_path}" || selected_port=""
    if [[ -z "${selected_port}" ]]; then
      selected_port=${bind_port}
    fi
    if [[ "${selected_port}" =~ ^[0-9]+$ ]] && (( selected_port >= 1 && selected_port <= 65535 )); then
      break
    fi
    printf '[nvimon-agent-install] invalid port: %s\n' "${selected_port}" >"${tty_path}"
  done

  printf '%s:%s\n' "${selected_host}" "${selected_port}"
}

update_bind_address() {
  local desired_bind=$1
  local current_bind=$2
  local tmp_config

  if [[ "${desired_bind}" == "${current_bind}" ]]; then
    log "keeping bind address ${desired_bind}"
    return 1
  fi

  tmp_config=$(mktemp)
  trap 'rm -f "${tmp_config}"' RETURN

  awk -v bind="${desired_bind}" '
    /^[[:space:]]*bind_address:[[:space:]]*/ && !done {
      indent = substr($0, 1, match($0, /bind_address:/) - 1)
      print indent "bind_address: " bind
      done = 1
      next
    }
    { print }
    END {
      if (!done) {
        print ""
        print "agent:"
        print "  bind_address: " bind
      }
    }
  ' "${TARGET_CONFIG}" > "${tmp_config}"

  run_as_root install -m 0644 "${tmp_config}" "${TARGET_CONFIG}"
  log "updated bind address to ${desired_bind} in ${TARGET_CONFIG}"
  return 0
}

service_exists() {
  run_as_root test -f "${TARGET_UNIT}"
}

service_is_active() {
  run_as_root systemctl is-active --quiet "${SERVICE_NAME}"
}

stop_service_if_running() {
  if service_is_active; then
    log "stopping running service ${SERVICE_NAME}"
    run_as_root systemctl stop "${SERVICE_NAME}"
  fi
}

enable_and_restart_service() {
  run_as_root install -d "${INSTALL_STATE_DIR}"
  run_as_root systemctl daemon-reload
  run_as_root systemctl enable "${SERVICE_NAME}"
  run_as_root systemctl restart "${SERVICE_NAME}"
}

verify_service_running() {
  if service_is_active; then
    return
  fi
  fail "service ${SERVICE_NAME} did not become active after restart; inspect with: systemctl status ${SERVICE_NAME}"
}

install_or_update_binary() {
  local source_binary=$1
  if [[ -f "${TARGET_BIN}" ]] && ! files_differ "${source_binary}" "${TARGET_BIN}"; then
    log "binary already current at ${TARGET_BIN}"
    return 1
  fi

  stop_service_if_running
  install_binary "${source_binary}"
  log "installed binary to ${TARGET_BIN}"
  return 0
}

install_or_update_service_unit() {
  local rendered_unit
  local tmp_unit

  rendered_unit=$(sed \
    -e "s|@BINARY_PATH@|${TARGET_BIN}|g" \
    -e "s|@CONFIG_PATH@|${TARGET_CONFIG}|g" \
    "${SERVICE_TEMPLATE}")

  tmp_unit=$(mktemp)
  trap 'rm -f "${tmp_unit}"' RETURN
  printf '%s\n' "${rendered_unit}" > "${tmp_unit}"

  if [[ -f "${TARGET_UNIT}" ]] && ! files_differ "${tmp_unit}" "${TARGET_UNIT}"; then
    log "service unit already current at ${TARGET_UNIT}"
    return 1
  fi

  run_as_root install -d "${SYSTEMD_UNIT_DIR}"
  run_as_root install -m 0644 "${tmp_unit}" "${TARGET_UNIT}"
  log "installed service unit to ${TARGET_UNIT}"
  return 0
}

print_summary() {
  local artifact_path=$1
  local action=$2
  local variant="portable"
  if [[ "${artifact_path}" == *"/local-nvml/"* ]]; then
    variant="local-nvml"
  fi

  log "${action} ${variant} agent binary at ${TARGET_BIN}"
  log "service: ${SERVICE_NAME}"
  log "config: ${TARGET_CONFIG}"
  log "status: systemctl status ${SERVICE_NAME}"
}

main() {
  have_command systemctl || fail "systemctl is required"
  have_command install || fail "install is required"
  have_command sed || fail "sed is required"
  have_command cmp || fail "cmp is required"
  have_command mktemp || fail "mktemp is required"
  [[ -f "${SERVICE_TEMPLATE}" ]] || fail "service template not found: ${SERVICE_TEMPLATE}"

  local artifact_path
  artifact_path=$(select_agent_artifact)

  local config_source
  config_source=$(select_config_source)
  local had_existing_install=0
  if [[ -f "${TARGET_BIN}" ]] || service_exists; then
    had_existing_install=1
  fi

  local binary_changed=0
  local config_changed=0
  local unit_changed=0

  if install_or_update_binary "${artifact_path}"; then
    binary_changed=1
  fi
  if install_config_if_missing "${config_source}"; then
    config_changed=1
  fi

  local current_bind
  current_bind=$(read_config_bind_address "${TARGET_CONFIG}" || true)
  if [[ -z "${current_bind}" ]]; then
    current_bind=$(read_config_bind_address "${config_source}" || true)
  fi
  if [[ -z "${current_bind}" ]]; then
    current_bind="0.0.0.0:9910"
  fi

  local desired_bind
  desired_bind=$(prompt_for_bind_address "${current_bind}")
  if update_bind_address "${desired_bind}" "${current_bind}"; then
    config_changed=1
  fi

  if install_or_update_service_unit; then
    unit_changed=1
  fi

  if [[ ${binary_changed} -eq 1 || ${config_changed} -eq 1 || ${unit_changed} -eq 1 ]] || ! service_is_active; then
    enable_and_restart_service
  else
    log "service ${SERVICE_NAME} already running with current binary and unit"
  fi

  verify_service_running

  local action="installed"
  if [[ ${had_existing_install} -eq 1 && ${binary_changed} -eq 0 && ${unit_changed} -eq 0 ]]; then
    action="verified"
  elif [[ ${had_existing_install} -eq 1 ]]; then
    action="updated"
  fi

  print_summary "${artifact_path}" "${action}"
}

main "$@"
