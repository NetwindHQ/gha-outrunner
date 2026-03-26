# Minimal GitHub Actions runner image for outrunner.
# Based on the official runner with a non-root user.
#
# Build: docker build -t outrunner-runner runner/
FROM ghcr.io/actions/actions-runner:latest

# Pre-install common tools jobs expect
USER root
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    git \
    jq \
    && rm -rf /var/lib/apt/lists/*
USER runner

WORKDIR /actions-runner
