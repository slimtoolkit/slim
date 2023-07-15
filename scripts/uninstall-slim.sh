#!/usr/bin/env bash

function uninstall_slim() {
  local VER=""

  # /usr/local/bin should be present on Linux and macOS hosts. Just be sure.
  if [ -d /usr/local/bin ]; then
    VER=$(slim --version | cut -d'|' -f3)
    echo " - Uninstalling version - ${VER}"

    echo " - Removing slim slim binaries from /usr/local/bin"
    rm /usr/local/bin/slim
    rm /usr/local/bin/slim-sensor

    echo " - Removing local state directory"
    rm -rfv /tmp/slim-state

    echo " - Removing state volume"
    docker volume rm slim-state

    echo " - Removing sensor volume"
    docker volume rm slim-sensor.${VER}
  else
    echo "ERROR! /usr/local/bin is not present. Uninstall aborted."
    exit 1
  fi
}

echo "Slim scripted uninstall"

if [ "$(id -u)" -ne 0 ]; then
  echo "ERROR! You must run this script as root."
  exit 1
fi

uninstall_slim
