package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gabriellsalesx/discord-mcp/internal/discord"
)

var (
	verificationLevels  = []string{"none", "low", "medium", "high", "very_high"}
	notificationLevels  = []string{"all_messages", "only_mentions"}
	contentFilterLevels = []string{"disabled", "members_without_roles", "all_members"}
	afkTimeouts         = []int{60, 300, 900, 1800, 3600}
)

func levelName(names []string, v int) string {
	if v >= 0 && v < len(names) {
		return names[v]
	}
	return fmt.Sprint(v)
}

func levelValue(field string, names []string, s string) (int, error) {
	for i, n := range names {
		if strings.EqualFold(strings.ReplaceAll(strings.TrimSpace(s), " ", "_"), n) {
			return i, nil
		}
	}
	return 0, fmt.Errorf("%s %q is invalid; use one of %s", field, s, strings.Join(names, ", "))
}

func (s *Server) listServers(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	c, def, err := s.session()
	if err != nil {
		return nil, nil, err
	}
	guilds, err := c.Guilds(ctx)
	if err != nil {
		return nil, nil, err
	}
	type entry struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Members    int    `json:"members"`
		Online     int    `json:"online"`
		BotIsOwner bool   `json:"bot_is_owner,omitempty"`
		// What the bot can do there, server-wide.
		Administrator  bool `json:"administrator"`
		ManageChannels bool `json:"manage_channels"`
		ManageRoles    bool `json:"manage_roles"`
		ManageServer   bool `json:"manage_server"`
	}
	out := struct {
		Servers []entry `json:"servers"`
		Default string  `json:"default_server,omitempty"`
		Hint    string  `json:"hint,omitempty"`
	}{Servers: []entry{}, Default: def}
	for _, g := range guilds {
		admin := g.Owner || g.Permissions.Has(discord.Administrator)
		out.Servers = append(out.Servers, entry{
			ID: g.ID, Name: g.Name, Members: g.ApproximateMemberCount, Online: g.ApproximatePresenceCount,
			BotIsOwner:     g.Owner,
			Administrator:  admin,
			ManageChannels: admin || g.Permissions.Has(discord.ManageChannels),
			ManageRoles:    admin || g.Permissions.Has(discord.ManageRoles),
			ManageServer:   admin || g.Permissions.Has(discord.ManageGuild),
		})
	}
	if len(guilds) == 0 {
		out.Hint = "The bot is in no server. The user can run `discord-mcp invite` in a terminal to add it."
	}
	return nil, out, nil
}

func (s *Server) inviteBot(ctx context.Context, _ *mcp.CallToolRequest, in inviteIn) (*mcp.CallToolResult, any, error) {
	c, _, err := s.session()
	if err != nil {
		return nil, nil, err
	}
	id := strings.TrimSpace(in.ServerID)
	if id != "" && !snowflake.MatchString(id) {
		return nil, nil, fmt.Errorf("server_id must be the numeric server id; the bot cannot look up servers it is not in by name")
	}
	app, err := c.Application(ctx)
	if err != nil {
		return nil, nil, err
	}
	perms := discord.InvitePerms
	if in.Admin {
		perms = discord.Administrator
	}
	out := map[string]any{
		"url":  discord.InviteURL(app.ID, perms, id),
		"next": "Ask the user to open the link, pick the server and click Authorize, then call list_servers. Afterwards they should drag the bot's role above the roles it should manage (Server Settings > Roles).",
	}
	if guilds, err := c.Guilds(ctx); err == nil {
		for _, g := range guilds {
			if g.ID == id {
				out["note"] = "The bot is already in " + g.Name + "."
			}
		}
	}
	return nil, out, nil
}

