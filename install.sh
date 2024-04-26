#!/bin/bash
#
# This script installs the latest version into /usr/local/bin or TARGET if specified
#
set -eo pipefail

required_tools=("curl" "grep" "awk" "unzip")
for tool in "${required_tools[@]}"; do
  if ! command -v $tool &>/dev/null; then
    echo "Error: $tool is required but not installed. Please install it first."
    exit 1
  fi
done

main() {
  local username="tractordev"
  local repo="wanix"
  local binpath="${TARGET:-/usr/local/bin}"

  # Check if write permission is available
  if [[ ! -w "$binpath" ]]; then
    echo "Error: No write permission for $binpath. Try running with sudo or choose a different TARGET."
    exit 1
  fi

  local repoURL="https://github.com/${username}/${repo}"
  local releaseURL="$(curl -sI ${repoURL}/releases/latest | grep 'location:' | awk '{print $2}')"
  local version="$(basename $releaseURL | cut -c 2- | tr -d '\r')"

  local os=""
  local arch=""

  # Detect operating system
  if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    os="linux"
  elif [[ "$OSTYPE" == "darwin"* ]]; then
    os="darwin"
  elif [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" ]]; then
    # WSL uses msys or cygwin as OSTYPE
    os="windows"
  else
    echo "Error: Unsupported operating system"
    exit 1
  fi

  # Detect architecture
  if [[ "$(uname -m)" == "x86_64" ]]; then
    arch="amd64"
  elif [[ "$(uname -m)" == "arm64" ]]; then
    arch="arm64"
  else
    echo "Error: Unsupported architecture"
    exit 1
  fi

  local filename="${repo}_${version}_${os}_${arch}.zip"
  curl -sSLO "${repoURL}/releases/download/v${version}/${filename}"
  unzip $filename wanix -d $binpath
  rm "./$filename"

  echo "Executable wanix ${version} installed to ${binpath}"
}

main "$@"