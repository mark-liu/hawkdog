#!/usr/bin/env bash
set -euo pipefail

APPDIR="$HOME/.local/share/hawkdog"
mkdir -p "$APPDIR"

cd "$(dirname "$0")/.."

go build -o "$APPDIR/hawkdog" ./cmd/sentinel-watch

mkdir -p "$HOME/.config/systemd/user"
cp systemd/hawkdog.service "$HOME/.config/systemd/user/hawkdog.service"

systemctl --user daemon-reload
systemctl --user enable --now hawkdog.service

echo "Installed and started hawkdog (user service)."

echo "Create config at: $HOME/.config/sentinel-watch/config.json (chmod 600)"
