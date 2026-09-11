#!/usr/bin/env bash
# Keen installer — downloads a verified release binary and installs it.
#
# Usage:
#   ./install.sh [--version vX.Y.Z] [--bin-dir DIR]
#
# Examples:
#   ./install.sh
#   ./install.sh --version v0.9.4 --bin-dir ~/.local/bin
#
# Environment overrides (repository, hosting, and version resolution):
#   KEEN_INSTALL_REPO            GitHub repository. Defaults to divijg19/Keen.
#   KEEN_INSTALL_BASE_URL        Release download base URL. Defaults to the
#                               repository's releases/download endpoint.
#   KEEN_INSTALL_LATEST_API_URL  API endpoint for latest-release resolution.
#   KEEN_INSTALL_LATEST_TAG      Use this tag instead of resolving latest.
#
# The installer never executes downloaded content before verification: the
# binary's SHA-256 is checked against the release checksums.txt first, and
# the installed binary must report the requested version afterwards.
set -euo pipefail

REPO="${KEEN_INSTALL_REPO:-divijg19/Keen}"
BASE_URL="${KEEN_INSTALL_BASE_URL:-https://github.com/${REPO}/releases/download}"
LATEST_API_URL="${KEEN_INSTALL_LATEST_API_URL:-https://api.github.com/repos/${REPO}/releases/latest}"
BIN_DIR="${HOME}/.local/bin"
VERSION=""

usage() {
  cat <<'EOF'
Install Keen from a GitHub release.

Usage:
  ./install.sh [--version vX.Y.Z] [--bin-dir DIR]

Options:
  --version TAG              Install a specific release tag. Defaults to the latest release.
  --bin-dir DIR              Install the keen binary into DIR. Defaults to ~/.local/bin.
  -h, --help                 Show this help text.

Release assets are single verified binaries:
  keen_<version>_<os>_<arch>

The installer verifies the downloaded binary against the release
checksums.txt file before installing it.
EOF
}

detect_os() {
  case "$(uname -s)" in
    Linux) echo "linux" ;;
    Darwin) echo "darwin" ;;
    *)
      echo "unsupported operating system: $(uname -s)" >&2
      exit 1
      ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *)
      echo "unsupported architecture: $(uname -m)" >&2
      exit 1
      ;;
  esac
}

resolve_version() {
  if [[ -n "${VERSION}" ]]; then
    echo "${VERSION}"
    return
  fi

  if [[ -n "${KEEN_INSTALL_LATEST_TAG:-}" ]]; then
    echo "${KEEN_INSTALL_LATEST_TAG}"
    return
  fi

  local tag
  tag="$(curl -fsSL "${LATEST_API_URL}" 2>/dev/null | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
  if [[ -z "${tag}" ]]; then
    # Unauthenticated API quota is small and shared (NATs, CI runners), so
    # fall back to the release page redirect, which needs no API calls.
    tag="$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest" 2>/dev/null | sed 's#.*/tag/##')"
  fi
  if [[ -z "${tag}" || "${tag}" == "latest" ]]; then
    echo "failed to resolve the latest Keen release tag" >&2
    exit 1
  fi
  echo "${tag}"
}

asset_name_for() {
  local version="$1"
  local os_name="$2"
  local arch="$3"
  echo "keen_${version}_${os_name}_${arch}"
}

asset_url_for() {
  local version="$1"
  local os_name="$2"
  local arch="$3"
  local asset_name
  asset_name="$(asset_name_for "${version}" "${os_name}" "${arch}")"
  echo "${BASE_URL}/${version}/${asset_name}"
}

checksums_url_for() {
  local version="$1"
  echo "${BASE_URL}/${version}/checksums.txt"
}

require_value() {
  local option="$1"
  local value="${2:-}"
  if [[ -z "${value}" || "${value}" == --* ]]; then
    echo "${option} requires a value" >&2
    usage >&2
    exit 1
  fi
}

verify_checksum() {
  local binary_path="$1"
  local asset_name="$2"
  local checksums_path="$3"
  local expected actual

  expected="$(awk -v asset="${asset_name}" '$2 == asset { print $1 }' "${checksums_path}")"
  if [[ -z "${expected}" ]]; then
    echo "checksums.txt did not contain ${asset_name}" >&2
    exit 1
  fi

  if command -v sha256sum >/dev/null 2>&1; then
    printf '%s  %s\n' "${expected}" "${binary_path}" | sha256sum -c - >/dev/null
    return
  fi

  if command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "${binary_path}" | awk '{ print $1 }')"
    if [[ "${actual}" != "${expected}" ]]; then
      echo "checksum mismatch for ${asset_name}" >&2
      exit 1
    fi
    return
  fi

  echo "cannot verify checksum: sha256sum or shasum is required" >&2
  exit 1
}

install_binary() {
  local source_bin="$1"
  mkdir -p "${BIN_DIR}"
  if command -v install >/dev/null 2>&1; then
    install -m 0755 "${source_bin}" "${BIN_DIR}/keen"
  else
    cp "${source_bin}" "${BIN_DIR}/keen"
    chmod 0755 "${BIN_DIR}/keen"
  fi
}

verify_installed_version() {
  local version="$1"
  local reported
  reported="$("${BIN_DIR}/keen" -version)"
  if [[ "${reported}" != "keen ${version}" ]]; then
    echo "installed binary reports '${reported}', want 'keen ${version}'" >&2
    exit 1
  fi
}

path_hint() {
  case ":${PATH}:" in
    *":${BIN_DIR}:"*) return 0 ;;
  esac

  echo "Add ${BIN_DIR} to your PATH:"
  echo "  export PATH=\"${BIN_DIR}:\$PATH\""
}

main() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --version)
        require_value "$1" "${2:-}"
        VERSION="$2"
        shift 2
        ;;
      --bin-dir)
        require_value "$1" "${2:-}"
        BIN_DIR="$2"
        shift 2
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        echo "unknown argument: $1" >&2
        usage >&2
        exit 1
        ;;
    esac
  done

  local os_name arch version asset_name asset_url checksums_url tmp_dir binary_path checksums_path
  os_name="$(detect_os)"
  arch="$(detect_arch)"
  version="$(resolve_version)"
  asset_name="$(asset_name_for "${version}" "${os_name}" "${arch}")"
  asset_url="$(asset_url_for "${version}" "${os_name}" "${arch}")"
  checksums_url="$(checksums_url_for "${version}")"
  tmp_dir="$(mktemp -d)"
  binary_path="${tmp_dir}/${asset_name}"
  checksums_path="${tmp_dir}/checksums.txt"
  trap 'rm -rf "${tmp_dir:-}"' EXIT

  echo "Installing Keen ${version} for ${os_name}/${arch}..."
  curl -fsSL "${asset_url}" -o "${binary_path}"
  curl -fsSL "${checksums_url}" -o "${checksums_path}"
  verify_checksum "${binary_path}" "${asset_name}" "${checksums_path}"
  install_binary "${binary_path}"
  verify_installed_version "${version}"
  echo "Verified: ${asset_name}"
  echo "Installed: ${BIN_DIR}/keen"

  path_hint || true
}

# Run main when executed — as a file or through a curl pipe — but not when
# sourced. `return` outside a function succeeds only when sourced, so this
# discriminates all four cases: ./install.sh, bash install.sh, curl | bash,
# and source install.sh. (A BASH_SOURCE comparison cannot see the piped
# case: the array is empty when the program arrives on stdin.)
if (return 0 2>/dev/null); then
  : "sourced into a shell; main not invoked"
else
  main "$@"
fi
