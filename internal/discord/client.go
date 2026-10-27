// Package discord is a small client for the parts of the Discord REST API
// (v10) that manage servers, channels, roles and permissions.
package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// API is the Discord REST base URL.
const API = "https://discord.com/api/v10"

// maxWait caps how long a call waits for a rate limit before giving up.
const maxWait = 60 * time.Second

// Client sends bot-authenticated requests and honors Discord's rate limits.
type Client struct {
	http      *http.Client
	token     string
	userAgent string
	timeout   time.Duration
	log       *slog.Logger

	// Base is the API root. Tests point it at a fake server.
	Base string

	mu      sync.Mutex
	buckets map[string]string    // route -> bucket hash
	limits  map[string]time.Time // bucket key -> time it refills, while exhausted
	global  time.Time            // global limit lifts at this time
}

// Options configures a Client.
type Options struct {
	Token     string
	UserAgent string
	Logger    *slog.Logger
	Timeout   time.Duration
}

// New returns a Client for a bot token.
func New(opt Options) *Client {
	if opt.UserAgent == "" {
		opt.UserAgent = "DiscordBot (https://github.com/gabriellsalesx/Discordbot-mcp, dev)"
	}
	if opt.Timeout == 0 {
		opt.Timeout = 30 * time.Second
	}
	if opt.Logger == nil {
		opt.Logger = slog.New(slog.DiscardHandler)
	}
	return &Client{
		http:      &http.Client{},
		token:     opt.Token,
		userAgent: opt.UserAgent,
		timeout:   opt.Timeout,
		log:       opt.Logger,
		Base:      API,
		buckets:   map[string]string{},
		limits:    map[string]time.Time{},
	}
}

// UserAgent formats the User-Agent Discord asks bots to send.
func UserAgent(version string) string {
	return "DiscordBot (https://github.com/gabriellsalesx/Discordbot-mcp, " + version + ")"
}

// Error is a non-2xx answer from Discord.
type Error struct {
	Status  int
	Code    int
	Method  string
	Path    string
	Message string
	Fields  []string
}

// Discord JSON error codes the hints below rely on.
const (
	CodeUnknownChannel     = 10003
	CodeUnknownGuild       = 10004
	CodeUnknownMember      = 10007
	CodeUnknownRole        = 10011
	CodeMissingAccess      = 50001
	CodeMissingPermissions = 50013
)

func (e *Error) Error() string {
	msg := e.Message
	if msg == "" {
		msg = http.StatusText(e.Status)
	}
	if len(e.Fields) > 0 {
		msg += " (" + strings.Join(e.Fields, "; ") + ")"
	}
	switch {
	case e.Status == http.StatusUnauthorized:
		return "Discord rejected the bot token (401). Run `discord-mcp login` again with a fresh token from the Developer Portal."
	case e.Code == CodeMissingPermissions:
		return fmt.Sprintf("Discord refused: %s. The bot needs that permission in this server or channel, and can only edit roles below its own highest role and grant permissions it has itself. In Server Settings > Roles, drag the bot's role above the roles it should manage, or reinvite it with `discord-mcp invite --admin`.", msg)
	case e.Code == CodeMissingAccess:
		return fmt.Sprintf("Discord refused: %s. The bot cannot see this channel or server; give its role View Channel there.", msg)
	case e.Status == http.StatusNotFound:
		return fmt.Sprintf("not found on Discord (%s %s): %s", e.Method, e.Path, msg)
	}
	return fmt.Sprintf("Discord API error %d (code %d) on %s %s: %s", e.Status, e.Code, e.Method, e.Path, msg)
}

// IsCode reports whether err is a Discord error with the given JSON code.
func IsCode(err error, code int) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == code
}

// IsStatus reports whether err is a Discord error with the given HTTP status.
func IsStatus(err error, status int) bool {
	var e *Error
	return errors.As(err, &e) && e.Status == status
}

// call is one API request. Body is sent as JSON when set.
type call struct {
	Method string
	Path   string
	Query  url.Values
	Body   any
	// Reason goes to the server's audit log.
	Reason string
}

// snowflakeSeg matches ids in a path, to group routes into rate-limit buckets.
var (
	snowflakeSeg = regexp.MustCompile(`/\d{15,25}`)
	majorParam   = regexp.MustCompile(`^/(channels|guilds|webhooks)/(\d+)`)
)

// routeKey identifies the route for rate limiting. Discord keys limits by
// bucket plus the top-level resource (channel, guild or webhook), so the first
// id is kept and the others are generalized.
func routeKey(method, path string) (route, major string) {
	if m := majorParam.FindStringSubmatch(path); m != nil {
		major = m[2]
	}
	return method + " " + snowflakeSeg.ReplaceAllString(path, "/:id"), major
}

