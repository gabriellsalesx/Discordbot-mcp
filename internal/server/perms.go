package server

import (
	"context"
	"fmt"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gabriellsalesx/Discordbot-mcp/internal/discord"
)

type permPair struct {
	Allow []string `json:"allow"`
	Deny  []string `json:"deny"`
}

func pairOf(o discord.Overwrite) permPair {
	return permPair{Allow: o.Allow.Names(), Deny: o.Deny.Names()}
}

func findOverwrite(list []discord.Overwrite, id string) (discord.Overwrite, bool) {
	i := slices.IndexFunc(list, func(o discord.Overwrite) bool { return o.ID == id })
	if i < 0 {
		return discord.Overwrite{}, false
	}
	return list[i], true
}

func (s *Server) setChannelPermissions(ctx context.Context, _ *mcp.CallToolRequest, in setPermsIn) (*mcp.CallToolResult, any, error) {
	sc, err := s.scope(ctx, in.Server)
	if err != nil {
		return nil, nil, err
	}
	ch, err := sc.channel(ctx, in.Channel, false)
	if err != nil {
		return nil, nil, err
	}
	t, err := sc.target(ctx, in.Target, in.TargetType)
	if err != nil {
		return nil, nil, err
	}
	allow, err := discord.ParsePerms(in.Allow)
	if err != nil {
		return nil, nil, err
	}
	deny, err := discord.ParsePerms(in.Deny)
	if err != nil {
		return nil, nil, err
	}
	reset, err := discord.ParsePerms(in.Reset)
	if err != nil {
		return nil, nil, err
	}
	if both := allow & deny; both != 0 {
		return nil, nil, fmt.Errorf("%v cannot be both allowed and denied", both.Names())
	}
	if !in.Remove && allow|deny|reset == 0 && !in.Replace {
		return nil, nil, fmt.Errorf("pass allow, deny or reset (or remove=true to drop the override)")
	}

	before, existed := findOverwrite(ch.Overwrites, t.ID)
	after := discord.Overwrite{ID: t.ID, Type: t.Type}
	if !in.Remove {
		start := before
		if in.Replace {
			start = discord.Overwrite{}
		}
		after.Allow = start.Allow&^(deny|reset) | allow
		after.Deny = start.Deny&^(allow|reset) | deny
	}
	if after.Allow == 0 && after.Deny == 0 && !existed {
		return nil, map[string]any{"channel": ch.Name, "target": t.Label, "changed": false, "note": "There was no override to change."}, nil
	}

	// The channels to write: this one, plus, on a category, the channels
	// that were synced with it, so they stay synced as in the Discord app.
	targets := []*discord.Channel{ch}
	if ch.Type == discord.ChannelCategory {
		for i := range sc.channels {
			child := &sc.channels[i]
			if child.ParentID == ch.ID && sameOverwrites(child.Overwrites, ch.Overwrites) {
				targets = append(targets, child)
			}
		}
	}
	// Denying @everyone what the bot relies on would lock the bot out too
	// (its role allow does not beat a channel deny). Give the bot its own
	// override for those bits, as the Discord app does for its creator.
	var keep discord.Perm
	if t.ID == sc.g.ID {
		keep = after.Deny & (discord.ViewChannel | discord.SendMessages | discord.ReadMessageHistory | discord.Connect)
	}
	var bot *discord.Member
	if keep != 0 {
		if m, err := sc.botMember(ctx); err == nil && !s.botIsAdmin(sc, m) {
			bot = m
			have := discord.Effective(sc.g.ID, sc.everyone().Permissions, sc.rolesOf(m.Roles), m.User.ID, sc.g.OwnerID, nil)
			keep &= have
		} else {
			keep = 0
		}
	}

	var updated []string
	for _, c := range targets {
		// The bot's override goes first: once @everyone is denied, the bot
		// can no longer see the channel to add it.
		if keep != 0 {
			o, _ := findOverwrite(c.Overwrites, bot.User.ID)
			o.ID, o.Type = bot.User.ID, discord.OverwriteMember
			o.Allow, o.Deny = o.Allow|keep, o.Deny&^keep
			if err := sc.c.SetOverwrite(ctx, c.ID, o, in.Reason); err != nil {
				return nil, nil, fmt.Errorf("could not keep the bot's own access to %s, so nothing was changed there: %w", c.Name, err)
			}
		}
		if after.Allow == 0 && after.Deny == 0 {
			err = sc.c.DeleteOverwrite(ctx, c.ID, t.ID, in.Reason)
		} else {
			err = sc.c.SetOverwrite(ctx, c.ID, after, in.Reason)
		}
		if err != nil {
			if c == ch {
				return nil, nil, err
			}
			return nil, nil, fmt.Errorf("updated %s but not the synced channel %s: %w", ch.Name, c.Name, err)
		}
		if c != ch {
			updated = append(updated, c.Name)
		}
	}

	out := map[string]any{
		"channel": ch.Name,
		"target":  t.Label,
		"before":  pairOf(before),
		"after":   pairOf(after),
	}
	if after.Allow == 0 && after.Deny == 0 {
		out["override_removed"] = true
	}
	if len(updated) > 0 {
		out["synced_channels_updated"] = updated
	}
	if keep != 0 {
		out["bot_kept_access"] = keep.Names()
	}
	if p := sc.channelByID(ch.ParentID); p != nil && sameOverwrites(ch.Overwrites, p.Overwrites) {
		next := slices.DeleteFunc(slices.Clone(ch.Overwrites), func(o discord.Overwrite) bool { return o.ID == t.ID })
		if after.Allow != 0 || after.Deny != 0 {
			next = append(next, after)
		}
		if !sameOverwrites(next, p.Overwrites) {
			out["note"] = "This channel no longer matches its category's permissions (it is now unsynced)."
		}
	}
	return nil, out, nil
}

