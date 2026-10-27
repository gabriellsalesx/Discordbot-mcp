// Package auth stores the Discord bot token on disk and resolves it from the
// environment.
package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Credentials is the persisted login state.
type Credentials struct {
	Version int    `json:"version"`
	Token   string `json:"token"`
	AppID   string `json:"application_id,omitempty"`
	BotID   string `json:"bot_id,omitempty"`
	BotName string `json:"bot_name,omitempty"`
	// Server is the default server (id or name) for tools called without one.
	Server  string    `json:"server,omitempty"`
	SavedAt time.Time `json:"saved_at"`
}

// ErrNoCredentials means neither the environment nor the config dir has a token.
var ErrNoCredentials = errors.New("no Discord bot connected: run `discord-mcp login` or set DISCORD_TOKEN")

// Dir returns the config directory, honoring DISCORD_MCP_CONFIG_DIR.
func Dir() (string, error) {
	if d := os.Getenv("DISCORD_MCP_CONFIG_DIR"); d != "" {
		return d, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "discord-mcp"), nil
}

// Path returns the credentials file path.
func Path() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "credentials.json"), nil
}

// Load returns credentials from the environment first, then from disk.
func Load() (*Credentials, error) {
	if c := FromEnv(); c != nil {
		return c, nil
	}
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNoCredentials
	}
	if err != nil {
		return nil, err
	}
	if runtime.GOOS != "windows" {
		if fi, err := os.Stat(p); err == nil && fi.Mode().Perm()&0o077 != 0 {
			return nil, fmt.Errorf("%s is readable by other users; run `chmod 600 %q`", p, p)
		}
	}
	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("read %s: %w", p, err)
	}
	c.Token = CleanToken(c.Token)
	if c.Token == "" {
		return nil, ErrNoCredentials
	}
	return &c, nil
}

// FromEnv builds credentials from DISCORD_TOKEN or DISCORD_BOT_TOKEN. It
// returns nil when neither is set.
func FromEnv() *Credentials {
	for _, k := range []string{"DISCORD_TOKEN", "DISCORD_BOT_TOKEN"} {
		if t := CleanToken(os.Getenv(k)); t != "" {
			return &Credentials{Version: 1, Token: t}
		}
	}
	return nil
}

// CleanToken trims what people tend to paste around a token: spaces, quotes
// and a "Bot " prefix.
func CleanToken(s string) string {
	s = strings.Trim(strings.TrimSpace(s), `"'`)
	if len(s) > 4 && strings.EqualFold(s[:4], "bot ") {
		s = strings.TrimSpace(s[4:])
	}
	return s
}

// Save writes credentials atomically with 0600 permissions.
func Save(c *Credentials) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	c.Version = 1
	c.SavedAt = time.Now().UTC()
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".credentials-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil && runtime.GOOS != "windows" {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), p)
}

// Delete removes the stored credentials. A missing file is not an error.
func Delete() error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
