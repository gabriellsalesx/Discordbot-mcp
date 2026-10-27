package server

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/gabriellsalesx/discord-mcp/internal/discord"
)

// scope is the server a tool call works on, with what it has loaded so far.
type scope struct {
	c        *discord.Client
	g        *discord.Guild
	channels []discord.Channel
	bot      *discord.Member
}

var (
	snowflake = regexp.MustCompile(`^\d{15,25}$`)
	mention   = regexp.MustCompile(`^<(#|@&|@!?)(\d{15,25})>$`)
)

// scope resolves the server selector: an id or a name, else the configured
// default, else the only server the bot is in.
func (s *Server) scope(ctx context.Context, selector string) (*scope, error) {
	c, def, err := s.session()
	if err != nil {
		return nil, err
	}
	sel := strings.TrimSpace(firstNonEmpty(selector, def))
	id := ""
	if snowflake.MatchString(sel) {
		id = sel
	} else {
		guilds, err := c.Guilds(ctx)
		if err != nil {
			return nil, err
		}
		if len(guilds) == 0 {
			return nil, fmt.Errorf("the bot is not in any server yet; run `discord-mcp invite` to add it")
		}
		var names []string
		for _, g := range guilds {
			names = append(names, g.Name)
		}
		switch {
		case sel == "" && len(guilds) == 1:
			id = guilds[0].ID
		case sel == "":
			return nil, fmt.Errorf("the bot is in %d servers; pass server (one of: %s)", len(guilds), quoteList(names))
		default:
			var hits []discord.PartialGuild
			for _, g := range guilds {
				if strings.EqualFold(g.Name, sel) {
					hits = append(hits, g)
				}
			}
			if len(hits) == 0 {
				for _, g := range guilds {
					if strings.Contains(strings.ToLower(g.Name), strings.ToLower(sel)) {
						hits = append(hits, g)
					}
				}
			}
			switch len(hits) {
			case 0:
				return nil, fmt.Errorf("no server named %q; the bot is in: %s", sel, quoteList(names))
			case 1:
				id = hits[0].ID
			default:
				return nil, fmt.Errorf("server %q is ambiguous; pass the id from list_servers", sel)
			}
		}
	}
	g, err := c.Guild(ctx, id)
	if err != nil {
		return nil, err
	}
	return &scope{c: c, g: g}, nil
}

func (sc *scope) loadChannels(ctx context.Context) ([]discord.Channel, error) {
	if sc.channels == nil {
		chs, err := sc.c.Channels(ctx, sc.g.ID)
		if err != nil {
			return nil, err
		}
		sc.channels = chs
	}
	return sc.channels, nil
}

func (sc *scope) channelByID(id string) *discord.Channel {
	for i := range sc.channels {
		if sc.channels[i].ID == id {
			return &sc.channels[i]
		}
	}
	return nil
}

