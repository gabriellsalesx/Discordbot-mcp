package server

import (
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gabriellsalesx/Discordbot-mcp/internal/discord"
)

type serverIn struct {
	Server string `json:"server,omitempty" jsonschema:"Server id or name. Optional when the bot is in a single server or a default is configured."`
}

type updateServerIn struct {
	Server               string  `json:"server,omitempty" jsonschema:"Server id or name. Optional when the bot is in a single server or a default is configured."`
	Name                 string  `json:"name,omitempty" jsonschema:"New server name."`
	Description          *string `json:"description,omitempty" jsonschema:"New description (Community servers). Empty string clears it."`
	VerificationLevel    string  `json:"verification_level,omitempty" jsonschema:"none, low, medium, high or very_high."`
	DefaultNotifications string  `json:"default_notifications,omitempty" jsonschema:"all_messages or only_mentions."`
	ContentFilter        string  `json:"explicit_content_filter,omitempty" jsonschema:"disabled, members_without_roles or all_members."`
	SystemChannel        string  `json:"system_channel,omitempty" jsonschema:"Channel for welcome and boost messages, or none."`
	RulesChannel         string  `json:"rules_channel,omitempty" jsonschema:"Rules channel (Community servers), or none."`
	AFKChannel           string  `json:"afk_channel,omitempty" jsonschema:"Voice channel for idle members, or none."`
	AFKTimeout           int     `json:"afk_timeout,omitempty" jsonschema:"Idle seconds before moving to the AFK channel: 60, 300, 900, 1800 or 3600."`
	PreferredLocale      string  `json:"preferred_locale,omitempty" jsonschema:"Locale such as pt-BR or en-US (Community servers)."`
	Reason               string  `json:"reason,omitempty" jsonschema:"Optional note recorded in the server's audit log."`
}

type channelIn struct {
	Server  string `json:"server,omitempty" jsonschema:"Server id or name. Optional when the bot is in a single server or a default is configured."`
	Channel string `json:"channel" jsonschema:"Channel id, #name, name or category/name."`
}

type createChannelIn struct {
	Server    string   `json:"server,omitempty" jsonschema:"Server id or name. Optional when the bot is in a single server or a default is configured."`
	Name      string   `json:"name" jsonschema:"Channel name. Discord lowercases text channel names and turns spaces into hyphens."`
	Type      string   `json:"type,omitempty" jsonschema:"text (default), voice, category, announcement, stage, forum or media."`
	Category  string   `json:"category,omitempty" jsonschema:"Category to create the channel in (id or name). Its permissions are copied, so the channel starts synced."`
	Topic     string   `json:"topic,omitempty" jsonschema:"Channel topic, up to 1024 characters."`
	NSFW      bool     `json:"nsfw,omitempty" jsonschema:"Mark as age-restricted."`
	Slowmode  int      `json:"slowmode_seconds,omitempty" jsonschema:"Seconds each member waits between messages, 0-21600."`
	Bitrate   int      `json:"bitrate,omitempty" jsonschema:"Voice bitrate in bits per second, 8000-96000 without boosts."`
	UserLimit int      `json:"user_limit,omitempty" jsonschema:"Voice user limit, 0-99 (0 means no limit)."`
	Position  *int     `json:"position,omitempty" jsonschema:"Sort position inside its category, 0 is the top."`
	Private   bool     `json:"private,omitempty" jsonschema:"Hide from @everyone. Only the roles or members in visible_to (and admins) will see it."`
	VisibleTo []string `json:"visible_to,omitempty" jsonschema:"Roles or members that can see a private channel."`
	Reason    string   `json:"reason,omitempty" jsonschema:"Optional note recorded in the server's audit log."`
}

