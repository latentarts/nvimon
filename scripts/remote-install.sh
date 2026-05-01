#!/usr/bin/env bash
set -euo pipefail

MODE=${1:-}

REPO_OWNER=${REPO_OWNER:-prods}
REPO_NAME=${REPO_NAME:-nvimon}
NVIMON_REF=${NVIMON_REF:-main}
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

Downloads the requested nvimon source tree, builds the binaries locally, and
runs the matching installer.
EOF
}

download_repo_archive() {
  local dest_path=$1
  local archive_url="https://github.com/${REPO_OWNER}/${REPO_NAME}/archive/refs/heads/${NVIMON_REF}.tar.gz"

  if have_command curl; then
    curl -fsSL "${archive_url}" -o "${dest_path}"
    return
  fi

  if have_command wget; then
    wget -qO "${dest_path}" "${archive_url}"
    return
  fi

  fail "curl or wget is required to download ${archive_url}"
}

build_dist() {
  local repo_dir=$1

  have_command tar || fail "tar is required"
  have_command make || fail "make is required"
  have_command go || fail "go is required to build nvimon from source"

  log "building nvimon artifacts from ${NVIMON_REF}"
  if make -C "${repo_dir}" build; then
    return
  fi

  log "full build failed; falling back to portable artifacts only"
  make -C "${repo_dir}" build-portable
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

  local work_dir archive_path repo_dir
  work_dir=$(mktemp -d "${TMP_ROOT}/nvimon-install.XXXXXX")
  trap 'rm -rf "${work_dir}"' EXIT
  archive_path="${work_dir}/nvimon.tar.gz"

  download_repo_archive "${archive_path}"
  tar -xzf "${archive_path}" -C "${work_dir}"
  repo_dir="${work_dir}/${REPO_NAME}-${NVIMON_REF}"
  [[ -d "${repo_dir}" ]] || fail "downloaded archive did not contain ${repo_dir}"

  build_dist "${repo_dir}"

  if [[ "${MODE}" == "client" ]]; then
    exec "${repo_dir}/scripts/install-client.sh"
  fi

  exec "${repo_dir}/scripts/install-agent.sh"
}

main "$@"
