package server

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gabriellsalesx/discord-mcp/internal/discord"
)

type memberView struct {
	ID       string   `json:"id"`
	Username string   `json:"username"`
	Name     string   `json:"name"`
	Bot      bool     `json:"bot,omitempty"`
	Roles    []string `json:"roles"`
	JoinedAt string   `json:"joined_at,omitempty"`
}

func (s *Server) findMembers(ctx context.Context, _ *mcp.CallToolRequest, in findMembersIn) (*mcp.CallToolResult, any, error) {
	sc, err := s.scope(ctx, in.Server)
	if err != nil {
		return nil, nil, err
	}
	q := strings.TrimPrefix(strings.TrimSpace(in.Query), "@")
	if q == "" {
		return nil, nil, fmt.Errorf("query is required")
	}
	found, err := sc.c.SearchMembers(ctx, sc.g.ID, q, clamp(in.Limit, 10, 1, 50))
	if err != nil {
		return nil, nil, fmt.Errorf("%w (member search needs Server Members Intent: Developer Portal > your app > Bot > Privileged Gateway Intents)", err)
	}
	out := []memberView{}
	for _, m := range found {
		v := memberView{ID: m.User.ID, Username: m.User.Username, Name: memberName(m), Bot: m.User.Bot, Roles: []string{}}
		for _, r := range sc.rolesOf(m.Roles) {
			v.Roles = append(v.Roles, r.Name)
		}
		if !m.JoinedAt.IsZero() {
			v.JoinedAt = m.JoinedAt.Format("2006-01-02")
		}
		out = append(out, v)
	}
	return nil, map[string]any{"server": sc.g.Name, "members": out}, nil
}

func (s *Server) setMemberRoles(ctx context.Context, _ *mcp.CallToolRequest, in memberRolesIn) (*mcp.CallToolResult, any, error) {
	sc, err := s.scope(ctx, in.Server)
	if err != nil {
		return nil, nil, err
	}
	if len(in.Add) == 0 && len(in.Remove) == 0 {
		return nil, nil, fmt.Errorf("pass roles to add or remove")
	}
	m, err := sc.member(ctx, in.Member)
	if err != nil {
		return nil, nil, err
	}
	var added, removed, unchanged []string
	apply := func(sels []string, give bool) error {
		for _, sel := range sels {
			r, err := sc.role(sel)
			if err != nil {
				return err
			}
			if r.ID == sc.g.ID {
				return fmt.Errorf("@everyone is not assignable; every member has it")
			}
			if r.Managed {
				return fmt.Errorf("%s is managed by an integration and cannot be assigned by hand", r.Name)
			}
			has := slices.Contains(m.Roles, r.ID)
			if has == give {
				unchanged = append(unchanged, r.Name)
				continue
			}
			if give {
				err = sc.c.AddMemberRole(ctx, sc.g.ID, m.User.ID, r.ID, in.Reason)
			} else {
				err = sc.c.RemoveMemberRole(ctx, sc.g.ID, m.User.ID, r.ID, in.Reason)
			}
			if err != nil {
				return fmt.Errorf("%s: %w", r.Name, err)
			}
			if give {
				added = append(added, r.Name)
				m.Roles = append(m.Roles, r.ID)
			} else {
				removed = append(removed, r.Name)
				m.Roles = slices.DeleteFunc(m.Roles, func(id string) bool { return id == r.ID })
			}
		}
		return nil
	}
	if err := apply(in.Add, true); err != nil {
		return nil, nil, err
	}
	if err := apply(in.Remove, false); err != nil {
		return nil, nil, fmt.Errorf("added %v, then failed: %w", added, err)
	}
	roles := []string{}
	for _, r := range sc.rolesOf(m.Roles) {
		roles = append(roles, r.Name)
	}
	out := map[string]any{"member": memberName(*m), "id": m.User.ID, "roles": roles}
	if len(added) > 0 {
		out["added"] = added
	}
	if len(removed) > 0 {
		out["removed"] = removed
	}
	if len(unchanged) > 0 {
		out["already_so"] = unchanged
	}
	return nil, out, nil
}

func (s *Server) sendMessage(ctx context.Context, _ *mcp.CallToolRequest, in sendMessageIn) (*mcp.CallToolResult, any, error) {
	sc, err := s.scope(ctx, in.Server)
	if err != nil {
		return nil, nil, err
	}
	ch, err := sc.channel(ctx, in.Channel, false, isMessageable)
	if err != nil {
		return nil, nil, err
	}
	content := strings.TrimSpace(in.Content)
	if content == "" {
		return nil, nil, fmt.Errorf("content is required")
	}
	if n := utf8.RuneCountInString(content); n > 2000 {
		return nil, nil, fmt.Errorf("content has %d characters; Discord allows 2000 per message", n)
	}
	// Check up front: a channel that denies @everyone sending often denies
	// the bot too, and Discord's own error does not say where.
	if m, err := sc.botMember(ctx); err == nil {
		have := discord.Effective(sc.g.ID, sc.everyone().Permissions, sc.rolesOf(m.Roles), m.User.ID, sc.g.OwnerID, ch)
		if !have.Has(discord.ViewChannel | discord.SendMessages) {
			return nil, nil, fmt.Errorf("the bot cannot post in #%s: its permissions there do not include view_channel and send_messages. Allow them for the bot with set_channel_permissions (target %q, target_type member)", ch.Name, m.User.ID)
		}
	}
	if !in.Confirm {
		return nil, confirmPreview("post a message in #"+ch.Name, map[string]any{
			"server":          sc.g.Name,
			"channel":         ch.Name,
			"content":         content,
			"mentions_notify": in.AllowMentions,
		}), nil
	}
	msg, err := sc.c.SendMessage(ctx, ch.ID, content, in.AllowMentions)
	if err != nil {
		return nil, nil, err
	}
	return nil, map[string]any{
		"sent": true,
		"id":   msg.ID,
		"url":  "https://discord.com/channels/" + sc.g.ID + "/" + ch.ID + "/" + msg.ID,
	}, nil
}