type updateChannelIn struct {
	Server    string  `json:"server,omitempty" jsonschema:"Server id or name. Optional when the bot is in a single server or a default is configured."`
	Channel   string  `json:"channel" jsonschema:"Channel id, #name, name or category/name."`
	Name      string  `json:"name,omitempty" jsonschema:"New name."`
	Topic     *string `json:"topic,omitempty" jsonschema:"New topic. Empty string clears it."`
	Category  string  `json:"category,omitempty" jsonschema:"Move into this category (id or name), or none to take it out of its category."`
	Sync      bool    `json:"sync_permissions,omitempty" jsonschema:"Replace the channel's permissions with its category's (after any move)."`
	Position  *int    `json:"position,omitempty" jsonschema:"New sort position. To reorder many channels use organize_channels."`
	NSFW      *bool   `json:"nsfw,omitempty" jsonschema:"Age-restricted or not."`
	Slowmode  *int    `json:"slowmode_seconds,omitempty" jsonschema:"Seconds between messages per member, 0 turns slowmode off."`
	Bitrate   int     `json:"bitrate,omitempty" jsonschema:"Voice bitrate in bits per second."`
	UserLimit *int    `json:"user_limit,omitempty" jsonschema:"Voice user limit, 0 means no limit."`
	Reason    string  `json:"reason,omitempty" jsonschema:"Optional note recorded in the server's audit log."`
}

type deleteChannelIn struct {
	Server  string `json:"server,omitempty" jsonschema:"Server id or name. Optional when the bot is in a single server or a default is configured."`
	Channel string `json:"channel" jsonschema:"Channel or category id, #name, name or category/name."`
	Confirm bool   `json:"confirm,omitempty" jsonschema:"false returns a preview; true does it. Only set true after the user agreed to the preview."`
	Reason  string `json:"reason,omitempty" jsonschema:"Optional note recorded in the server's audit log."`
}

type organizeIn struct {
	Server   string   `json:"server,omitempty" jsonschema:"Server id or name. Optional when the bot is in a single server or a default is configured."`
	Channels []string `json:"channels" jsonschema:"Channels in the order they should appear, top first. To order categories, list only categories."`
	Category string   `json:"category,omitempty" jsonschema:"Move the listed channels into this category first (id or name), or none for no category. Omit to keep each channel where it is."`
	Sync     bool     `json:"sync_permissions,omitempty" jsonschema:"When moving, also take the new category's permissions."`
	Reason   string   `json:"reason,omitempty" jsonschema:"Optional note recorded in the server's audit log."`
}

type createRoleIn struct {
	Server      string   `json:"server,omitempty" jsonschema:"Server id or name. Optional when the bot is in a single server or a default is configured."`
	Name        string   `json:"name" jsonschema:"Role name."`
	Color       string   `json:"color,omitempty" jsonschema:"Hex color such as #5865F2. Default: no color."`
	Permissions []string `json:"permissions,omitempty" jsonschema:"Server-wide permissions, e.g. manage_messages, kick_members, moderate_members. Default: none (members still get @everyone's)."`
	Hoist       bool     `json:"hoist,omitempty" jsonschema:"Show members with this role separately in the member list."`
	Mentionable bool     `json:"mentionable,omitempty" jsonschema:"Let anyone @mention this role."`
	Position    *int     `json:"position,omitempty" jsonschema:"Place in the hierarchy; 1 is just above @everyone. Default: bottom."`
	Reason      string   `json:"reason,omitempty" jsonschema:"Optional note recorded in the server's audit log."`
}

type updateRoleIn struct {
	Server            string   `json:"server,omitempty" jsonschema:"Server id or name. Optional when the bot is in a single server or a default is configured."`
	Role              string   `json:"role" jsonschema:"Role id, @name or name. everyone is the @everyone role."`
	Name              string   `json:"name,omitempty" jsonschema:"New name."`
	Color             string   `json:"color,omitempty" jsonschema:"Hex color such as #5865F2, or none."`
	Permissions       []string `json:"permissions,omitempty" jsonschema:"Replace all server-wide permissions with these."`
	AddPermissions    []string `json:"add_permissions,omitempty" jsonschema:"Permissions to add, keeping the rest."`
	RemovePermissions []string `json:"remove_permissions,omitempty" jsonschema:"Permissions to remove, keeping the rest."`
	Hoist             *bool    `json:"hoist,omitempty" jsonschema:"Show members separately in the member list."`
	Mentionable       *bool    `json:"mentionable,omitempty" jsonschema:"Let anyone @mention this role."`
	Position          *int     `json:"position,omitempty" jsonschema:"New place in the hierarchy; 1 is just above @everyone."`
	Reason            string   `json:"reason,omitempty" jsonschema:"Optional note recorded in the server's audit log."`
}

