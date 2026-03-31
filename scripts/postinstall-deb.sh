#!/bin/sh
set -e

# Set up apt repository for automatic updates
# Skip with: OUTRUNNER_NO_REPO=1 dpkg -i outrunner_*.deb
if [ -z "${OUTRUNNER_NO_REPO:-}" ] && [ ! -f /etc/apt/sources.list.d/outrunner.sources ]; then
    mkdir -p /etc/apt/keyrings
    curl -fsSL https://pkg.netwind.pl/NetwindHQ/gha-outrunner/public.key \
        -o /etc/apt/keyrings/outrunner.asc
    cat > /etc/apt/sources.list.d/outrunner.sources <<EOF
Types: deb
URIs: https://pkg.netwind.pl/NetwindHQ/gha-outrunner
Suites: stable
Components: main
Signed-By: /etc/apt/keyrings/outrunner.asc
EOF
fi

# Reload systemd
if [ -d /run/systemd/system ]; then
    systemctl daemon-reload >/dev/null || true
fi
