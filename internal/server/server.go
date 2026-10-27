// Package server exposes Discord server management as MCP tools.
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gabriellsalesx/discord-mcp/internal/auth"
	"github.com/gabriellsalesx/discord-mcp/internal/discord"
)

const instructions = `Manage Discord servers through the user's bot: channels, categories, roles, permissions and member roles. Servers, channels, roles and members can be named instead of using ids. Call list_channels and list_roles before reorganizing. delete_channel, delete_role and send_message call first without confirm to get a preview, show it to the user, and repeat with confirm=true only after they agree.`

// Config wires the server to its environment.
type Config struct {
	Version string
	Logger  *slog.Logger
	// Server is the default server (id or name).
	Server string
	// NewClient overrides client construction, for tests.
	NewClient func() (*discord.Client, error)
}

// Server holds the lazily created Discord client.
type Server struct {
	cfg Config

	mu     sync.Mutex
	client *discord.Client
	def    string
	loaded time.Time // mtime of the credentials file behind client
}

// New builds the MCP server with every tool registered.
func New(cfg Config) *mcp.Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	s := &Server{cfg: cfg}
	ms := mcp.NewServer(&mcp.Implementation{
		Name:       "discord-mcp",
		Title:      "Discord",
		Version:    cfg.Version,
		WebsiteURL: "https://github.com/gabriellsalesx/discord-mcp",
	}, &mcp.ServerOptions{Instructions: instructions, Logger: cfg.Logger})
	s.register(ms)
	ms.AddReceivingMiddleware(stringIDs)
	return ms
}

// stringIDs turns large JSON numbers in tool arguments into strings. Models
// often pass snowflake ids unquoted; the raw digits are kept, so no precision
// is lost to float64.
func stringIDs(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		if p, ok := req.GetParams().(*mcp.CallToolParamsRaw); ok && method == "tools/call" && len(p.Arguments) > 0 {
			dec := json.NewDecoder(bytes.NewReader(p.Arguments))
			dec.UseNumber()
			var args any
			if dec.Decode(&args) == nil {
				if v, changed := quoteSnowflakes(args); changed {
					if b, err := json.Marshal(v); err == nil {
						p.Arguments = b
					}
				}
			}
		}
		return next(ctx, method, req)
	}
}

func quoteSnowflakes(v any) (any, bool) {
	changed := false
	switch t := v.(type) {
	case json.Number:
		if s := t.String(); len(s) >= 15 && !strings.ContainsAny(s, ".eE-") {
			return s, true
		}
	case map[string]any:
		for k, e := range t {
			if nv, c := quoteSnowflakes(e); c {
				t[k], changed = nv, true
			}
		}
	case []any:
		for i, e := range t {
			if nv, c := quoteSnowflakes(e); c {
				t[i], changed = nv, true
			}
		}
	}
	return v, changed
}

// session returns the client, creating it on first use so that a login made
// after the MCP client started is picked up without a restart.
func (s *Server) session() (*discord.Client, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil && !s.credentialsChanged() {
		return s.client, s.def, nil
	}
	if s.cfg.NewClient != nil {
		c, err := s.cfg.NewClient()
		if err != nil {
			return nil, "", err
		}
		s.client, s.def = c, s.cfg.Server
		return c, s.def, nil
	}
	creds, err := auth.Load()
	if err != nil {
		return nil, "", err
	}
	s.loaded = credentialsMtime()
	s.client = discord.New(discord.Options{Token: creds.Token, UserAgent: discord.UserAgent(s.cfg.Version), Logger: s.cfg.Logger})
	s.def = firstNonEmpty(s.cfg.Server, creds.Server)
	return s.client, s.def, nil
}

// credentialsChanged reports whether `discord-mcp login` wrote a new token
// since the client was built. Environment tokens never change.
func (s *Server) credentialsChanged() bool {
	if s.cfg.NewClient != nil || auth.FromEnv() != nil {
		return false
	}
	return !credentialsMtime().Equal(s.loaded)
}

func credentialsMtime() time.Time {
	p, err := auth.Path()
	if err != nil {
		return time.Time{}
	}
	fi, err := os.Stat(p)
	if err != nil {
		return time.Time{}
	}
	return fi.ModTime()
}

// schemaFor infers the input schema of T and drops the "null" that Go
// pointers and slices add to a property's type. A type array such as
// ["null","array"] is rejected by Gemini and some other clients; clients just
// omit optional fields instead of sending null.
func schemaFor[T any]() *jsonschema.Schema {
	s, err := jsonschema.For[T](nil)
	if err != nil {
		panic(err)
	}
	dropNull(s)
	return s
}

func dropNull(s *jsonschema.Schema) {
	if s == nil {
		return
	}
	if len(s.Types) > 0 {
		var kept []string
		for _, t := range s.Types {
			if t != "null" {
				kept = append(kept, t)
			}
		}
		if len(kept) == 1 {
			s.Type, s.Types = kept[0], nil
		} else {
			s.Types = kept
		}
	}
	for _, p := range s.Properties {
		dropNull(p)
	}
	dropNull(s.Items)
}

// EnvServer reads the default server from the environment.
func EnvServer() string {
	for _, k := range []string{"DISCORD_SERVER", "DISCORD_GUILD_ID"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

func clamp(v, def, lo, hi int) int {
	if v == 0 {
		return def
	}
	return max(lo, min(v, hi))
}

func quoteList(items []string) string {
	if len(items) > 15 {
		items = append(items[:15:15], fmt.Sprintf("and %d more", len(items)-15))
	}
	return strings.Join(items, ", ")
}