type deleteRoleIn struct {
	Server  string `json:"server,omitempty" jsonschema:"Server id or name. Optional when the bot is in a single server or a default is configured."`
	Role    string `json:"role" jsonschema:"Role id, @name or name."`
	Confirm bool   `json:"confirm,omitempty" jsonschema:"false returns a preview; true does it. Only set true after the user agreed to the preview."`
	Reason  string `json:"reason,omitempty" jsonschema:"Optional note recorded in the server's audit log."`
}

type setPermsIn struct {
	Server     string   `json:"server,omitempty" jsonschema:"Server id or name. Optional when the bot is in a single server or a default is configured."`
	Channel    string   `json:"channel" jsonschema:"Channel or category id, #name, name or category/name."`
	Target     string   `json:"target" jsonschema:"Role (name, @name or id; everyone for @everyone) or member (name or id) the override is for."`
	TargetType string   `json:"target_type,omitempty" jsonschema:"role or member, when the name could be either. Default: roles are tried first."`
	Allow      []string `json:"allow,omitempty" jsonschema:"Permissions to allow here, e.g. view_channel, send_messages, connect."`
	Deny       []string `json:"deny,omitempty" jsonschema:"Permissions to deny here."`
	Reset      []string `json:"reset,omitempty" jsonschema:"Permissions to set back to inherit (neither allowed nor denied)."`
	Replace    bool     `json:"replace,omitempty" jsonschema:"Start from an empty override instead of changing the current one."`
	Remove     bool     `json:"remove,omitempty" jsonschema:"Delete this target's override entirely; allow, deny and reset are ignored."`
	Reason     string   `json:"reason,omitempty" jsonschema:"Optional note recorded in the server's audit log."`
}

type checkPermsIn struct {
	Server     string `json:"server,omitempty" jsonschema:"Server id or name. Optional when the bot is in a single server or a default is configured."`
	Target     string `json:"target" jsonschema:"Role (name or id; everyone for @everyone) or member (name or id)."`
	TargetType string `json:"target_type,omitempty" jsonschema:"role or member, when the name could be either."`
	Channel    string `json:"channel,omitempty" jsonschema:"Channel to check. Omit for server-wide permissions."`
}

type inviteIn struct {
	ServerID string `json:"server_id,omitempty" jsonschema:"Id of the server to preselect (Discord: right-click the server icon > Copy Server ID). Optional: without it the user picks the server on the page."`
	Admin    bool   `json:"admin,omitempty" jsonschema:"Ask for Administrator instead of the individual permissions."`
}

type findMembersIn struct {
	Server string `json:"server,omitempty" jsonschema:"Server id or name. Optional when the bot is in a single server or a default is configured."`
	Query  string `json:"query" jsonschema:"Start of a username or nickname."`
	Limit  int    `json:"limit,omitempty" jsonschema:"How many members to return, 1-50. Default 10."`
}

type memberRolesIn struct {
	Server string   `json:"server,omitempty" jsonschema:"Server id or name. Optional when the bot is in a single server or a default is configured."`
	Member string   `json:"member" jsonschema:"Member user id, mention or name."`
	Add    []string `json:"add,omitempty" jsonschema:"Roles to give."`
	Remove []string `json:"remove,omitempty" jsonschema:"Roles to take away."`
	Reason string   `json:"reason,omitempty" jsonschema:"Optional note recorded in the server's audit log."`
}

type sendMessageIn struct {
	Server        string `json:"server,omitempty" jsonschema:"Server id or name. Optional when the bot is in a single server or a default is configured."`
	Channel       string `json:"channel" jsonschema:"Channel id, #name, name or category/name."`
	Content       string `json:"content" jsonschema:"Message text, Discord Markdown, up to 2000 characters."`
	AllowMentions bool   `json:"allow_mentions,omitempty" jsonschema:"Let @everyone, @here, role and user mentions notify people. Default false: they render but do not ping."`
	Confirm       bool   `json:"confirm,omitempty" jsonschema:"false returns a preview; true does it. Only set true after the user agreed to the preview."`
}

type preview struct {
	ConfirmationRequired bool   `json:"confirmation_required"`
	Action               string `json:"action"`
	Details              any    `json:"details,omitempty"`
	Next                 string `json:"next"`
}

