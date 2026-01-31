#!/usr/bin/env bash
set -euo pipefail

REPO="mark-liu/hawkdog"
INSTALL_DIR="$HOME/.local/share/hawkdog"
BIN_NAME="hawkdog"
OS="linux"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64) ARCH="amd64";;
  aarch64|arm64) ARCH="arm64";;
  *) echo "Unsupported arch: $ARCH"; exit 1;;
 esac

mkdir -p "$INSTALL_DIR"

# Fetch latest release JSON
json=$(python3 - <<'PY'
import json,urllib.request
url='https://api.github.com/repos/mark-liu/hawkdog/releases/latest'
req=urllib.request.Request(url, headers={'User-Agent':'hawkdog-installer'})
print(urllib.request.urlopen(req, timeout=20).read().decode())
PY
)

# Pick asset URLs
python3 - <<'PY'
import json,sys
j=json.loads(sys.stdin.read())
assets=j.get('assets',[])
want_bin=None
want_sum=None
for a in assets:
  url=a.get('browser_download_url','')
  name=a.get('name','')
  if name.endswith('checksums.txt'):
    want_sum=url
  if 'linux' in name and ('amd64' in name or 'arm64' in name) and name.endswith('.tar.gz'):
    # choose later in bash by arch match
    pass
print(want_sum or '')
PY
<<<"$json" > /tmp/hawkdog_checksums_url

CHECKSUMS_URL=$(cat /tmp/hawkdog_checksums_url)
if [[ -z "$CHECKSUMS_URL" ]]; then
  echo "Could not find checksums.txt in latest release." >&2
  exit 1
fi

TARBALL_NAME="hawkdog_${OS}_${ARCH}.tar.gz"
TARBALL_URL=$(python3 - <<'PY'
import json,sys
j=json.loads(sys.stdin.read())
arch=sys.argv[1]
os=sys.argv[2]
name=f"hawkdog_{os}_{arch}.tar.gz"
for a in j.get('assets',[]):
  if a.get('name')==name:
    print(a.get('browser_download_url',''))
    break
PY
"$ARCH" "$OS" <<<"$json")

if [[ -z "$TARBALL_URL" ]]; then
  echo "Could not find asset $TARBALL_NAME in latest release." >&2
  exit 1
fi

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

cd "$tmpdir"

echo "Downloading $TARBALL_NAME …"
curl -fsSLO "$TARBALL_URL"
echo "Downloading checksums.txt …"
curl -fsSLo checksums.txt "$CHECKSUMS_URL"

echo "Verifying checksum …"
grep "  $TARBALL_NAME$" checksums.txt | sha256sum -c -

echo "Extracting …"
tar -xzf "$TARBALL_NAME"

if [[ ! -f "$BIN_NAME" ]]; then
  echo "Expected binary '$BIN_NAME' not found in archive." >&2
  exit 1
fi

install -m 0755 "$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"

mkdir -p "$HOME/.config/systemd/user"
cp systemd/sentinel-watch.service "$HOME/.config/systemd/user/hawkdog.service"

systemctl --user daemon-reload
systemctl --user enable --now hawkdog.service

echo "Installed hawkdog to: $INSTALL_DIR/$BIN_NAME"
echo "Service: hawkdog.service (systemd user)"
echo "Config: $HOME/.config/hawkdog/config.json (preferred) or legacy $HOME/.config/sentinel-watch/config.json"
