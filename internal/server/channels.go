package server

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gabriellsalesx/discord-mcp/internal/discord"
)

type channelSummary struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Position   int    `json:"position"`
	Topic      string `json:"topic,omitempty"`
	Private    bool   `json:"private,omitempty"`
	Synced     *bool  `json:"synced_with_category,omitempty"`
	Overwrites int    `json:"permission_overrides,omitempty"`
	NSFW       bool   `json:"nsfw,omitempty"`
	Slowmode   int    `json:"slowmode_seconds,omitempty"`
	UserLimit  int    `json:"user_limit,omitempty"`
}

type overwriteView struct {
	Target string   `json:"target"`
	Type   string   `json:"type"`
	ID     string   `json:"id"`
	Allow  []string `json:"allow"`
	Deny   []string `json:"deny"`
}

// sortChannels orders channels as the Discord sidebar does: by position, then
// id, with voice-like channels after text-like ones.
func sortChannels(chs []discord.Channel) {
	slices.SortStableFunc(chs, func(a, b discord.Channel) int {
		if va, vb := isVoice(a.Type), isVoice(b.Type); va != vb {
			if va {
				return 1
			}
			return -1
		}
		return cmp.Or(cmp.Compare(a.Position, b.Position), cmp.Compare(len(a.ID), len(b.ID)), strings.Compare(a.ID, b.ID))
	})
}

// isPrivate reports whether @everyone is denied View Channel.
func isPrivate(ch *discord.Channel, guildID string) bool {
	for _, o := range ch.Overwrites {
		if o.ID == guildID && o.Type == discord.OverwriteRole {
			return o.Deny.Has(discord.ViewChannel)
		}
	}
	return false
}

// sameOverwrites reports whether two channels have identical overrides, which
// is what Discord means by a channel being synced with its category.
func sameOverwrites(a, b []discord.Overwrite) bool {
	if len(a) != len(b) {
		return false
	}
	for _, x := range a {
		i := slices.IndexFunc(b, func(y discord.Overwrite) bool { return y.ID == x.ID })
		if i < 0 || b[i].Allow != x.Allow || b[i].Deny != x.Deny || b[i].Type != x.Type {
			return false
		}
	}
	return true
}

func (sc *scope) summarize(ch *discord.Channel) channelSummary {
	s := channelSummary{
		ID: ch.ID, Name: ch.Name, Type: typeName(ch.Type), Position: ch.Position,
		Topic: truncate(ch.Topic, 120), Private: isPrivate(ch, sc.g.ID), Overwrites: len(ch.Overwrites),
		NSFW: ch.NSFW, Slowmode: ch.RateLimitPerUser, UserLimit: ch.UserLimit,
	}
	if p := sc.channelByID(ch.ParentID); p != nil {
		s.Synced = new(sameOverwrites(ch.Overwrites, p.Overwrites))
	}
	return s
}

func (sc *scope) overwriteViews(ctx context.Context, os []discord.Overwrite, lookupMembers bool) []overwriteView {
	out := []overwriteView{}
	for _, o := range os {
		v := overwriteView{ID: o.ID, Allow: o.Allow.Names(), Deny: o.Deny.Names()}
		if o.Type == discord.OverwriteRole {
			v.Type, v.Target = "role", o.ID
			if r, err := sc.role(o.ID); err == nil {
				v.Target = roleLabel(r, sc.g.ID)
			}
		} else {
			v.Type, v.Target = "member", o.ID
			if lookupMembers {
				if m, err := sc.c.Member(ctx, sc.g.ID, o.ID); err == nil {
					v.Target = memberName(*m)
				}
			}
		}
		out = append(out, v)
	}
	return out
}