func confirmPreview(action string, details any) preview {
	return preview{
		ConfirmationRequired: true,
		Action:               action,
		Details:              details,
		Next:                 "Nothing was changed. Show this to the user and call the tool again with confirm=true only if they approve.",
	}
}

func ann(title string, readOnly, destructive, idempotent bool) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		Title:           title,
		ReadOnlyHint:    readOnly,
		DestructiveHint: &destructive,
		IdempotentHint:  idempotent,
		OpenWorldHint:   new(true),
	}
}

// addTool registers a tool with a portable input schema (see schemaFor).
func addTool[In any](ms *mcp.Server, t *mcp.Tool, h mcp.ToolHandlerFor[In, any]) {
	t.InputSchema = schemaFor[In]()
	mcp.AddTool(ms, t, h)
}

func (s *Server) register(ms *mcp.Server) {
	addTool(ms, &mcp.Tool{
		Name:        "list_servers",
		Description: "List the Discord servers the bot is in, with member counts and what the bot may manage in each.",
		Annotations: ann("List servers", true, false, true),
	}, s.listServers)

	addTool(ms, &mcp.Tool{
		Name:        "get_server",
		Description: "Show a server's settings (verification, notifications, content filter, system/rules/AFK channels), counts, and the bot's own role and permissions there.",
		Annotations: ann("Server info", true, false, true),
	}, s.getServer)

	addTool(ms, &mcp.Tool{
		Name:        "update_server",
		Description: "Change server settings: name, description, verification level, default notifications, content filter, system/rules/AFK channels, AFK timeout, locale. Only the fields you pass change.",
		Annotations: ann("Update server", false, true, true),
	}, s.updateServer)

	addTool(ms, &mcp.Tool{
		Name:        "list_channels",
		Description: "Show the server's channels as the sidebar does: categories in order with their channels, each with id, type, topic, whether it is private and whether its permissions are synced with the category.",
		Annotations: ann("List channels", true, false, true),
	}, s.listChannels)

	addTool(ms, &mcp.Tool{
		Name:        "get_channel",
		Description: "Show one channel or category with its settings and every permission override, by role or member, as allowed and denied permission names.",
		Annotations: ann("Channel info", true, false, true),
	}, s.getChannel)

	addTool(ms, &mcp.Tool{
		Name:        "create_channel",
		Description: "Create a text, voice, announcement, stage, forum or media channel, or a category. Can place it in a category (inheriting its permissions) and make it private to chosen roles or members.",
		Annotations: ann("Create channel", false, false, false),
	}, s.createChannel)

	addTool(ms, &mcp.Tool{
		Name:        "update_channel",
		Description: "Rename a channel, change its topic, slowmode, NSFW flag, voice settings or position, move it to another category, or sync its permissions with its category. Only the fields you pass change.",
		Annotations: ann("Update channel", false, true, true),
	}, s.updateChannel)

	addTool(ms, &mcp.Tool{
		Name:        "delete_channel",
		Description: "Delete a channel or category and its messages, permanently. Channels inside a deleted category are kept, without a category. Call without confirm first, then with confirm=true once the user agrees.",
		Annotations: ann("Delete channel", false, true, true),
	}, s.deleteChannel)

	addTool(ms, &mcp.Tool{
		Name:        "organize_channels",
		Description: "Reorder channels, or categories, in one go, optionally moving the channels into a category first. Channels you do not list keep their relative order below the listed ones.",
		Annotations: ann("Organize channels", false, false, true),
	}, s.organizeChannels)

	addTool(ms, &mcp.Tool{
		Name:        "list_roles",
		Description: "List the server's roles from the top of the hierarchy: color, member count, server-wide permissions, and whether the bot can manage each one.",
		Annotations: ann("List roles", true, false, true),
	}, s.listRoles)

	addTool(ms, &mcp.Tool{
		Name:        "create_role",
		Description: "Create a role with a name, color, server-wide permissions, and display options.",
		Annotations: ann("Create role", false, false, false),
	}, s.createRole)

	addTool(ms, &mcp.Tool{
		Name:        "update_role",
		Description: "Change a role: name, color, permissions (replace, add or remove), display options, or its place in the hierarchy. Works on @everyone too, to change what every member can do.",
		Annotations: ann("Update role", false, true, true),
	}, s.updateRole)

	addTool(ms, &mcp.Tool{
		Name:        "delete_role",
		Description: "Delete a role, taking it from every member who has it. Call without confirm first, then with confirm=true once the user agrees.",
		Annotations: ann("Delete role", false, true, true),
	}, s.deleteRole)

	addTool(ms, &mcp.Tool{
		Name:        "set_channel_permissions",
		Description: "Allow, deny or reset permissions for a role or member in one channel or category (a permission override). Changes merge into the current override unless replace is set. On a category, channels synced with it are updated too. Use target everyone with deny view_channel to make a channel private.",
		Annotations: ann("Set channel permissions", false, true, true),
	}, s.setChannelPermissions)

	addTool(ms, &mcp.Tool{
		Name:        "check_permissions",
		Description: "Work out what a role or member can actually do, server-wide or in one channel, after roles and channel overrides are applied.",
		Annotations: ann("Check permissions", true, false, true),
	}, s.checkPermissions)

	addTool(ms, &mcp.Tool{
		Name:        "invite_bot",
		Description: "Get the link that adds the bot to another server. The user opens it, picks the server and clicks Authorize; they need Manage Server there. The new server is usable right after, without restarting.",
		Annotations: ann("Invite bot", true, false, true),
	}, s.inviteBot)

	addTool(ms, &mcp.Tool{
		Name:        "find_members",
		Description: "Find members by the start of their username or nickname, with their roles. Needs the Server Members intent enabled for the bot.",
		Annotations: ann("Find members", true, false, true),
	}, s.findMembers)

	addTool(ms, &mcp.Tool{
		Name:        "set_member_roles",
		Description: "Give roles to, or take roles from, a member.",
		Annotations: ann("Set member roles", false, true, true),
	}, s.setMemberRoles)

	addTool(ms, &mcp.Tool{
		Name:        "send_message",
		Description: "Post a message as the bot, for example rules or a welcome text in a new channel. Public: call without confirm first, then with confirm=true once the user approves. Mentions do not ping unless allow_mentions is set.",
		Annotations: ann("Send message", false, false, false),
	}, s.sendMessage)

	s.registerLayouts(ms)
}

