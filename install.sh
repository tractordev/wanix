#!/bin/bash
#
# This script installs the latest version into /usr/local/bin or TARGET if specified
#
set -eo pipefail

username="tractordev"
repo="wanix"
binpath="${TARGET:-/usr/local/bin}"

repoURL="https://github.com/${username}/${repo}"
releaseURL="$(curl -sI ${repoURL}/releases/latest | grep 'location:' | awk '{print $2}')"
version="$(basename $releaseURL | cut -c 2- | tr -d '\r')"

os=""
arch=""

# Detect operating system
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
  os="linux"
elif [[ "$OSTYPE" == "darwin"* ]]; then
  os="darwin"
elif [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" ]]; then
  # WSL uses msys or cygwin as OSTYPE
  os="windows"
else
  echo "Unsupported operating system"
  exit 1
fi

# Detect architecture
if [[ "$(uname -m)" == "x86_64" ]]; then
  arch="amd64"
elif [[ "$(uname -m)" == "arm64" ]]; then
  arch="arm64"
else
  echo "Unsupported architecture"
  exit 1
fi

filename="${repo}_${version}_${os}_${arch}.zip"

curl -sSLO "${repoURL}/releases/download/v${version}/${filename}"
unzip $filename wanix -d $binpath
rm $filename