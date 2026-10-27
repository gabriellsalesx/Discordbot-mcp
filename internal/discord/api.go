package discord

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// User is a Discord account.
type User struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	GlobalName string `json:"global_name"`
	Bot        bool   `json:"bot"`
}

// DisplayName is the name the Discord app shows.
func (u User) DisplayName() string {
	if u.GlobalName != "" {
		return u.GlobalName
	}
	return u.Username
}

// Application is the bot's application.
type Application struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Flags int64  `json:"flags"`
}

// Application flags that tell whether the Server Members intent is on.
const (
	appFlagGuildMembers        = 1 << 14
	appFlagGuildMembersLimited = 1 << 15
)

// MembersIntent reports whether the Server Members privileged intent is
// enabled, which member search needs.
func (a Application) MembersIntent() bool {
	return a.Flags&(appFlagGuildMembers|appFlagGuildMembersLimited) != 0
}

// PartialGuild is a server as listed by /users/@me/guilds.
type PartialGuild struct {
	ID                       string `json:"id"`
	Name                     string `json:"name"`
	Owner                    bool   `json:"owner"`
	Permissions              Perm   `json:"permissions"`
	ApproximateMemberCount   int    `json:"approximate_member_count"`
	ApproximatePresenceCount int    `json:"approximate_presence_count"`
}

// Guild is a server.
type Guild struct {
	ID                          string   `json:"id"`
	Name                        string   `json:"name"`
	Description                 string   `json:"description"`
	OwnerID                     string   `json:"owner_id"`
	VerificationLevel           int      `json:"verification_level"`
	DefaultMessageNotifications int      `json:"default_message_notifications"`
	ExplicitContentFilter       int      `json:"explicit_content_filter"`
	AFKChannelID                string   `json:"afk_channel_id"`
	AFKTimeout                  int      `json:"afk_timeout"`
	SystemChannelID             string   `json:"system_channel_id"`
	RulesChannelID              string   `json:"rules_channel_id"`
	PublicUpdatesChannelID      string   `json:"public_updates_channel_id"`
	PreferredLocale             string   `json:"preferred_locale"`
	Features                    []string `json:"features"`
	PremiumTier                 int      `json:"premium_tier"`
	PremiumSubscriptionCount    int      `json:"premium_subscription_count"`
	ApproximateMemberCount      int      `json:"approximate_member_count"`
	ApproximatePresenceCount    int      `json:"approximate_presence_count"`
	Roles                       []Role   `json:"roles"`
}

// Channel types.
const (
	ChannelText         = 0
	ChannelVoice        = 2
	ChannelCategory     = 4
	ChannelAnnouncement = 5
	ChannelStage        = 13
	ChannelForum        = 15
	ChannelMedia        = 16
)

// Channel is a guild channel or category.
type Channel struct {
	ID               string      `json:"id"`
	Type             int         `json:"type"`
	GuildID          string      `json:"guild_id"`
	Name             string      `json:"name"`
	Topic            string      `json:"topic"`
	Position         int         `json:"position"`
	ParentID         string      `json:"parent_id"`
	NSFW             bool        `json:"nsfw"`
	RateLimitPerUser int         `json:"rate_limit_per_user"`
	Bitrate          int         `json:"bitrate"`
	UserLimit        int         `json:"user_limit"`
	Overwrites       []Overwrite `json:"permission_overwrites"`
}

// Overwrite is a channel permission override for a role or member.
type Overwrite struct {
	ID    string `json:"id"`
	Type  int    `json:"type"`
	Allow Perm   `json:"allow"`
	Deny  Perm   `json:"deny"`
}

// Role is a server role.
type Role struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Color  int    `json:"color"`
	Colors *struct {
		PrimaryColor   int  `json:"primary_color"`
		SecondaryColor *int `json:"secondary_color"`
		TertiaryColor  *int `json:"tertiary_color"`
	} `json:"colors"`
	Hoist       bool `json:"hoist"`
	Position    int  `json:"position"`
	Permissions Perm `json:"permissions"`
	Managed     bool `json:"managed"`
	Mentionable bool `json:"mentionable"`
	Tags        *struct {
		BotID string `json:"bot_id"`
	} `json:"tags"`
}

// PrimaryColor is the role color, from colors or the deprecated color field.
func (r Role) PrimaryColor() int {
	if r.Colors != nil && r.Colors.PrimaryColor != 0 {
		return r.Colors.PrimaryColor
	}
	return r.Color
}

// RoleAbove reports whether a is higher than b in the hierarchy. Equal
// positions are ordered by id, the older (smaller) role on top.
func RoleAbove(a, b Role) bool {
	if a.Position != b.Position {
		return a.Position > b.Position
	}
	if len(a.ID) != len(b.ID) {
		return len(a.ID) < len(b.ID)
	}
	return a.ID < b.ID
}

