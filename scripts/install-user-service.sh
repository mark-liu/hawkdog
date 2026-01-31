#!/usr/bin/env bash
set -euo pipefail

APPDIR="$HOME/.local/share/sentinel-watch"
mkdir -p "$APPDIR"

cd "$(dirname "$0")/.."

go build -o "$APPDIR/sentinel-watch" ./cmd/sentinel-watch

mkdir -p "$HOME/.config/systemd/user"
cp systemd/sentinel-watch.service "$HOME/.config/systemd/user/sentinel-watch.service"

systemctl --user daemon-reload
systemctl --user enable --now sentinel-watch.service

echo "Installed and started sentinel-watch (user service)."

echo "Create config at: $HOME/.config/sentinel-watch/config.json (chmod 600)"