// channel finds a channel by id, <#id> mention, #name or name. "category/name"
// picks between channels with the same name. With categoryOnly, only
// categories match; otherwise categories are skipped unless nothing else does.
//
// When a name is ambiguous and prefer is given, the one channel it accepts
// wins, so "geral" picks the text channel where only text makes sense.
func (sc *scope) channel(ctx context.Context, sel string, categoryOnly bool, prefer ...func(t int) bool) (*discord.Channel, error) {
	if _, err := sc.loadChannels(ctx); err != nil {
		return nil, err
	}
	sel = strings.TrimSpace(sel)
	if sel == "" {
		return nil, fmt.Errorf("a channel is required")
	}
	if m := mention.FindStringSubmatch(sel); m != nil && m[1] == "#" {
		sel = m[2]
	}
	if snowflake.MatchString(sel) {
		if ch := sc.channelByID(sel); ch != nil {
			return ch, nil
		}
		return nil, fmt.Errorf("no channel with id %s in %s", sel, sc.g.Name)
	}
	parent, name := "", strings.TrimPrefix(sel, "#")
	var hits []*discord.Channel
	find := func() {
		hits = hits[:0]
		for i := range sc.channels {
			ch := &sc.channels[i]
			if (ch.Type == discord.ChannelCategory) != categoryOnly {
				continue
			}
			if !sameName(ch.Name, name) {
				continue
			}
			if parent != "" {
				p := sc.channelByID(ch.ParentID)
				if p == nil || !sameName(p.Name, parent) {
					continue
				}
			}
			hits = append(hits, ch)
		}
	}
	find()
	if len(hits) == 0 && strings.Contains(name, "/") {
		i := strings.LastIndex(name, "/")
		parent, name = strings.TrimSpace(name[:i]), strings.TrimPrefix(strings.TrimSpace(name[i+1:]), "#")
		find()
	}
	if len(hits) == 0 && !categoryOnly {
		// Allow naming a category where any channel is accepted.
		for i := range sc.channels {
			if ch := &sc.channels[i]; ch.Type == discord.ChannelCategory && sameName(ch.Name, sel) {
				hits = append(hits, ch)
			}
		}
	}
	if len(hits) > 1 && len(prefer) > 0 {
		var kept []*discord.Channel
		for _, h := range hits {
			if prefer[0](h.Type) {
				kept = append(kept, h)
			}
		}
		if len(kept) == 1 {
			hits = kept
		}
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		kind := "channel"
		if categoryOnly {
			kind = "category"
		}
		return nil, fmt.Errorf("no %s named %q in %s; call list_channels to see them", kind, sel, sc.g.Name)
	}
	var opts []string
	for _, h := range hits {
		where := "no category"
		if p := sc.channelByID(h.ParentID); p != nil {
			where = p.Name
		}
		opts = append(opts, fmt.Sprintf("%s in %s (id %s)", h.Name, where, h.ID))
	}
	return nil, fmt.Errorf("%q matches several channels: %s; pass the id or category/name", sel, strings.Join(opts, "; "))
}

// sameName compares channel names the way people type them: case and the
// spaces Discord turns into hyphens in text channel names are ignored.
func sameName(a, b string) bool {
	norm := func(s string) string {
		s = strings.ToLower(strings.TrimSpace(s))
		return strings.ReplaceAll(s, " ", "-")
	}
	return norm(a) == norm(b)
}

// role finds a role by id, <@&id> mention, @name or name. "everyone" is the
// @everyone role, whose id is the server id.
func (sc *scope) role(sel string) (*discord.Role, error) {
	sel = strings.TrimSpace(sel)
	if m := mention.FindStringSubmatch(sel); m != nil && m[1] == "@&" {
		sel = m[2]
	}
	name := strings.TrimPrefix(sel, "@")
	if strings.EqualFold(name, "everyone") {
		sel = sc.g.ID
	}
	var hits []*discord.Role
	for i := range sc.g.Roles {
		r := &sc.g.Roles[i]
		if r.ID == sel || strings.EqualFold(r.Name, name) || strings.EqualFold(r.Name, sel) {
			hits = append(hits, r)
		}
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		return nil, fmt.Errorf("no role named %q in %s; call list_roles to see them", sel, sc.g.Name)
	}
	return nil, fmt.Errorf("%q matches %d roles; pass the role id from list_roles", sel, len(hits))
}

// everyone returns the @everyone role.
func (sc *scope) everyone() discord.Role {
	for _, r := range sc.g.Roles {
		if r.ID == sc.g.ID {
			return r
		}
	}
	return discord.Role{ID: sc.g.ID, Name: "@everyone"}
}

