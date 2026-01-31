# hawkdog

A tiny Linux daemon that watches a single “sentinel” file for access and alerts you via **Telegram + email**.

(Internal note for the owner: look for `# ask peasdog` in the watched file.)

This is designed to catch the class of supply‑chain / instruction-following failures where something unexpectedly touches sensitive local paths.

## What it does
- Ensures a sentinel file exists at a configured path (default: `~/.clawdbot/credentials/aws_creds_cache.ini`).
- Watches that file using Linux **inotify** for events like:
  - open/read (`IN_OPEN`)
  - modify (`IN_MODIFY`)
  - attribute change (`IN_ATTRIB`)
  - delete/move (`IN_DELETE_SELF`, `IN_MOVE_SELF`)
- On trigger, sends an alert to:
  - Telegram (Bot API)
  - Email (via local `msmtp`)

## Non-goals
- This is not a firewall.
- This does not attempt to block access (detection-first).

## Install

### Option A: Install from a release (recommended)
1) Download the latest release for your platform from GitHub Releases.
2) Verify `checksums.txt`.
3) Place the `hawkdog` binary somewhere in your PATH (or follow the systemd user service install below).

### Option B: Build from source
```bash
go build -o hawkdog ./cmd/sentinel-watch
```

### 2) Configure
Create `~/.config/sentinel-watch/config.json` (permissions 600):
```json
{
  "sentinelPath": "/home/ubuntu/.clawdbot/credentials/aws_creds_cache.ini",
  "telegramBotToken": "<BOT_TOKEN>",
  "telegramChatId": 1592940510,
  "emailTo": "mark@prove.com.au",
  "emailFrom": "peasdog@idlepig.com",
  "msmtpAccount": "idlepig"
}
```

### 3) Run (manual)
```bash
./hawkdog
```

### 4) Run as a systemd user service
Edit `systemd/sentinel-watch.service` if needed and install:
```bash
mkdir -p ~/.config/systemd/user
cp systemd/sentinel-watch.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now sentinel-watch
```

## Testing
Trigger an open event:
```bash
cat /home/ubuntu/.clawdbot/credentials/aws_creds_cache.ini >/dev/null
```

You should receive a Telegram + email alert.

## License
MIT