func (s *Server) listChannels(ctx context.Context, _ *mcp.CallToolRequest, in serverIn) (*mcp.CallToolResult, any, error) {
	sc, err := s.scope(ctx, in.Server)
	if err != nil {
		return nil, nil, err
	}
	chs, err := sc.loadChannels(ctx)
	if err != nil {
		return nil, nil, err
	}
	type category struct {
		ID       string           `json:"id"`
		Name     string           `json:"name"`
		Position int              `json:"position"`
		Private  bool             `json:"private,omitempty"`
		Channels []channelSummary `json:"channels"`
	}
	sorted := slices.Clone(chs)
	sortChannels(sorted)
	out := struct {
		Server        string           `json:"server"`
		Uncategorized []channelSummary `json:"uncategorized"`
		Categories    []category       `json:"categories"`
	}{Server: sc.g.Name, Uncategorized: []channelSummary{}, Categories: []category{}}
	index := map[string]int{}
	for i := range sorted {
		if ch := &sorted[i]; ch.Type == discord.ChannelCategory {
			index[ch.ID] = len(out.Categories)
			out.Categories = append(out.Categories, category{ID: ch.ID, Name: ch.Name, Position: ch.Position, Private: isPrivate(ch, sc.g.ID), Channels: []channelSummary{}})
		}
	}
	for i := range sorted {
		ch := &sorted[i]
		if ch.Type == discord.ChannelCategory {
			continue
		}
		if k, ok := index[ch.ParentID]; ok {
			out.Categories[k].Channels = append(out.Categories[k].Channels, sc.summarize(ch))
		} else {
			out.Uncategorized = append(out.Uncategorized, sc.summarize(ch))
		}
	}
	return nil, out, nil
}

func (s *Server) getChannel(ctx context.Context, _ *mcp.CallToolRequest, in channelIn) (*mcp.CallToolResult, any, error) {
	sc, err := s.scope(ctx, in.Server)
	if err != nil {
		return nil, nil, err
	}
	ch, err := sc.channel(ctx, in.Channel, false)
	if err != nil {
		return nil, nil, err
	}
	out := map[string]any{
		"id":                   ch.ID,
		"name":                 ch.Name,
		"type":                 typeName(ch.Type),
		"position":             ch.Position,
		"topic":                ch.Topic,
		"nsfw":                 ch.NSFW,
		"slowmode_seconds":     ch.RateLimitPerUser,
		"private":              isPrivate(ch, sc.g.ID),
		"permission_overrides": sc.overwriteViews(ctx, ch.Overwrites, true),
		"url":                  "https://discord.com/channels/" + sc.g.ID + "/" + ch.ID,
	}
	if isVoice(ch.Type) {
		out["bitrate"], out["user_limit"] = ch.Bitrate, ch.UserLimit
	}
	if p := sc.channelByID(ch.ParentID); p != nil {
		out["category"] = p.Name
		out["synced_with_category"] = sameOverwrites(ch.Overwrites, p.Overwrites)
	}
	if ch.Type == discord.ChannelCategory {
		var children []string
		for _, c := range sc.channels {
			if c.ParentID == ch.ID {
				children = append(children, c.Name)
			}
		}
		out["channels"] = children
	}
	return nil, out, nil
}