// Permissions worth calling out per kind of channel.
var (
	textKeys  = []discord.Perm{discord.ViewChannel, discord.SendMessages, discord.ReadMessageHistory, discord.AttachFiles, discord.EmbedLinks, discord.AddReactions, discord.CreatePublicThreads, discord.SendMessagesInThreads, discord.MentionEveryone, discord.ManageMessages}
	voiceKeys = []discord.Perm{discord.ViewChannel, discord.Connect, discord.Speak, discord.Stream, discord.UseVAD, discord.SendMessages, discord.MuteMembers, discord.MoveMembers}
)

func (s *Server) checkPermissions(ctx context.Context, _ *mcp.CallToolRequest, in checkPermsIn) (*mcp.CallToolResult, any, error) {
	sc, err := s.scope(ctx, in.Server)
	if err != nil {
		return nil, nil, err
	}
	t, err := sc.target(ctx, in.Target, in.TargetType)
	if err != nil {
		return nil, nil, err
	}
	var roles []discord.Role
	member := ""
	if t.Member != nil {
		roles, member = sc.rolesOf(t.Member.Roles), t.Member.User.ID
	} else if t.Role.ID != sc.g.ID {
		roles = []discord.Role{*t.Role}
	}
	var ch *discord.Channel
	if in.Channel != "" {
		if ch, err = sc.channel(ctx, in.Channel, false); err != nil {
			return nil, nil, err
		}
	}
	perm := discord.Effective(sc.g.ID, sc.everyone().Permissions, roles, member, sc.g.OwnerID, ch)
	out := map[string]any{"target": t.Label, "permissions": permList(perm)}
	if perm == discord.AllPerms {
		out["permissions"] = []string{"all (administrator or server owner)"}
	}
	if t.Member != nil {
		var names []string
		for _, r := range roles {
			names = append(names, r.Name)
		}
		out["roles"] = names
	}
	if ch != nil {
		out["channel"] = ch.Name
		keys := textKeys
		if isVoice(ch.Type) {
			keys = voiceKeys
		}
		summary := map[string]bool{}
		for _, k := range keys {
			summary[k.Names()[0]] = perm.Has(k)
		}
		out["summary"] = summary
	}
	return nil, out, nil
}
