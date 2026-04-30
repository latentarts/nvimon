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
    return
  fi

  run_as_root install -m 0644 "${source_config}" "${TARGET_CONFIG}"
  log "installed default config to ${TARGET_CONFIG}"
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
  local unit_changed=0

  if install_or_update_binary "${artifact_path}"; then
    binary_changed=1
  fi
  install_config_if_missing "${config_source}"
  if install_or_update_service_unit; then
    unit_changed=1
  fi

  if [[ ${binary_changed} -eq 1 || ${unit_changed} -eq 1 ]] || ! service_is_active; then
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