func (s *Server) createChannel(ctx context.Context, _ *mcp.CallToolRequest, in createChannelIn) (*mcp.CallToolResult, any, error) {
	sc, err := s.scope(ctx, in.Server)
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, nil, fmt.Errorf("name is required")
	}
	typ, ok := channelTypes[strings.ToLower(firstNonEmpty(strings.TrimSpace(in.Type), "text"))]
	if !ok {
		return nil, nil, fmt.Errorf("type %q is invalid; use text, voice, category, announcement, stage, forum or media", in.Type)
	}
	body := map[string]any{"name": in.Name, "type": typ}
	var overwrites []discord.Overwrite
	if in.Category != "" && typ != discord.ChannelCategory {
		parent, err := sc.channel(ctx, in.Category, true)
		if err != nil {
			return nil, nil, err
		}
		// Overwrites are left to Discord: copying the category's would need
		// the bot to hold every bit they touch.
		body["parent_id"] = parent.ID
	}
	if in.Topic != "" {
		body["topic"] = in.Topic
	}
	if in.NSFW {
		body["nsfw"] = true
	}
	if in.Slowmode > 0 {
		body["rate_limit_per_user"] = in.Slowmode
	}
	if in.Bitrate > 0 {
		body["bitrate"] = in.Bitrate
	}
	if in.UserLimit > 0 {
		body["user_limit"] = in.UserLimit
	}
	if in.Position != nil {
		body["position"] = *in.Position
	}
	var visible []string
	if in.Private {
		see := discord.ViewChannel
		if isVoice(typ) {
			see |= discord.Connect
		}
		overwrites = setOverwrite(overwrites, discord.Overwrite{ID: sc.g.ID, Type: discord.OverwriteRole}, 0, see)
		for _, v := range in.VisibleTo {
			t, err := sc.target(ctx, v, "")
			if err != nil {
				return nil, nil, err
			}
			overwrites = setOverwrite(overwrites, discord.Overwrite{ID: t.ID, Type: t.Type}, see, 0)
			visible = append(visible, t.Label)
		}
		// Keep the bot able to see and manage the channel it hides. A bot
		// may only set bits it has itself.
		m, err := sc.botMember(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("checking the bot's own access before hiding the channel: %w", err)
		}
		if !s.botIsAdmin(sc, m) {
			have := discord.Effective(sc.g.ID, sc.everyone().Permissions, sc.rolesOf(m.Roles), m.User.ID, sc.g.OwnerID, nil)
			if !have.Has(see) {
				return nil, nil, fmt.Errorf("the bot needs %v server-wide to create a private channel it can still see", (see &^ have).Names())
			}
			overwrites = setOverwrite(overwrites, discord.Overwrite{ID: m.User.ID, Type: discord.OverwriteMember}, (see|discord.ManageChannels)&have, 0)
		}
	} else if len(in.VisibleTo) > 0 {
		return nil, nil, fmt.Errorf("visible_to only applies with private=true")
	}
	if overwrites != nil {
		body["permission_overwrites"] = overwrites
	}
	ch, err := sc.c.CreateChannel(ctx, sc.g.ID, body, in.Reason)
	if err != nil {
		return nil, nil, err
	}
	out := map[string]any{
		"id":      ch.ID,
		"name":    ch.Name,
		"type":    typeName(ch.Type),
		"private": in.Private,
		"url":     "https://discord.com/channels/" + sc.g.ID + "/" + ch.ID,
	}
	if p := sc.channelByID(ch.ParentID); p != nil {
		out["category"] = p.Name
		out["synced_with_category"] = sameOverwrites(ch.Overwrites, p.Overwrites)
	}
	if len(visible) > 0 {
		out["visible_to"] = visible
	}
	return nil, out, nil
}

func (s *Server) botIsAdmin(sc *scope, m *discord.Member) bool {
	p := discord.Effective(sc.g.ID, sc.everyone().Permissions, sc.rolesOf(m.Roles), m.User.ID, sc.g.OwnerID, nil)
	return p.Has(discord.Administrator) || p == discord.AllPerms
}

// setOverwrite merges allow and deny into the target's override in list,
// adding it when missing. A permission moves out of deny when allowed, and
// the other way round.
func setOverwrite(list []discord.Overwrite, t discord.Overwrite, allow, deny discord.Perm) []discord.Overwrite {
	i := slices.IndexFunc(list, func(o discord.Overwrite) bool { return o.ID == t.ID })
	if i < 0 {
		list = append(list, t)
		i = len(list) - 1
	}
	o := &list[i]
	o.Allow = o.Allow&^deny | allow
	o.Deny = o.Deny&^allow | deny
	return list
}

