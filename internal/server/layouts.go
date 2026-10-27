package server

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gabriellsalesx/discord-mcp/internal/discord"
	"github.com/gabriellsalesx/discord-mcp/internal/layout"
)

type saveLayoutIn struct {
	Server      string `json:"server,omitempty" jsonschema:"Server to copy the structure from (id or name). Optional when the bot is in a single server or a default is configured."`
	Name        string `json:"name" jsonschema:"Layout name, e.g. Comunidade padrão. Saving with an existing name replaces it."`
	Description string `json:"description,omitempty" jsonschema:"What the layout is for."`
}

type layoutNameIn struct {
	Name string `json:"name" jsonschema:"Layout name from list_layouts."`
}

type applyLayoutIn struct {
	Server   string `json:"server,omitempty" jsonschema:"Server to build the layout in (id or name). Optional when the bot is in a single server or a default is configured."`
	Name     string `json:"name" jsonschema:"Layout name from list_layouts."`
	Everyone bool   `json:"apply_everyone_permissions,omitempty" jsonschema:"Also replace the server-wide permissions of @everyone with the layout's. Default false."`
	Confirm  bool   `json:"confirm,omitempty" jsonschema:"false returns a preview; true does it. Only set true after the user agreed to the preview."`
}

func (s *Server) registerLayouts(ms *mcp.Server) {
	addTool(ms, &mcp.Tool{
		Name:        "save_layout",
		Description: "Save a server's structure as a reusable layout: roles (names, colors, permissions, order), @everyone's permissions, categories and channels in order, topics, slowmode and per-role channel permissions. Messages and members are not saved.",
		Annotations: ann("Save layout", false, true, true),
	}, s.saveLayout)

	addTool(ms, &mcp.Tool{
		Name:        "list_layouts",
		Description: "List the saved server layouts with what each contains.",
		Annotations: ann("List layouts", true, false, true),
	}, s.listLayouts)

	addTool(ms, &mcp.Tool{
		Name:        "get_layout",
		Description: "Show a saved layout in full: roles, categories, channels and their permissions.",
		Annotations: ann("Show layout", true, false, true),
	}, s.getLayout)

	addTool(ms, &mcp.Tool{
		Name:        "apply_layout",
		Description: "Recreate a saved layout in a server: creates the missing roles, categories and channels with their permissions. Roles and channels that already exist with the same name are reused, never changed or deleted. Call without confirm first to see the plan, then with confirm=true once the user agrees.",
		Annotations: ann("Apply layout", false, false, false),
	}, s.applyLayout)

	addTool(ms, &mcp.Tool{
		Name:        "delete_layout",
		Description: "Delete a saved layout. Servers built from it are not touched.",
		Annotations: ann("Delete layout", false, true, true),
	}, s.deleteLayout)
}

// roleOverwrites keeps the role overrides, the part of a channel's
// permissions that carries over to another server.
func roleOverwrites(list []discord.Overwrite) []discord.Overwrite {
	return slices.DeleteFunc(slices.Clone(list), func(o discord.Overwrite) bool { return o.Type != discord.OverwriteRole })
}

func (sc *scope) layoutOverwrites(list []discord.Overwrite) []layout.Overwrite {
	var out []layout.Overwrite
	for _, o := range roleOverwrites(list) {
		r, err := sc.role(o.ID)
		if err != nil || (r.Managed && r.ID != sc.g.ID) {
			continue // deleted role, or a bot's own role
		}
		out = append(out, layout.Overwrite{Role: roleLabel(r, sc.g.ID), Allow: o.Allow.Names(), Deny: o.Deny.Names()})
	}
	return out
}

