// Package layout stores server layouts (roles, categories, channels and
// their permissions) as JSON files, so one server's structure can be
// recreated in another.
package layout

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/gabriellsalesx/Discordbot-mcp/internal/auth"
)

// Layout is a saved server structure. Permissions are by name and targets
// are role names, so it does not depend on any server's ids.
type Layout struct {
	Version       int        `json:"version"`
	Name          string     `json:"name"`
	Description   string     `json:"description,omitempty"`
	Source        string     `json:"source_server,omitempty"`
	SavedAt       time.Time  `json:"saved_at"`
	Everyone      []string   `json:"everyone_permissions"`
	Roles         []Role     `json:"roles"`
	Uncategorized []Channel  `json:"uncategorized"`
	Categories    []Category `json:"categories"`
}

// Role is a role, listed from the top of the hierarchy.
type Role struct {
	Name        string   `json:"name"`
	Color       string   `json:"color,omitempty"`
	Permissions []string `json:"permissions"`
	Hoist       bool     `json:"hoist,omitempty"`
	Mentionable bool     `json:"mentionable,omitempty"`
}

// Category holds its channels in sidebar order.
type Category struct {
	Name       string      `json:"name"`
	Overwrites []Overwrite `json:"permissions,omitempty"`
	Channels   []Channel   `json:"channels"`
}

// Channel is a channel inside a category or at the top level.
type Channel struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Topic     string `json:"topic,omitempty"`
	NSFW      bool   `json:"nsfw,omitempty"`
	Slowmode  int    `json:"slowmode_seconds,omitempty"`
	UserLimit int    `json:"user_limit,omitempty"`
	// Synced channels take their category's permissions.
	Synced     bool        `json:"synced_with_category,omitempty"`
	Overwrites []Overwrite `json:"permissions,omitempty"`
}

// Overwrite is a permission override for a role ("@everyone" or a role name).
type Overwrite struct {
	Role  string   `json:"role"`
	Allow []string `json:"allow,omitempty"`
	Deny  []string `json:"deny,omitempty"`
}

// Counts summarizes a layout.
func (l *Layout) Counts() (roles, categories, channels int) {
	channels = len(l.Uncategorized)
	for _, c := range l.Categories {
		channels += len(c.Channels)
	}
	return len(l.Roles), len(l.Categories), channels
}

// Dir is where layouts are stored, next to the credentials.
func Dir() (string, error) {
	d, err := auth.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "layouts"), nil
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Slug turns a layout name into its file name.
func Slug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ç", "c").Replace(s)
	return strings.Trim(nonSlug.ReplaceAllString(s, "-"), "-")
}

func path(name string) (string, error) {
	slug := Slug(name)
	if slug == "" {
		return "", errors.New("the layout needs a name with letters or digits")
	}
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, slug+".json"), nil
}

// ErrNotFound means no layout has that name.
var ErrNotFound = errors.New("layout not found")

// Save writes the layout, replacing one with the same name.
func Save(l *Layout) (string, error) {
	p, err := path(l.Name)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return "", err
	}
	l.Version = 1
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return "", err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return "", err
	}
	return p, os.Rename(tmp, p)
}

// Load reads a layout by name.
func Load(name string) (*Layout, error) {
	p, err := path(name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	if err != nil {
		return nil, err
	}
	var l Layout
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("read %s: %w", p, err)
	}
	return &l, nil
}

// List returns every saved layout, by name.
func List() ([]*Layout, error) {
	d, err := Dir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(d)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []*Layout
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(d, e.Name()))
		if err != nil {
			continue
		}
		var l Layout
		if json.Unmarshal(data, &l) == nil && l.Name != "" {
			out = append(out, &l)
		}
	}
	slices.SortFunc(out, func(a, b *Layout) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	return out, nil
}

// Delete removes a layout.
func Delete(name string) error {
	p, err := path(name)
	if err != nil {
		return err
	}
	if err := os.Remove(p); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: %q", ErrNotFound, name)
	} else if err != nil {
		return err
	}
	return nil
}