func (s *Server) updateChannel(ctx context.Context, _ *mcp.CallToolRequest, in updateChannelIn) (*mcp.CallToolResult, any, error) {
	sc, err := s.scope(ctx, in.Server)
	if err != nil {
		return nil, nil, err
	}
	ch, err := sc.channel(ctx, in.Channel, false)
	if err != nil {
		return nil, nil, err
	}
	body := map[string]any{}
	if in.Name != "" {
		body["name"] = in.Name
	}
	if in.Topic != nil {
		body["topic"] = *in.Topic
	}
	parentID := ch.ParentID
	if in.Category != "" {
		if ch.Type == discord.ChannelCategory {
			return nil, nil, fmt.Errorf("a category cannot be placed inside another category")
		}
		if strings.EqualFold(in.Category, "none") {
			body["parent_id"], parentID = nil, ""
		} else {
			p, err := sc.channel(ctx, in.Category, true)
			if err != nil {
				return nil, nil, err
			}
			body["parent_id"], parentID = p.ID, p.ID
		}
	}
	if in.Sync {
		p := sc.channelByID(parentID)
		if p == nil {
			return nil, nil, fmt.Errorf("sync_permissions needs the channel to be in a category")
		}
		body["permission_overwrites"] = nonNil(p.Overwrites)
	}
	if in.Position != nil {
		body["position"] = *in.Position
	}
	if in.NSFW != nil {
		body["nsfw"] = *in.NSFW
	}
	if in.Slowmode != nil {
		body["rate_limit_per_user"] = *in.Slowmode
	}
	if in.Bitrate > 0 {
		body["bitrate"] = in.Bitrate
	}
	if in.UserLimit != nil {
		body["user_limit"] = *in.UserLimit
	}
	if len(body) == 0 {
		return nil, nil, fmt.Errorf("nothing to change; pass at least one field")
	}
	updated, err := sc.c.ModifyChannel(ctx, ch.ID, body, in.Reason)
	if err != nil {
		return nil, nil, err
	}
	out := map[string]any{"id": updated.ID, "name": updated.Name, "type": typeName(updated.Type), "position": updated.Position}
	if p := sc.channelByID(updated.ParentID); p != nil {
		out["category"] = p.Name
		out["synced_with_category"] = sameOverwrites(updated.Overwrites, p.Overwrites)
	}
	return nil, out, nil
}

func nonNil(o []discord.Overwrite) []discord.Overwrite {
	if o == nil {
		return []discord.Overwrite{}
	}
	return o
}

func (s *Server) deleteChannel(ctx context.Context, _ *mcp.CallToolRequest, in deleteChannelIn) (*mcp.CallToolResult, any, error) {
	sc, err := s.scope(ctx, in.Server)
	if err != nil {
		return nil, nil, err
	}
	ch, err := sc.channel(ctx, in.Channel, false)
	if err != nil {
		return nil, nil, err
	}
	details := map[string]any{"server": sc.g.Name, "id": ch.ID, "name": ch.Name, "type": typeName(ch.Type)}
	if p := sc.channelByID(ch.ParentID); p != nil {
		details["category"] = p.Name
	}
	if ch.Type == discord.ChannelCategory {
		var kept []string
		for _, c := range sc.channels {
			if c.ParentID == ch.ID {
				kept = append(kept, c.Name)
			}
		}
		details["channels_left_without_category"] = kept
	} else {
		details["warning"] = "All messages in this channel are lost for good."
	}
	if !in.Confirm {
		return nil, confirmPreview("delete "+typeName(ch.Type)+" "+ch.Name, details), nil
	}
	if err := sc.c.DeleteChannel(ctx, ch.ID, in.Reason); err != nil {
		return nil, nil, err
	}
	return nil, map[string]any{"deleted": true, "id": ch.ID, "name": ch.Name}, nil
}

