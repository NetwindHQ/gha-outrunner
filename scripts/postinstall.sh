#!/bin/sh
set -e

# Create system user/group
if ! getent group outrunner >/dev/null; then
    groupadd --system outrunner
fi
if ! getent passwd outrunner >/dev/null; then
    useradd --system \
        --gid outrunner \
        --no-create-home \
        --shell /usr/sbin/nologin \
        --comment "outrunner service" \
        outrunner
fi

# Reload systemd
if [ -d /run/systemd/system ]; then
    systemctl daemon-reload >/dev/null || true
fi