func (c *Client) do(ctx context.Context, r call, out any) error {
	u := c.Base + r.Path
	if len(r.Query) > 0 {
		u += "?" + r.Query.Encode()
	}
	var payload []byte
	if r.Body != nil {
		var err error
		if payload, err = json.Marshal(r.Body); err != nil {
			return err
		}
	}
	route, major := routeKey(r.Method, r.Path)

	for attempt := 0; ; attempt++ {
		if err := c.wait(ctx, route, major); err != nil {
			return err
		}
		resp, body, err := c.send(ctx, r, u, payload)
		if err != nil {
			// Network errors: retry reads only.
			if r.Method == http.MethodGet && attempt < 2 && ctx.Err() == nil {
				if err := sleep(ctx, time.Duration(attempt+1)*500*time.Millisecond); err != nil {
					return err
				}
				continue
			}
			return err
		}
		c.update(route, major, resp.Header)

		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			// A 429 was not processed, so every method is safe to repeat.
			retry := c.retryAfter(resp, body)
			if attempt >= 4 || retry > maxWait {
				return fmt.Errorf("rate limited by Discord on %s %s; try again in %s", r.Method, r.Path, retry.Round(time.Second))
			}
			c.log.Debug("rate limited", "route", route, "retry_after", retry)
			if err := sleep(ctx, retry); err != nil {
				return err
			}
			continue
		case resp.StatusCode >= 500 && attempt < 2 && r.Method != http.MethodPost && r.Method != http.MethodPatch:
			if err := sleep(ctx, time.Duration(attempt+1)*time.Second); err != nil {
				return err
			}
			continue
		case resp.StatusCode >= 300:
			return parseError(resp.StatusCode, r.Method, r.Path, body)
		}
		if out == nil || resp.StatusCode == http.StatusNoContent || len(body) == 0 {
			return nil
		}
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("decode %s %s: %w", r.Method, r.Path, err)
		}
		return nil
	}
}

func (c *Client) send(ctx context.Context, r call, u string, payload []byte) (*http.Response, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, r.Method, u, bytes.NewReader(payload))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bot "+c.token)
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if r.Reason != "" {
		req.Header.Set("X-Audit-Log-Reason", url.PathEscape(truncateRunes(r.Reason, 512)))
	}
	start := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	c.log.Debug("discord", "method", r.Method, "path", r.Path, "status", resp.StatusCode, "took", time.Since(start))
	return resp, body, err
}

// wait blocks while the route's bucket, or the global limit, is exhausted.
func (c *Client) wait(ctx context.Context, route, major string) error {
	c.mu.Lock()
	until := c.global
	if b, ok := c.buckets[route]; ok {
		if t := c.limits[b+":"+major]; t.After(until) {
			until = t
		}
	}
	c.mu.Unlock()
	d := time.Until(until)
	if d <= 0 {
		return nil
	}
	if d > maxWait {
		return fmt.Errorf("rate limited by Discord; this action is available again in %s", d.Round(time.Second))
	}
	return sleep(ctx, d)
}

// update records the bucket state from the rate-limit headers.
func (c *Client) update(route, major string, h http.Header) {
	bucket := h.Get("X-RateLimit-Bucket")
	if bucket == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.buckets[route] = bucket
	key := bucket + ":" + major
	if h.Get("X-RateLimit-Remaining") == "0" {
		if s, err := strconv.ParseFloat(h.Get("X-RateLimit-Reset-After"), 64); err == nil {
			c.limits[key] = time.Now().Add(time.Duration(s * float64(time.Second)))
			return
		}
	}
	delete(c.limits, key)
}

func (c *Client) retryAfter(resp *http.Response, body []byte) time.Duration {
	var rl struct {
		RetryAfter float64 `json:"retry_after"`
		Global     bool    `json:"global"`
	}
	_ = json.Unmarshal(body, &rl)
	if rl.RetryAfter == 0 {
		rl.RetryAfter, _ = strconv.ParseFloat(resp.Header.Get("Retry-After"), 64)
	}
	d := time.Duration(rl.RetryAfter * float64(time.Second))
	if d <= 0 {
		d = time.Second
	}
	if rl.Global || resp.Header.Get("X-RateLimit-Global") == "true" {
		c.mu.Lock()
		c.global = time.Now().Add(d)
		c.mu.Unlock()
	}
	return d
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// parseError reads Discord's {code, message, errors} body. Field errors are
// nested by path and flattened to "path: message".
func parseError(status int, method, path string, body []byte) error {
	e := &Error{Status: status, Method: method, Path: path}
	var raw struct {
		Code    int            `json:"code"`
		Message string         `json:"message"`
		Errors  map[string]any `json:"errors"`
	}
	if json.Unmarshal(body, &raw) == nil {
		e.Code, e.Message = raw.Code, raw.Message
		flattenErrors("", raw.Errors, &e.Fields)
		slices.Sort(e.Fields)
	} else {
		e.Message = truncateRunes(strings.TrimSpace(string(body)), 300)
	}
	return e
}

func flattenErrors(prefix string, m map[string]any, out *[]string) {
	for _, k := range slices.Sorted(maps.Keys(m)) {
		v := m[k]
		if k == "_errors" {
			if list, ok := v.([]any); ok {
				for _, item := range list {
					if em, ok := item.(map[string]any); ok {
						*out = append(*out, fmt.Sprintf("%s: %v", strings.TrimPrefix(prefix, "."), em["message"]))
					}
				}
			}
			continue
		}
		if sub, ok := v.(map[string]any); ok {
			flattenErrors(prefix+"."+k, sub, out)
		}
	}
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