func (s *Server) organizeChannels(ctx context.Context, _ *mcp.CallToolRequest, in organizeIn) (*mcp.CallToolResult, any, error) {
	sc, err := s.scope(ctx, in.Server)
	if err != nil {
		return nil, nil, err
	}
	if len(in.Channels) == 0 {
		return nil, nil, fmt.Errorf("channels is required: list them in the order they should appear")
	}
	if _, err := sc.loadChannels(ctx); err != nil {
		return nil, nil, err
	}
	// Names are read as channels; if that does not give a single kind, as
	// categories, so a category named like a channel can still be ordered.
	resolveAll := func(categoryOnly bool) ([]*discord.Channel, int, error) {
		var out []*discord.Channel
		cats := 0
		for _, sel := range in.Channels {
			ch, err := sc.channel(ctx, sel, categoryOnly)
			if err != nil {
				return nil, 0, err
			}
			if ch.Type == discord.ChannelCategory {
				cats++
			}
			out = append(out, ch)
		}
		return out, cats, nil
	}
	listed, categories, err := resolveAll(false)
	if err == nil && categories > 0 && categories < len(listed) {
		if l, c, err2 := resolveAll(true); err2 == nil {
			listed, categories = l, c
		}
	}
	if err != nil {
		if l, c, err2 := resolveAll(true); err2 == nil {
			listed, categories, err = l, c, nil
		}
	}
	if err != nil {
		return nil, nil, err
	}
	seen := map[string]bool{}
	for _, ch := range listed {
		if seen[ch.ID] {
			return nil, nil, fmt.Errorf("%s is listed twice", ch.Name)
		}
		seen[ch.ID] = true
	}
	if categories > 0 && categories < len(listed) {
		return nil, nil, fmt.Errorf("list either categories or channels, not both; order categories in one call and each category's channels in another")
	}
	if categories > 0 && in.Category != "" {
		return nil, nil, fmt.Errorf("categories cannot be moved into a category")
	}

	// Move first. Discord takes one parent change per request.
	var moved []string
	if in.Category != "" {
		dest := ""
		if !strings.EqualFold(in.Category, "none") {
			p, err := sc.channel(ctx, in.Category, true)
			if err != nil {
				return nil, nil, err
			}
			dest = p.ID
		}
		parent := json.RawMessage("null")
		if dest != "" {
			parent, _ = json.Marshal(dest)
		}
		for _, ch := range listed {
			if ch.ParentID == dest {
				continue
			}
			pos := discord.ChannelPosition{ID: ch.ID, ParentID: parent}
			if in.Sync {
				pos.LockPermissions = new(true)
			}
			if err := sc.c.SetChannelPositions(ctx, sc.g.ID, []discord.ChannelPosition{pos}, in.Reason); err != nil {
				return nil, nil, fmt.Errorf("moving %s: %w (moved so far: %s)", ch.Name, err, strings.Join(moved, ", "))
			}
			ch.ParentID = dest
			moved = append(moved, ch.Name)
		}
	}

	// The group being ordered: all categories, or every channel that shares
	// the first listed channel's category.
	var group []discord.Channel
	for _, ch := range sc.channels {
		if categories > 0 {
			if ch.Type == discord.ChannelCategory {
				group = append(group, ch)
			}
		} else if ch.Type != discord.ChannelCategory && ch.ParentID == listed[0].ParentID {
			group = append(group, ch)
		}
	}
	for _, ch := range listed[1:] {
		if ch.ParentID != listed[0].ParentID {
			return nil, nil, fmt.Errorf("%s and %s are in different categories; pass category to move them together, or order each category separately", listed[0].Name, ch.Name)
		}
	}
	sortChannels(group)
	order := make([]string, 0, len(group))
	for _, ch := range listed {
		order = append(order, ch.ID)
	}
	for _, ch := range group {
		if !seen[ch.ID] {
			order = append(order, ch.ID)
		}
	}
	pos := make([]discord.ChannelPosition, len(order))
	names := make([]string, len(order))
	for i, id := range order {
		pos[i] = discord.ChannelPosition{ID: id, Position: new(i)}
		names[i] = sc.channelLabel(id)
	}
	if err := sc.c.SetChannelPositions(ctx, sc.g.ID, pos, in.Reason); err != nil {
		return nil, nil, err
	}
	out := map[string]any{"order": names}
	if len(moved) > 0 {
		out["moved"] = moved
	}
	if categories == 0 {
		out["category"] = firstNonEmpty(sc.channelLabelOrEmpty(listed[0].ParentID), "(none)")
	}
	return nil, out, nil
}

func (sc *scope) channelLabelOrEmpty(id string) string {
	if ch := sc.channelByID(id); ch != nil {
		return ch.Name
	}
	return ""
}

// truncate shortens s to at most n runes.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
