package server

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gabriellsalesx/Discordbot-mcp/internal/discord"
)

type roleView struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Position    int      `json:"position"`
	Color       string   `json:"color,omitempty"`
	Members     *int     `json:"members,omitempty"`
	Hoist       bool     `json:"hoist,omitempty"`
	Mentionable bool     `json:"mentionable,omitempty"`
	Managed     string   `json:"managed_by,omitempty"`
	Permissions []string `json:"permissions"`
	BotCanEdit  bool     `json:"bot_can_manage"`
}

func (sc *scope) roleView(r *discord.Role, counts map[string]int, botTop *discord.Role) roleView {
	v := roleView{
		ID: r.ID, Name: roleLabel(r, sc.g.ID), Position: r.Position, Color: colorHex(r.PrimaryColor()),
		Hoist: r.Hoist, Mentionable: r.Mentionable, Permissions: permList(r.Permissions),
		BotCanEdit: botCanManage(*r, botTop),
	}
	if n, ok := counts[r.ID]; ok {
		v.Members = new(n)
	}
	if r.Managed {
		v.Managed = "integration"
		if r.Tags != nil && r.Tags.BotID != "" {
			v.Managed = "bot " + r.Tags.BotID
		}
	}
	return v
}

func (s *Server) listRoles(ctx context.Context, _ *mcp.CallToolRequest, in serverIn) (*mcp.CallToolResult, any, error) {
	sc, err := s.scope(ctx, in.Server)
	if err != nil {
		return nil, nil, err
	}
	counts, err := sc.c.RoleMemberCounts(ctx, sc.g.ID)
	if err != nil {
		counts = nil // older or restricted servers: counts are optional
	}
	var botTop *discord.Role
	botRole := ""
	if m, err := sc.botMember(ctx); err == nil {
		if botTop = sc.topRole(m.Roles); botTop != nil {
			botRole = botTop.Name
		}
	}
	roles := slices.Clone(sc.g.Roles)
	slices.SortFunc(roles, func(a, b discord.Role) int {
		if discord.RoleAbove(a, b) {
			return -1
		}
		return 1
	})
	views := make([]roleView, 0, len(roles))
	for i := range roles {
		views = append(views, sc.roleView(&roles[i], counts, botTop))
	}
	return nil, map[string]any{
		"server":       sc.g.Name,
		"roles":        views,
		"bot_top_role": botRole,
		"note":         "Higher position is higher in the hierarchy. The bot can only manage roles below its top role and grant permissions it has.",
	}, nil
}

func (s *Server) createRole(ctx context.Context, _ *mcp.CallToolRequest, in createRoleIn) (*mcp.CallToolResult, any, error) {
	sc, err := s.scope(ctx, in.Server)
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, nil, fmt.Errorf("name is required")
	}
	perms, err := discord.ParsePerms(in.Permissions)
	if err != nil {
		return nil, nil, err
	}
	color, err := parseColor(in.Color)
	if err != nil {
		return nil, nil, err
	}
	body := map[string]any{
		"name":        in.Name,
		"permissions": perms,
		"colors":      map[string]any{"primary_color": color},
		"hoist":       in.Hoist,
		"mentionable": in.Mentionable,
	}
	r, err := sc.c.CreateRole(ctx, sc.g.ID, body, in.Reason)
	if err != nil {
		return nil, nil, err
	}
	out := map[string]any{"id": r.ID, "name": r.Name, "color": colorHex(r.PrimaryColor()), "permissions": permList(r.Permissions), "position": r.Position}
	if in.Position != nil {
		if err := sc.c.SetRolePosition(ctx, sc.g.ID, r.ID, max(*in.Position, 1), in.Reason); err != nil {
			out["position_error"] = err.Error()
		} else {
			out["position"] = max(*in.Position, 1)
		}
	}
	return nil, out, nil
}