// member finds a member by id, <@id> mention or name. Names go through
// Discord's member search, which needs the Server Members intent.
func (sc *scope) member(ctx context.Context, sel string) (*discord.Member, error) {
	sel = strings.TrimSpace(sel)
	if m := mention.FindStringSubmatch(sel); m != nil && strings.HasPrefix(m[1], "@") && m[1] != "@&" {
		sel = m[2]
	}
	if snowflake.MatchString(sel) {
		return sc.c.Member(ctx, sc.g.ID, sel)
	}
	q := strings.TrimPrefix(sel, "@")
	found, err := sc.c.SearchMembers(ctx, sc.g.ID, q, 10)
	if err != nil {
		return nil, fmt.Errorf("member search failed (%w); enable Server Members Intent in the Developer Portal (Bot tab), or pass the user id", err)
	}
	var exact []discord.Member
	for _, m := range found {
		if strings.EqualFold(m.User.Username, q) || strings.EqualFold(m.User.GlobalName, q) || strings.EqualFold(m.Nick, q) {
			exact = append(exact, m)
		}
	}
	if len(exact) == 1 {
		return &exact[0], nil
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("no member matching %q in %s", q, sc.g.Name)
	}
	var opts []string
	for _, m := range found {
		opts = append(opts, fmt.Sprintf("%s (@%s, id %s)", memberName(m), m.User.Username, m.User.ID))
	}
	return nil, fmt.Errorf("no member is named exactly %q; candidates: %s. Pass the exact name or the user id", q, strings.Join(opts, "; "))
}

func memberName(m discord.Member) string {
	if m.Nick != "" {
		return m.Nick
	}
	return m.User.DisplayName()
}

// target is the role or member a permission overwrite applies to.
type target struct {
	ID    string
	Type  int
	Label string
	Role  *discord.Role
	// Member is set for member targets.
	Member *discord.Member
}

// target resolves an overwrite target. kind is "role", "member" or "" to try
// roles first, then members.
func (sc *scope) target(ctx context.Context, sel, kind string) (*target, error) {
	switch kind {
	case "", "role", "member":
	default:
		return nil, fmt.Errorf("target_type %q is invalid; use role or member", kind)
	}
	if kind != "member" {
		r, err := sc.role(sel)
		if err == nil {
			return &target{ID: r.ID, Type: discord.OverwriteRole, Label: roleLabel(r, sc.g.ID), Role: r}, nil
		}
		if kind == "role" {
			return nil, err
		}
	}
	m, err := sc.member(ctx, sel)
	if err != nil {
		if kind == "" {
			return nil, fmt.Errorf("%q is neither a role nor a member: %w", sel, err)
		}
		return nil, err
	}
	return &target{ID: m.User.ID, Type: discord.OverwriteMember, Label: "member " + memberName(*m), Member: m}, nil
}

func roleLabel(r *discord.Role, guildID string) string {
	if r.ID == guildID {
		return "@everyone"
	}
	return "@" + r.Name
}

// botMember returns the bot's own membership in this server.
func (sc *scope) botMember(ctx context.Context) (*discord.Member, error) {
	if sc.bot != nil {
		return sc.bot, nil
	}
	me, err := sc.c.Me(ctx)
	if err != nil {
		return nil, err
	}
	m, err := sc.c.Member(ctx, sc.g.ID, me.ID)
	if err != nil {
		return nil, err
	}
	sc.bot = m
	return m, nil
}

// topRole is the highest of the given roles, or nil when there is none
// besides @everyone.
func (sc *scope) topRole(ids []string) *discord.Role {
	var top *discord.Role
	for i := range sc.g.Roles {
		r := &sc.g.Roles[i]
		if r.ID != sc.g.ID && slices.Contains(ids, r.ID) && (top == nil || discord.RoleAbove(*r, *top)) {
			top = r
		}
	}
	return top
}

// botCanManage reports whether a role sits below the bot's top role, which
// is all Discord checks, even for administrators.
func botCanManage(r discord.Role, top *discord.Role) bool {
	return top != nil && discord.RoleAbove(*top, r)
}

// rolesOf returns the role objects for ids, without @everyone.
func (sc *scope) rolesOf(ids []string) []discord.Role {
	var out []discord.Role
	for _, r := range sc.g.Roles {
		if slices.Contains(ids, r.ID) && r.ID != sc.g.ID {
			out = append(out, r)
		}
	}
	return out
}

// channelLabel names a channel for output.
func (sc *scope) channelLabel(id string) string {
	if ch := sc.channelByID(id); ch != nil {
		return ch.Name
	}
	return id
}
