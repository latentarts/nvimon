#!/usr/bin/env bash
set -euo pipefail

MODE=${1:-}

REPO_OWNER=${REPO_OWNER:-latentarts}
REPO_NAME=${REPO_NAME:-nvimon}
NVIMON_REF=${NVIMON_REF:-latest}
TMP_ROOT=${TMPDIR:-/tmp}

log() {
  printf '[nvimon-remote-install] %s\n' "$*"
}

fail() {
  printf '[nvimon-remote-install] error: %s\n' "$*" >&2
  exit 1
}

have_command() {
  command -v "$1" >/dev/null 2>&1
}

usage() {
  cat <<'EOF'
usage: remote-install.sh client|server

Downloads the latest portable nvimon release archive, extracts the binaries,
and runs the matching installer.
EOF
}

download_to_path() {
  local url=$1
  local dest_path=$2

  if have_command curl; then
    curl -fsSL "${url}" -o "${dest_path}"
    return
  fi

  if have_command wget; then
    wget -qO "${dest_path}" "${url}"
    return
  fi

  fail "curl or wget is required to download ${url}"
}

fetch_release_metadata() {
  local metadata_path=$1
  local release_url

  if [[ "${NVIMON_REF}" == "latest" ]]; then
    release_url="https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest"
  else
    release_url="https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/tags/${NVIMON_REF}"
  fi

  download_to_path "${release_url}" "${metadata_path}"
}

find_portable_asset_url() {
  local metadata_path=$1
  local asset_url

  asset_url=$(sed -n 's/^[[:space:]]*"browser_download_url":[[:space:]]*"\([^"]*nvimon_[^"]*_linux_amd64_portable\.tar\.gz\)".*/\1/p' "${metadata_path}" | head -n 1)
  [[ -n "${asset_url}" ]] || fail "could not find portable linux amd64 release asset in GitHub release metadata"
  printf '%s\n' "${asset_url}"
}

download_release_archive() {
  local dest_path=$1
  local metadata_path=$2
  local asset_url

  fetch_release_metadata "${metadata_path}"
  asset_url=$(find_portable_asset_url "${metadata_path}")
  log "downloading portable release archive from ${asset_url}"
  download_to_path "${asset_url}" "${dest_path}"
}

find_release_bundle_dir() {
  local work_dir=$1
  local bundle_dir

  bundle_dir=$(find "${work_dir}" -mindepth 1 -maxdepth 1 -type d -name 'nvimon_*_linux_amd64_portable' | head -n 1)
  [[ -n "${bundle_dir}" ]] || fail "downloaded archive did not contain a portable release bundle directory"
  printf '%s\n' "${bundle_dir}"
}

prepare_installer_layout() {
  local bundle_dir=$1
  local install_root=$2

  install -d "${install_root}/dist/portable"
  install -d "${install_root}/scripts"
  install -d "${install_root}/packaging/systemd"

  install -m 0755 "${bundle_dir}/nvimon" "${install_root}/dist/portable/nvimon"
  install -m 0755 "${bundle_dir}/nvimon-agent" "${install_root}/dist/portable/nvimon-agent"
  install -m 0755 "${bundle_dir}/install-client.sh" "${install_root}/scripts/install-client.sh"
  install -m 0755 "${bundle_dir}/install-agent.sh" "${install_root}/scripts/install-agent.sh"
  install -m 0644 "${bundle_dir}/config.example.yaml" "${install_root}/config.example.yaml"
  install -m 0644 "${bundle_dir}/nvimon-agent.service" "${install_root}/packaging/systemd/nvimon-agent.service"
}

main() {
  case "${MODE}" in
    client|server)
      ;;
    *)
      usage >&2
      exit 1
      ;;
  esac

  have_command find || fail "find is required"
  have_command install || fail "install is required"
  have_command mktemp || fail "mktemp is required"
  have_command sed || fail "sed is required"
  have_command tar || fail "tar is required"

  local work_dir archive_path metadata_path bundle_dir install_root
  work_dir=$(mktemp -d "${TMP_ROOT}/nvimon-install.XXXXXX")
  trap 'rm -rf "${work_dir}"' EXIT
  archive_path="${work_dir}/nvimon.tar.gz"
  metadata_path="${work_dir}/release.json"
  install_root="${work_dir}/installer"

  download_release_archive "${archive_path}" "${metadata_path}"
  tar -xzf "${archive_path}" -C "${work_dir}"
  bundle_dir=$(find_release_bundle_dir "${work_dir}")
  prepare_installer_layout "${bundle_dir}" "${install_root}"

  if [[ "${MODE}" == "client" ]]; then
    exec "${install_root}/scripts/install-client.sh"
  fi

  exec "${install_root}/scripts/install-agent.sh"
}

main "$@"
