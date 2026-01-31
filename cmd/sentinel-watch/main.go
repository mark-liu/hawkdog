package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

type Config struct {
	SentinelPath      string `json:"sentinelPath"`
	TelegramBotToken  string `json:"telegramBotToken"`
	TelegramChatID    int64  `json:"telegramChatId"`
	EmailTo           string `json:"emailTo"`
	EmailFrom         string `json:"emailFrom"`
	MsmtpAccount      string `json:"msmtpAccount"`
	AlertMinIntervalS int    `json:"alertMinIntervalSeconds"`
}

func defaultConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		SentinelPath:      filepath.Join(home, ".clawdbot", "credentials", "aws_creds_cache.ini"),
		AlertMinIntervalS: 60,
	}
}

func loadConfig() (Config, error) {
	cfg := defaultConfig()
	home, _ := os.UserHomeDir()
	p := filepath.Join(home, ".config", "sentinel-watch", "config.json")
	b, err := os.ReadFile(p)
	if err != nil {
		return cfg, fmt.Errorf("read config %s: %w", p, err)
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	if cfg.SentinelPath == "" {
		return cfg, errors.New("sentinelPath required")
	}
	if cfg.TelegramBotToken == "" {
		return cfg, errors.New("telegramBotToken required")
	}
	if cfg.TelegramChatID == 0 {
		return cfg, errors.New("telegramChatId required")
	}
	if cfg.EmailTo == "" || cfg.EmailFrom == "" {
		return cfg, errors.New("emailTo and emailFrom required")
	}
	if cfg.MsmtpAccount == "" {
		cfg.MsmtpAccount = "idlepig"
	}
	if cfg.AlertMinIntervalS <= 0 {
		cfg.AlertMinIntervalS = 60
	}
	return cfg, nil
}

func randHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func ensureSentinel(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// Create if missing.
	if _, err := os.Stat(path); err == nil {
		// Keep permissions tight.
		_ = os.Chmod(path, 0o600)
		return nil
	}
	tok, err := randHex(16)
	if err != nil {
		return err
	}
	// Plausible AWS-ish INI, but safely invalid values + private sentinel key.
	content := fmt.Sprintf("[default]\naws_access_key_id = ASIAFAKEFAKEFAKEFAKE\naws_secret_access_key = NOT_A_REAL_SECRET_%s\naws_session_token = NOT_A_REAL_TOKEN_%s\nx-idlepig-sentinel = %s\n", tok, tok, tok)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return err
	}
	return nil
}

func tgSend(token string, chatID int64, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	payload := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func emailSend(msmtpAccount, from, to, subject, body string) error {
	msg := fmt.Sprintf("From: %s\nTo: %s\nSubject: %s\n\n%s\n", from, to, subject, body)
	cmd := exec.Command("msmtp", "-a", msmtpAccount, to)
	cmd.Stdin = strings.NewReader(msg)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("msmtp failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func watch(cfg Config) error {
	if err := ensureSentinel(cfg.SentinelPath); err != nil {
		return fmt.Errorf("ensure sentinel: %w", err)
	}

	fd, err := unix.InotifyInit1(unix.IN_NONBLOCK)
	if err != nil {
		return fmt.Errorf("inotify init: %w", err)
	}
	defer unix.Close(fd)

	mask := uint32(unix.IN_OPEN | unix.IN_ATTRIB | unix.IN_MODIFY | unix.IN_DELETE_SELF | unix.IN_MOVE_SELF)
	wd, err := unix.InotifyAddWatch(fd, cfg.SentinelPath, mask)
	if err != nil {
		return fmt.Errorf("add watch: %w", err)
	}
	_ = wd

	lastAlert := time.Time{}
	minInterval := time.Duration(cfg.AlertMinIntervalS) * time.Second

	buf := make([]byte, 4096)
	for {
		n, err := unix.Read(fd, buf)
		if err != nil {
			if err == unix.EAGAIN {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			return fmt.Errorf("read inotify: %w", err)
		}
		if n <= 0 {
			continue
		}

		now := time.Now()
		if !lastAlert.IsZero() && now.Sub(lastAlert) < minInterval {
			continue
		}
		lastAlert = now

		event := "sentinel file accessed"
		msg := fmt.Sprintf("hawkdog alert\n\npath: %s\nevent: %s\ntime: %s\nhost: %s", cfg.SentinelPath, event, now.Format(time.RFC3339), hostname())

		if err := tgSend(cfg.TelegramBotToken, cfg.TelegramChatID, msg); err != nil {
			fmt.Fprintln(os.Stderr, "telegram send failed:", err)
		} else {
			fmt.Fprintln(os.Stderr, "telegram sent")
		}
		if err := emailSend(cfg.MsmtpAccount, cfg.EmailFrom, cfg.EmailTo, "hawkdog alert", msg); err != nil {
			fmt.Fprintln(os.Stderr, "email send failed:", err)
		} else {
			fmt.Fprintln(os.Stderr, "email sent")
		}
	}
}

func hostname() string {
	h, _ := os.Hostname()
	if h == "" {
		return "unknown"
	}
	return h
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := watch(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
