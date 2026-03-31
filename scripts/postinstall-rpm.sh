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

# Set up rpm repository for automatic updates
# Skip with: OUTRUNNER_NO_REPO=1 rpm -i outrunner_*.rpm
if [ -z "${OUTRUNNER_NO_REPO:-}" ] && [ ! -f /etc/yum.repos.d/outrunner.repo ]; then
    rpm --import https://pkg.netwind.pl/NetwindHQ/gha-outrunner/public.key
    cat > /etc/yum.repos.d/outrunner.repo <<EOF
[outrunner]
name=outrunner from GitHub via pkg.netwind.pl
baseurl=https://pkg.netwind.pl/NetwindHQ/gha-outrunner
enabled=1
gpgcheck=0
repo_gpgcheck=1
gpgkey=https://pkg.netwind.pl/NetwindHQ/gha-outrunner/public.key
EOF
fi

# Reload systemd
if [ -d /run/systemd/system ]; then
    systemctl daemon-reload >/dev/null || true
fi