// channelTypes maps the names tools accept to Discord channel types.
var channelTypes = map[string]int{
	"text":         discord.ChannelText,
	"voice":        discord.ChannelVoice,
	"category":     discord.ChannelCategory,
	"announcement": discord.ChannelAnnouncement,
	"news":         discord.ChannelAnnouncement,
	"stage":        discord.ChannelStage,
	"forum":        discord.ChannelForum,
	"media":        discord.ChannelMedia,
}

func typeName(t int) string {
	switch t {
	case discord.ChannelText:
		return "text"
	case discord.ChannelVoice:
		return "voice"
	case discord.ChannelCategory:
		return "category"
	case discord.ChannelAnnouncement:
		return "announcement"
	case discord.ChannelStage:
		return "stage"
	case discord.ChannelForum:
		return "forum"
	case discord.ChannelMedia:
		return "media"
	}
	return fmt.Sprintf("type %d", t)
}

func isVoice(t int) bool { return t == discord.ChannelVoice || t == discord.ChannelStage }

// isMessageable is a channel people type in.
func isMessageable(t int) bool {
	return t == discord.ChannelText || t == discord.ChannelAnnouncement || t == discord.ChannelVoice
}

func isTextLike(t int) bool { return t == discord.ChannelText || t == discord.ChannelAnnouncement }

// parseColor reads #RRGGBB, RRGGBB or 0xRRGGBB. "none" is 0.
func parseColor(s string) (int, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "none" || s == "" {
		return 0, nil
	}
	h := strings.TrimPrefix(strings.TrimPrefix(s, "#"), "0x")
	var n int
	if len(h) != 6 {
		return 0, fmt.Errorf("color %q is not a hex color like #5865F2", s)
	}
	if _, err := fmt.Sscanf(h, "%06x", &n); err != nil {
		return 0, fmt.Errorf("color %q is not a hex color like #5865F2", s)
	}
	return n, nil
}

func colorHex(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("#%06X", n)
}

// permList shows a permission set compactly: administrator implies the rest.
func permList(p discord.Perm) []string {
	if p.Has(discord.Administrator) {
		return []string{"administrator"}
	}
	return p.Names()
}
