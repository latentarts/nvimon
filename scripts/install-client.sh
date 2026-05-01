#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "${SCRIPT_DIR}/.." && pwd)

DIST_DIR=${DIST_DIR:-"${REPO_ROOT}/dist"}
INSTALL_BIN_DIR=${INSTALL_BIN_DIR:-"${HOME}/.local/bin"}
CONFIG_HOME=${XDG_CONFIG_HOME:-"${HOME}/.config"}
TARGET_BIN="${INSTALL_BIN_DIR}/nvimon"
TARGET_CONFIG_DIR="${CONFIG_HOME}/nvimon"
TARGET_CONFIG="${TARGET_CONFIG_DIR}/config.yaml"

log() {
  printf '[nvimon-client-install] %s\n' "$*"
}

fail() {
  printf '[nvimon-client-install] error: %s\n' "$*" >&2
  exit 1
}

have_command() {
  command -v "$1" >/dev/null 2>&1
}

can_execute_binary() {
  local binary_path=$1
  [[ -x "${binary_path}" ]] || return 1
  "${binary_path}" --help >/dev/null 2>&1
}

select_cli_artifact() {
  local nvml_path="${DIST_DIR}/local-nvml/nvimon"
  local portable_path="${DIST_DIR}/portable/nvimon"

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

  fail "no compatible nvimon artifact found under ${DIST_DIR}"
}

install_binary() {
  local source_binary=$1
  install -d "${INSTALL_BIN_DIR}"
  install -m 0755 "${source_binary}" "${TARGET_BIN}"
  log "installed binary to ${TARGET_BIN}"
}

install_default_config_if_missing() {
  install -d "${TARGET_CONFIG_DIR}"

  if [[ -f "${TARGET_CONFIG}" ]]; then
    log "keeping existing config at ${TARGET_CONFIG}"
    return 1
  fi

  cat >"${TARGET_CONFIG}" <<'EOF'
refresh_interval: 1s
history_length: 120

timeouts:
  connect: 2s
  request: 2s

agent:
  bind_address: 127.0.0.1:9910
  auth_token: ""

hosts:
  - name: localhost
    mode: local
EOF

  log "installed default config to ${TARGET_CONFIG}"
  return 0
}

print_path_hint() {
  case ":${PATH}:" in
    *":${INSTALL_BIN_DIR}:"*)
      return
      ;;
  esac

  log "add ${INSTALL_BIN_DIR} to PATH if you want to run nvimon without a full path"
}

main() {
  have_command install || fail "install is required"

  local artifact_path
  artifact_path=$(select_cli_artifact)

  install_binary "${artifact_path}"
  local config_created=0
  if install_default_config_if_missing; then
    config_created=1
  fi
  print_path_hint

  log "binary: ${TARGET_BIN}"
  log "config: ${TARGET_CONFIG}"
  if [[ ${config_created} -eq 1 ]]; then
    log "edit ${TARGET_CONFIG} to add remote hosts or change defaults"
  else
    log "review ${TARGET_CONFIG} if you need to change hosts or timeouts"
  fi
  log "test: ${TARGET_BIN} --once"
}

main "$@"