func (s *Server) getServer(ctx context.Context, _ *mcp.CallToolRequest, in serverIn) (*mcp.CallToolResult, any, error) {
	sc, err := s.scope(ctx, in.Server)
	if err != nil {
		return nil, nil, err
	}
	chs, err := sc.loadChannels(ctx)
	if err != nil {
		return nil, nil, err
	}
	g := sc.g
	counts := map[string]int{}
	for _, ch := range chs {
		counts[typeName(ch.Type)]++
	}
	type botInfo struct {
		Name        string   `json:"name"`
		TopRole     string   `json:"top_role"`
		Position    int      `json:"top_role_position"`
		Permissions []string `json:"permissions"`
		Note        string   `json:"note"`
	}
	var bot *botInfo
	if m, err := sc.botMember(ctx); err == nil {
		roles := sc.rolesOf(m.Roles)
		perms := discord.Effective(g.ID, sc.everyone().Permissions, roles, m.User.ID, g.OwnerID, nil)
		bot = &botInfo{Name: m.User.DisplayName(), Permissions: permList(perms),
			Note: "The bot can only manage roles below its top role, and only grant permissions it has."}
		if top := sc.topRole(m.Roles); top != nil {
			bot.TopRole, bot.Position = top.Name, top.Position
		}
	}
	out := map[string]any{
		"id":                      g.ID,
		"name":                    g.Name,
		"description":             g.Description,
		"owner_id":                g.OwnerID,
		"members":                 g.ApproximateMemberCount,
		"online":                  g.ApproximatePresenceCount,
		"boost_tier":              g.PremiumTier,
		"boosts":                  g.PremiumSubscriptionCount,
		"verification_level":      levelName(verificationLevels, g.VerificationLevel),
		"default_notifications":   levelName(notificationLevels, g.DefaultMessageNotifications),
		"explicit_content_filter": levelName(contentFilterLevels, g.ExplicitContentFilter),
		"system_channel":          sc.channelLabel(g.SystemChannelID),
		"rules_channel":           sc.channelLabel(g.RulesChannelID),
		"afk_channel":             sc.channelLabel(g.AFKChannelID),
		"afk_timeout":             g.AFKTimeout,
		"preferred_locale":        g.PreferredLocale,
		"community":               containsFold(g.Features, "COMMUNITY"),
		"features":                g.Features,
		"channels":                counts,
		"roles":                   len(g.Roles),
	}
	if bot != nil {
		out["bot"] = bot
	}
	return nil, out, nil
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

func (s *Server) updateServer(ctx context.Context, _ *mcp.CallToolRequest, in updateServerIn) (*mcp.CallToolResult, any, error) {
	sc, err := s.scope(ctx, in.Server)
	if err != nil {
		return nil, nil, err
	}
	body := map[string]any{}
	if in.Name != "" {
		body["name"] = in.Name
	}
	if in.Description != nil {
		body["description"] = *in.Description
	}
	for _, f := range []struct {
		field, key, val string
		names           []string
	}{
		{"verification_level", "verification_level", in.VerificationLevel, verificationLevels},
		{"default_notifications", "default_message_notifications", in.DefaultNotifications, notificationLevels},
		{"explicit_content_filter", "explicit_content_filter", in.ContentFilter, contentFilterLevels},
	} {
		if f.val == "" {
			continue
		}
		v, err := levelValue(f.field, f.names, f.val)
		if err != nil {
			return nil, nil, err
		}
		body[f.key] = v
	}
	for _, f := range []struct {
		key, val string
		kind     func(int) bool
	}{
		{"system_channel_id", in.SystemChannel, isTextLike},
		{"rules_channel_id", in.RulesChannel, isTextLike},
		{"afk_channel_id", in.AFKChannel, isVoice},
	} {
		if f.val == "" {
			continue
		}
		if strings.EqualFold(f.val, "none") {
			body[f.key] = nil
			continue
		}
		ch, err := sc.channel(ctx, f.val, false, f.kind)
		if err != nil {
			return nil, nil, err
		}
		body[f.key] = ch.ID
	}
	if in.AFKTimeout != 0 {
		ok := false
		for _, t := range afkTimeouts {
			ok = ok || t == in.AFKTimeout
		}
		if !ok {
			return nil, nil, fmt.Errorf("afk_timeout must be one of 60, 300, 900, 1800 or 3600 seconds")
		}
		body["afk_timeout"] = in.AFKTimeout
	}
	if in.PreferredLocale != "" {
		body["preferred_locale"] = in.PreferredLocale
	}
	if len(body) == 0 {
		return nil, nil, fmt.Errorf("nothing to change; pass at least one setting")
	}
	g, err := sc.c.ModifyGuild(ctx, sc.g.ID, body, in.Reason)
	if err != nil {
		return nil, nil, err
	}
	changed := make([]string, 0, len(body))
	for k := range body {
		changed = append(changed, k)
	}
	return nil, map[string]any{"id": g.ID, "name": g.Name, "updated": changed}, nil
}