func (sc *scope) snapshot(ctx context.Context) (*layout.Layout, error) {
	chs, err := sc.loadChannels(ctx)
	if err != nil {
		return nil, err
	}
	l := &layout.Layout{Source: sc.g.Name, SavedAt: time.Now().UTC(), Everyone: sc.everyone().Permissions.Names(),
		Roles: []layout.Role{}, Uncategorized: []layout.Channel{}, Categories: []layout.Category{}}
	roles := slices.Clone(sc.g.Roles)
	slices.SortFunc(roles, func(a, b discord.Role) int {
		if discord.RoleAbove(a, b) {
			return -1
		}
		return 1
	})
	for _, r := range roles {
		if r.ID == sc.g.ID || r.Managed {
			continue
		}
		l.Roles = append(l.Roles, layout.Role{Name: r.Name, Color: colorHex(r.PrimaryColor()), Permissions: r.Permissions.Names(), Hoist: r.Hoist, Mentionable: r.Mentionable})
	}
	sorted := slices.Clone(chs)
	sortChannels(sorted)
	index := map[string]int{}
	for _, ch := range sorted {
		if ch.Type == discord.ChannelCategory {
			index[ch.ID] = len(l.Categories)
			l.Categories = append(l.Categories, layout.Category{Name: ch.Name, Overwrites: sc.layoutOverwrites(ch.Overwrites), Channels: []layout.Channel{}})
		}
	}
	for _, ch := range sorted {
		if ch.Type == discord.ChannelCategory {
			continue
		}
		c := layout.Channel{Name: ch.Name, Type: typeName(ch.Type), Topic: ch.Topic, NSFW: ch.NSFW, Slowmode: ch.RateLimitPerUser}
		if isVoice(ch.Type) {
			c.UserLimit = ch.UserLimit
		}
		parent := sc.channelByID(ch.ParentID)
		if parent != nil && sameOverwrites(roleOverwrites(ch.Overwrites), roleOverwrites(parent.Overwrites)) {
			c.Synced = true
		} else {
			c.Overwrites = sc.layoutOverwrites(ch.Overwrites)
		}
		if k, ok := index[ch.ParentID]; ok {
			l.Categories[k].Channels = append(l.Categories[k].Channels, c)
		} else {
			l.Uncategorized = append(l.Uncategorized, c)
		}
	}
	return l, nil
}

func layoutSummary(l *layout.Layout) map[string]any {
	roles, cats, chans := l.Counts()
	return map[string]any{"name": l.Name, "description": l.Description, "source_server": l.Source,
		"saved_at": l.SavedAt.Format(time.DateOnly), "roles": roles, "categories": cats, "channels": chans}
}

func (s *Server) saveLayout(ctx context.Context, _ *mcp.CallToolRequest, in saveLayoutIn) (*mcp.CallToolResult, any, error) {
	if layout.Slug(in.Name) == "" {
		return nil, nil, fmt.Errorf("name is required")
	}
	sc, err := s.scope(ctx, in.Server)
	if err != nil {
		return nil, nil, err
	}
	l, err := sc.snapshot(ctx)
	if err != nil {
		return nil, nil, err
	}
	l.Name, l.Description = strings.TrimSpace(in.Name), in.Description
	_, loadErr := layout.Load(l.Name)
	path, err := layout.Save(l)
	if err != nil {
		return nil, nil, err
	}
	out := layoutSummary(l)
	out["file"] = path
	out["replaced"] = loadErr == nil
	out["next"] = "Use apply_layout with this name to build the same structure in another server."
	return nil, out, nil
}

func (s *Server) listLayouts(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	ls, err := layout.List()
	if err != nil {
		return nil, nil, err
	}
	out := []map[string]any{}
	for _, l := range ls {
		out = append(out, layoutSummary(l))
	}
	dir, _ := layout.Dir()
	return nil, map[string]any{"layouts": out, "folder": dir}, nil
}

func (s *Server) getLayout(_ context.Context, _ *mcp.CallToolRequest, in layoutNameIn) (*mcp.CallToolResult, any, error) {
	l, err := layout.Load(in.Name)
	if err != nil {
		return nil, nil, withLayoutHint(err)
	}
	return nil, l, nil
}

func (s *Server) deleteLayout(_ context.Context, _ *mcp.CallToolRequest, in layoutNameIn) (*mcp.CallToolResult, any, error) {
	if err := layout.Delete(in.Name); err != nil {
		return nil, nil, withLayoutHint(err)
	}
	return nil, map[string]any{"deleted": true, "name": in.Name}, nil
}

func withLayoutHint(err error) error {
	if errors.Is(err, layout.ErrNotFound) {
		return fmt.Errorf("%w; call list_layouts to see the saved ones", err)
	}
	return err
}

