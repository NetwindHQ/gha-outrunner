#!/bin/sh
set -e

if [ -d /run/systemd/system ]; then
    systemctl stop outrunner.service >/dev/null 2>&1 || true
    systemctl disable outrunner.service >/dev/null 2>&1 || true
fi