// Member is a user in a server.
type Member struct {
	User     User      `json:"user"`
	Nick     string    `json:"nick"`
	Roles    []string  `json:"roles"`
	JoinedAt time.Time `json:"joined_at"`
}

// Message is a sent message.
type Message struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
}

// Me returns the bot user.
func (c *Client) Me(ctx context.Context) (*User, error) {
	var u User
	return &u, c.do(ctx, call{Method: http.MethodGet, Path: "/users/@me"}, &u)
}

// Application returns the bot's application.
func (c *Client) Application(ctx context.Context) (*Application, error) {
	var a Application
	return &a, c.do(ctx, call{Method: http.MethodGet, Path: "/applications/@me"}, &a)
}

// Guilds lists the servers the bot is in, with approximate member counts.
func (c *Client) Guilds(ctx context.Context) ([]PartialGuild, error) {
	var all []PartialGuild
	after := ""
	for {
		q := url.Values{"limit": {"200"}, "with_counts": {"true"}}
		if after != "" {
			q.Set("after", after)
		}
		var page []PartialGuild
		if err := c.do(ctx, call{Method: http.MethodGet, Path: "/users/@me/guilds", Query: q}, &page); err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < 200 {
			return all, nil
		}
		after = page[len(page)-1].ID
	}
}

// Guild returns a server with its roles and approximate counts.
func (c *Client) Guild(ctx context.Context, id string) (*Guild, error) {
	var g Guild
	return &g, c.do(ctx, call{Method: http.MethodGet, Path: "/guilds/" + id, Query: url.Values{"with_counts": {"true"}}}, &g)
}

// ModifyGuild changes server settings.
func (c *Client) ModifyGuild(ctx context.Context, id string, body map[string]any, reason string) (*Guild, error) {
	var g Guild
	return &g, c.do(ctx, call{Method: http.MethodPatch, Path: "/guilds/" + id, Body: body, Reason: reason}, &g)
}

// Channels lists a server's channels and categories (threads excluded).
func (c *Client) Channels(ctx context.Context, guildID string) ([]Channel, error) {
	var out []Channel
	return out, c.do(ctx, call{Method: http.MethodGet, Path: "/guilds/" + guildID + "/channels"}, &out)
}

// Channel returns one channel.
func (c *Client) Channel(ctx context.Context, id string) (*Channel, error) {
	var ch Channel
	return &ch, c.do(ctx, call{Method: http.MethodGet, Path: "/channels/" + id}, &ch)
}

// CreateChannel creates a channel or category.
func (c *Client) CreateChannel(ctx context.Context, guildID string, body map[string]any, reason string) (*Channel, error) {
	var ch Channel
	return &ch, c.do(ctx, call{Method: http.MethodPost, Path: "/guilds/" + guildID + "/channels", Body: body, Reason: reason}, &ch)
}

// ModifyChannel changes a channel. A nil value in body clears that field.
func (c *Client) ModifyChannel(ctx context.Context, id string, body map[string]any, reason string) (*Channel, error) {
	var ch Channel
	return &ch, c.do(ctx, call{Method: http.MethodPatch, Path: "/channels/" + id, Body: body, Reason: reason}, &ch)
}

// DeleteChannel deletes a channel or category.
func (c *Client) DeleteChannel(ctx context.Context, id, reason string) error {
	return c.do(ctx, call{Method: http.MethodDelete, Path: "/channels/" + id, Reason: reason}, nil)
}

// ChannelPosition is one entry of a bulk position change.
type ChannelPosition struct {
	ID       string `json:"id"`
	Position *int   `json:"position,omitempty"`
	// ParentID is a JSON string id, or null to leave the category.
	ParentID        json.RawMessage `json:"parent_id,omitempty"`
	LockPermissions *bool           `json:"lock_permissions,omitempty"`
}

// SetChannelPositions reorders channels. Discord allows at most one parent
// change per request.
func (c *Client) SetChannelPositions(ctx context.Context, guildID string, pos []ChannelPosition, reason string) error {
	return c.do(ctx, call{Method: http.MethodPatch, Path: "/guilds/" + guildID + "/channels", Body: pos, Reason: reason}, nil)
}

// SetOverwrite creates or replaces a channel permission overwrite.
func (c *Client) SetOverwrite(ctx context.Context, channelID string, o Overwrite, reason string) error {
	body := map[string]any{"allow": o.Allow, "deny": o.Deny, "type": o.Type}
	return c.do(ctx, call{Method: http.MethodPut, Path: "/channels/" + channelID + "/permissions/" + o.ID, Body: body, Reason: reason}, nil)
}