func (s *Server) updateRole(ctx context.Context, _ *mcp.CallToolRequest, in updateRoleIn) (*mcp.CallToolResult, any, error) {
	sc, err := s.scope(ctx, in.Server)
	if err != nil {
		return nil, nil, err
	}
	r, err := sc.role(in.Role)
	if err != nil {
		return nil, nil, err
	}
	everyone := r.ID == sc.g.ID
	if everyone && in.Position != nil {
		return nil, nil, fmt.Errorf("@everyone always stays at the bottom")
	}
	body := map[string]any{}
	if in.Name != "" {
		if everyone {
			return nil, nil, fmt.Errorf("@everyone cannot be renamed")
		}
		body["name"] = in.Name
	}
	if in.Color != "" {
		c, err := parseColor(in.Color)
		if err != nil {
			return nil, nil, err
		}
		// Keep gradient colors, which only the primary color replaces here.
		colors := map[string]any{"primary_color": c}
		if r.Colors != nil && c != 0 {
			colors["secondary_color"], colors["tertiary_color"] = r.Colors.SecondaryColor, r.Colors.TertiaryColor
		}
		body["colors"] = colors
	}
	perms := r.Permissions
	if in.Permissions != nil {
		if perms, err = discord.ParsePerms(in.Permissions); err != nil {
			return nil, nil, err
		}
	}
	add, err := discord.ParsePerms(in.AddPermissions)
	if err != nil {
		return nil, nil, err
	}
	remove, err := discord.ParsePerms(in.RemovePermissions)
	if err != nil {
		return nil, nil, err
	}
	perms = perms&^remove | add
	if perms != r.Permissions {
		body["permissions"] = perms
	}
	if in.Hoist != nil {
		body["hoist"] = *in.Hoist
	}
	if in.Mentionable != nil {
		body["mentionable"] = *in.Mentionable
	}
	if len(body) == 0 && in.Position == nil {
		return nil, nil, fmt.Errorf("nothing to change; pass at least one field")
	}
	updated := r
	if len(body) > 0 {
		if updated, err = sc.c.ModifyRole(ctx, sc.g.ID, r.ID, body, in.Reason); err != nil {
			return nil, nil, err
		}
	}
	out := map[string]any{"id": updated.ID, "name": roleLabel(updated, sc.g.ID), "color": colorHex(updated.PrimaryColor()), "permissions": permList(updated.Permissions), "position": updated.Position}
	if perms != r.Permissions {
		out["added"] = (perms &^ r.Permissions).Names()
		out["removed"] = (r.Permissions &^ perms).Names()
	}
	if in.Position != nil {
		if err := sc.c.SetRolePosition(ctx, sc.g.ID, r.ID, max(*in.Position, 1), in.Reason); err != nil {
			return nil, nil, err
		}
		out["position"] = max(*in.Position, 1)
	}
	return nil, out, nil
}

func (s *Server) deleteRole(ctx context.Context, _ *mcp.CallToolRequest, in deleteRoleIn) (*mcp.CallToolResult, any, error) {
	sc, err := s.scope(ctx, in.Server)
	if err != nil {
		return nil, nil, err
	}
	r, err := sc.role(in.Role)
	if err != nil {
		return nil, nil, err
	}
	if r.ID == sc.g.ID {
		return nil, nil, fmt.Errorf("@everyone cannot be deleted")
	}
	if r.Managed {
		return nil, nil, fmt.Errorf("%s belongs to a bot or integration; remove that integration instead", r.Name)
	}
	details := map[string]any{"server": sc.g.Name, "id": r.ID, "name": r.Name, "permissions": permList(r.Permissions)}
	if counts, err := sc.c.RoleMemberCounts(ctx, sc.g.ID); err == nil {
		details["members_losing_it"] = counts[r.ID]
	}
	if !in.Confirm {
		return nil, confirmPreview("delete role "+r.Name, details), nil
	}
	if err := sc.c.DeleteRole(ctx, sc.g.ID, r.ID, in.Reason); err != nil {
		return nil, nil, err
	}
	return nil, map[string]any{"deleted": true, "id": r.ID, "name": r.Name}, nil
}
