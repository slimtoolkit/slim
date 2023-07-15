#!/usr/bin/env bash

function get_slim() {
  local DIST=""
  local EXT=""
  local FILENAME=""
  local KERNEL=""
  local MACHINE=""
  local TMP_DIR=""
  local URL=""
  local VER=""

  if [ -n "$1" ]; then
    VER=$1
  else
    # Get the current released tag_name
    VER=$(curl -sL https://api.github.com/repos/slimtoolkit/slim/releases \
        | grep tag_name | head -n1 | cut -d'"' -f4)
  fi

  if [ -n "${VER}" ]; then
    URL="https://downloads.dockerslim.com/releases/${VER}"
  else
    echo "ERROR! Could not retrieve the current Slim version number."
    exit 1
  fi

  # Get kernel name and machine architecture.
  KERNEL=$(uname -s)
  MACHINE=$(uname -m)

  # Determine the target distrubution
  if [ "${KERNEL}" == "Linux" ]; then
    EXT="tar.gz"
    if [ "${MACHINE}" == "x86_64" ]; then
      DIST="linux"
    elif [ "${MACHINE}" == "armv7l" ]; then
      DIST="linux_arm"
    elif [ "${MACHINE}" == "aarch64" ]; then
      DIST="linux_arm64"
    fi
  elif [ "${KERNEL}" == "Darwin" ]; then
    EXT="zip"
    if [ "${MACHINE}" == "x86_64" ]; then
      DIST="mac"
    elif [ "${MACHINE}" == "arm64" ]; then
      DIST="mac_m1"
    fi
  else
    echo "ERROR! ${KERNEL} is not a supported platform."
    exit 1
  fi

  # Was a known distribution detected?
  if [ -z "${DIST}" ]; then
    echo "ERROR! ${MACHINE} is not a supported architecture."
    exit 1
  fi

  # Derive the filename
  FILENAME="dist_${DIST}.${EXT}"

  echo " - Downloading ${URL}/${FILENAME}"
  TMP_DIR=$(mktemp -d)
  curl -sLo "${TMP_DIR}/${FILENAME}" "${URL}/${FILENAME}"

  echo " - Unpacking ${FILENAME}"
  if [ "${EXT}" == "zip" ]; then
    unzip -qq -o "${TMP_DIR}/${FILENAME}" -d "${TMP_DIR}"
  elif [ "${EXT}" == "tar.gz" ]; then
    tar -xf "${TMP_DIR}/${FILENAME}" --directory "${TMP_DIR}"
  else
    echo "ERROR! Unexpected file extension."
    exit 1
  fi

  # /usr/local/bin should be present on Linux and macOS hosts. Just be sure.
  if [ -d /usr/local/bin ]; then
    echo " - Placing slim in /usr/local/bin"
    mv "${TMP_DIR}/dist_${DIST}/slim" /usr/local/bin/
    mv "${TMP_DIR}/dist_${DIST}/slim-sensor" /usr/local/bin/
    chmod +x /usr/local/bin/slim
    chmod +x /usr/local/bin/slim-sensor

    echo " - Cleaning up"
    rm -rf "${TMP_DIR}"
    echo -en " - "
    slim --version
  else
    echo "ERROR! /usr/local/bin is not present. Install aborted."
    rm -rf "${TMP_DIR}"
    exit 1
  fi
}

echo "Slim scripted install"

if [ "$(id -u)" -ne 0 ]; then
  echo "ERROR! You must run this script as root."
  exit 1
fi

get_slim $1

# You can pass a specific version to install otherwise the latest version will be installed