// builder applies a layout, or with dry set only records what it would do.
type builder struct {
	sc    *scope
	ctx   context.Context
	dry   bool
	l     *layout.Layout
	bot   *discord.Member
	admin bool
	have  discord.Perm // what the bot may grant or set in overrides
	roles map[string]string

	CreatedRoles    []string `json:"roles_created"`
	ReusedRoles     []string `json:"roles_already_there,omitempty"`
	CreatedChannels []string `json:"channels_created"`
	Existing        []string `json:"channels_already_there,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
}

func (b *builder) warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if !slices.Contains(b.Warnings, msg) {
		b.Warnings = append(b.Warnings, msg)
	}
}

// grantable drops what the bot cannot set and warns about it.
func (b *builder) grantable(p discord.Perm, where string) discord.Perm {
	limit := b.have
	if !b.admin {
		// Only administrators may set Manage Permissions in a channel.
		if where != "role" && where != "@everyone" {
			limit &^= discord.ManageRoles
		}
	}
	if lost := p &^ limit; lost != 0 && !b.admin {
		b.warn("%s: skipped %v, which the bot does not have itself", where, lost.Names())
	}
	if b.admin {
		return p
	}
	return p & limit
}

func (b *builder) overwrites(list []layout.Overwrite, where string, voice bool) ([]discord.Overwrite, error) {
	var out []discord.Overwrite
	var locked discord.Perm // what @everyone is denied that the bot relies on
	for _, o := range list {
		id, ok := b.roles[strings.ToLower(strings.TrimPrefix(o.Role, "@"))]
		if strings.EqualFold(o.Role, "@everyone") {
			id, ok = b.sc.g.ID, true
		}
		if !ok {
			b.warn("%s: role %s is not in the layout or the server, its override was skipped", where, o.Role)
			continue
		}
		allow, err := discord.ParsePerms(o.Allow)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", where, err)
		}
		deny, err := discord.ParsePerms(o.Deny)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", where, err)
		}
		if id == b.sc.g.ID {
			locked = deny & (discord.ViewChannel | discord.SendMessages | discord.ReadMessageHistory | discord.Connect)
		}
		out = append(out, discord.Overwrite{ID: id, Type: discord.OverwriteRole, Allow: b.grantable(allow, where), Deny: b.grantable(deny, where)})
	}
	// Whatever @everyone is locked out of, the bot that manages the channel
	// keeps (its role allow does not beat a channel deny).
	if locked != 0 && !b.admin {
		keep := locked
		if locked.Has(discord.ViewChannel) {
			keep |= discord.SendMessages | discord.ReadMessageHistory | discord.ManageChannels
			if voice {
				keep |= discord.Connect
			}
		}
		out = append(out, discord.Overwrite{ID: b.bot.User.ID, Type: discord.OverwriteMember, Allow: keep & b.have})
	}
	return out, nil
}

func (b *builder) run(everyone bool) error {
	sc := b.sc
	// Roles, top first: Discord puts each new role at the bottom, so
	// creating in this order keeps the layout's hierarchy.
	for _, r := range b.l.Roles {
		if existing, err := sc.role(r.Name); err == nil && existing.ID != sc.g.ID {
			b.roles[strings.ToLower(r.Name)] = existing.ID
			b.ReusedRoles = append(b.ReusedRoles, r.Name)
			continue
		}
		perms, err := discord.ParsePerms(r.Permissions)
		if err != nil {
			return fmt.Errorf("role %s: %w", r.Name, err)
		}
		color, err := parseColor(r.Color)
		if err != nil {
			return fmt.Errorf("role %s: %w", r.Name, err)
		}
		perms = b.grantable(perms, "role")
		b.CreatedRoles = append(b.CreatedRoles, r.Name)
		if b.dry {
			b.roles[strings.ToLower(r.Name)] = "new"
			continue
		}
		created, err := sc.c.CreateRole(b.ctx, sc.g.ID, map[string]any{
			"name": r.Name, "permissions": perms, "colors": map[string]any{"primary_color": color},
			"hoist": r.Hoist, "mentionable": r.Mentionable,
		}, "layout "+b.l.Name)
		if err != nil {
			return fmt.Errorf("creating role %s: %w", r.Name, err)
		}
		b.roles[strings.ToLower(r.Name)] = created.ID
	}

	if everyone {
		perms, err := discord.ParsePerms(b.l.Everyone)
		if err != nil {
			return fmt.Errorf("@everyone: %w", err)
		}
		perms = b.grantable(perms, "@everyone")
		if !b.dry {
			if _, err := sc.c.ModifyRole(b.ctx, sc.g.ID, sc.g.ID, map[string]any{"permissions": perms}, "layout "+b.l.Name); err != nil {
				return fmt.Errorf("updating @everyone: %w", err)
			}
		}
	}

	for _, c := range b.l.Uncategorized {
		if err := b.channel(c, nil); err != nil {
			return err
		}
	}
	for _, cat := range b.l.Categories {
		var parent *discord.Channel
		for i := range sc.channels {
			if ch := &sc.channels[i]; ch.Type == discord.ChannelCategory && sameName(ch.Name, cat.Name) {
				parent = ch
				break
			}
		}
		if parent != nil {
			b.Existing = append(b.Existing, cat.Name+" (category)")
		} else {
			ow, err := b.overwrites(cat.Overwrites, cat.Name, false)
			if err != nil {
				return err
			}
			b.CreatedChannels = append(b.CreatedChannels, cat.Name+" (category)")
			if b.dry {
				parent = &discord.Channel{Name: cat.Name}
			} else {
				body := map[string]any{"name": cat.Name, "type": discord.ChannelCategory, "permission_overwrites": nonNil(ow)}
				if parent, err = sc.c.CreateChannel(b.ctx, sc.g.ID, body, "layout "+b.l.Name); err != nil {
					return fmt.Errorf("creating category %s: %w", cat.Name, err)
				}
			}
		}
		for _, c := range cat.Channels {
			if err := b.channel(c, parent); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *builder) channel(c layout.Channel, parent *discord.Channel) error {
	sc := b.sc
	typ, ok := channelTypes[c.Type]
	if !ok || typ == discord.ChannelCategory {
		return fmt.Errorf("channel %s has an invalid type %q", c.Name, c.Type)
	}
	label, parentID := c.Name, ""
	if parent != nil {
		label, parentID = parent.Name+"/"+c.Name, parent.ID
	}
	// A category made in this run (or only planned) has nothing in it yet.
	if parent == nil || parentID != "" {
		for _, ch := range sc.channels {
			if ch.Type == typ && sameName(ch.Name, c.Name) && ch.ParentID == parentID {
				b.Existing = append(b.Existing, label)
				return nil
			}
		}
	}
	body := map[string]any{"name": c.Name, "type": typ}
	if parentID != "" {
		body["parent_id"] = parentID
	}
	if c.Topic != "" {
		body["topic"] = c.Topic
	}
	if c.NSFW {
		body["nsfw"] = true
	}
	if c.Slowmode > 0 {
		body["rate_limit_per_user"] = c.Slowmode
	}
	if c.UserLimit > 0 && isVoice(typ) {
		body["user_limit"] = c.UserLimit
	}
	// Synced channels inherit the category's permissions when created
	// without overrides.
	if !c.Synced || parent == nil {
		ow, err := b.overwrites(c.Overwrites, label, isVoice(typ))
		if err != nil {
			return err
		}
		if len(ow) > 0 || parent != nil {
			body["permission_overwrites"] = nonNil(ow)
		}
	}
	b.CreatedChannels = append(b.CreatedChannels, label)
	if b.dry {
		return nil
	}
	if _, err := sc.c.CreateChannel(b.ctx, sc.g.ID, body, "layout "+b.l.Name); err != nil {
		return fmt.Errorf("creating %s: %w", label, err)
	}
	return nil
}

func (s *Server) applyLayout(ctx context.Context, _ *mcp.CallToolRequest, in applyLayoutIn) (*mcp.CallToolResult, any, error) {
	l, err := layout.Load(in.Name)
	if err != nil {
		return nil, nil, withLayoutHint(err)
	}
	sc, err := s.scope(ctx, in.Server)
	if err != nil {
		return nil, nil, err
	}
	if _, err := sc.loadChannels(ctx); err != nil {
		return nil, nil, err
	}
	bot, err := sc.botMember(ctx)
	if err != nil {
		return nil, nil, err
	}
	b := &builder{sc: sc, ctx: ctx, dry: !in.Confirm, l: l, bot: bot, admin: s.botIsAdmin(sc, bot), roles: map[string]string{},
		CreatedRoles: []string{}, CreatedChannels: []string{}}
	b.have = discord.Effective(sc.g.ID, sc.everyone().Permissions, sc.rolesOf(bot.Roles), bot.User.ID, sc.g.OwnerID, nil)
	err = b.run(in.Everyone)
	if b.dry {
		if err != nil {
			return nil, nil, err
		}
		details := map[string]any{"server": sc.g.Name, "layout": l.Name, "plan": b, "everyone_permissions_replaced": in.Everyone}
		return nil, confirmPreview("build layout "+l.Name+" in "+sc.g.Name, details), nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("%w (done before the error: roles %v, channels %v)", err, b.CreatedRoles, b.CreatedChannels)
	}
	return nil, map[string]any{"server": sc.g.Name, "layout": l.Name, "result": b,
		"next": "Call list_channels and list_roles to review. Existing roles and channels were left as they were."}, nil
}