// DeleteOverwrite removes a channel permission overwrite.
func (c *Client) DeleteOverwrite(ctx context.Context, channelID, targetID, reason string) error {
	return c.do(ctx, call{Method: http.MethodDelete, Path: "/channels/" + channelID + "/permissions/" + targetID, Reason: reason}, nil)
}

// Roles lists a server's roles.
func (c *Client) Roles(ctx context.Context, guildID string) ([]Role, error) {
	var out []Role
	return out, c.do(ctx, call{Method: http.MethodGet, Path: "/guilds/" + guildID + "/roles"}, &out)
}

// RoleMemberCounts maps role ids to how many members hold them.
func (c *Client) RoleMemberCounts(ctx context.Context, guildID string) (map[string]int, error) {
	out := map[string]int{}
	return out, c.do(ctx, call{Method: http.MethodGet, Path: "/guilds/" + guildID + "/roles/member-counts"}, &out)
}

// CreateRole creates a role.
func (c *Client) CreateRole(ctx context.Context, guildID string, body map[string]any, reason string) (*Role, error) {
	var r Role
	return &r, c.do(ctx, call{Method: http.MethodPost, Path: "/guilds/" + guildID + "/roles", Body: body, Reason: reason}, &r)
}

// ModifyRole changes a role.
func (c *Client) ModifyRole(ctx context.Context, guildID, roleID string, body map[string]any, reason string) (*Role, error) {
	var r Role
	return &r, c.do(ctx, call{Method: http.MethodPatch, Path: "/guilds/" + guildID + "/roles/" + roleID, Body: body, Reason: reason}, &r)
}

// SetRolePosition moves a role in the hierarchy.
func (c *Client) SetRolePosition(ctx context.Context, guildID, roleID string, position int, reason string) error {
	body := []map[string]any{{"id": roleID, "position": position}}
	return c.do(ctx, call{Method: http.MethodPatch, Path: "/guilds/" + guildID + "/roles", Body: body, Reason: reason}, nil)
}

// DeleteRole deletes a role.
func (c *Client) DeleteRole(ctx context.Context, guildID, roleID, reason string) error {
	return c.do(ctx, call{Method: http.MethodDelete, Path: "/guilds/" + guildID + "/roles/" + roleID, Reason: reason}, nil)
}

// SearchMembers finds members whose username or nickname starts with query.
func (c *Client) SearchMembers(ctx context.Context, guildID, query string, limit int) ([]Member, error) {
	var out []Member
	q := url.Values{"query": {query}, "limit": {strconv.Itoa(limit)}}
	return out, c.do(ctx, call{Method: http.MethodGet, Path: "/guilds/" + guildID + "/members/search", Query: q}, &out)
}

// Member returns one member.
func (c *Client) Member(ctx context.Context, guildID, userID string) (*Member, error) {
	var m Member
	return &m, c.do(ctx, call{Method: http.MethodGet, Path: "/guilds/" + guildID + "/members/" + userID}, &m)
}

// AddMemberRole gives a role to a member.
func (c *Client) AddMemberRole(ctx context.Context, guildID, userID, roleID, reason string) error {
	return c.do(ctx, call{Method: http.MethodPut, Path: "/guilds/" + guildID + "/members/" + userID + "/roles/" + roleID, Reason: reason}, nil)
}

// RemoveMemberRole takes a role from a member.
func (c *Client) RemoveMemberRole(ctx context.Context, guildID, userID, roleID, reason string) error {
	return c.do(ctx, call{Method: http.MethodDelete, Path: "/guilds/" + guildID + "/members/" + userID + "/roles/" + roleID, Reason: reason}, nil)
}

// SendMessage posts a message. Mentions only notify when allowMentions is set.
func (c *Client) SendMessage(ctx context.Context, channelID, content string, allowMentions bool) (*Message, error) {
	body := map[string]any{"content": content}
	if !allowMentions {
		body["allowed_mentions"] = map[string]any{"parse": []string{}}
	}
	var m Message
	return &m, c.do(ctx, call{Method: http.MethodPost, Path: "/channels/" + channelID + "/messages", Body: body}, &m)
}

// InvitePerms is what the bot asks for by default: everything a server
// manager needs, except Administrator. Discord only lets a bot set channel
// overrides and role permissions it has itself, hence the long list.
var InvitePerms = AllPerms &^ (Administrator | ViewCreatorMonetizationAnalytics)

// InviteURL is the OAuth2 link that adds the bot to a server. With guildID,
// that server comes preselected.
func InviteURL(appID string, perms Perm, guildID string) string {
	q := url.Values{
		"client_id":   {appID},
		"scope":       {"bot"},
		"permissions": {strconv.FormatUint(uint64(perms), 10)},
	}
	if guildID != "" {
		q.Set("guild_id", guildID)
		q.Set("disable_guild_select", "true")
	}
	return "https://discord.com/oauth2/authorize?" + q.Encode()
}
