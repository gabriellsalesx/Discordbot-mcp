package discord

import (
	"fmt"
	"math/bits"
	"strconv"
	"strings"
)

// Perm is a Discord permission bitfield. The API sends it as a decimal string.
type Perm uint64

// Permission flags, from https://discord.com/developers/docs/topics/permissions.
const (
	CreateInstantInvite Perm = 1 << iota
	KickMembers
	BanMembers
	Administrator
	ManageChannels
	ManageGuild
	AddReactions
	ViewAuditLog
	PrioritySpeaker
	Stream
	ViewChannel
	SendMessages
	SendTTSMessages
	ManageMessages
	EmbedLinks
	AttachFiles
	ReadMessageHistory
	MentionEveryone
	UseExternalEmojis
	ViewGuildInsights
	Connect
	Speak
	MuteMembers
	DeafenMembers
	MoveMembers
	UseVAD
	ChangeNickname
	ManageNicknames
	ManageRoles
	ManageWebhooks
	ManageGuildExpressions
	UseApplicationCommands
	RequestToSpeak
	ManageEvents
	ManageThreads
	CreatePublicThreads
	CreatePrivateThreads
	UseExternalStickers
	SendMessagesInThreads
	UseEmbeddedActivities
	ModerateMembers
	ViewCreatorMonetizationAnalytics
	UseSoundboard
	CreateGuildExpressions
	CreateEvents
	UseExternalSounds
	SendVoiceMessages
	_
	SetVoiceChannelStatus
	SendPolls
	UseExternalApps
	PinMessages
	BypassSlowmode
)

// permNames lists every known flag in bit order, by its API name in lower case.
var permNames = []struct {
	Name string
	Bit  Perm
}{
	{"create_instant_invite", CreateInstantInvite},
	{"kick_members", KickMembers},
	{"ban_members", BanMembers},
	{"administrator", Administrator},
	{"manage_channels", ManageChannels},
	{"manage_guild", ManageGuild},
	{"add_reactions", AddReactions},
	{"view_audit_log", ViewAuditLog},
	{"priority_speaker", PrioritySpeaker},
	{"stream", Stream},
	{"view_channel", ViewChannel},
	{"send_messages", SendMessages},
	{"send_tts_messages", SendTTSMessages},
	{"manage_messages", ManageMessages},
	{"embed_links", EmbedLinks},
	{"attach_files", AttachFiles},
	{"read_message_history", ReadMessageHistory},
	{"mention_everyone", MentionEveryone},
	{"use_external_emojis", UseExternalEmojis},
	{"view_guild_insights", ViewGuildInsights},
	{"connect", Connect},
	{"speak", Speak},
	{"mute_members", MuteMembers},
	{"deafen_members", DeafenMembers},
	{"move_members", MoveMembers},
	{"use_vad", UseVAD},
	{"change_nickname", ChangeNickname},
	{"manage_nicknames", ManageNicknames},
	{"manage_roles", ManageRoles},
	{"manage_webhooks", ManageWebhooks},
	{"manage_guild_expressions", ManageGuildExpressions},
	{"use_application_commands", UseApplicationCommands},
	{"request_to_speak", RequestToSpeak},
	{"manage_events", ManageEvents},
	{"manage_threads", ManageThreads},
	{"create_public_threads", CreatePublicThreads},
	{"create_private_threads", CreatePrivateThreads},
	{"use_external_stickers", UseExternalStickers},
	{"send_messages_in_threads", SendMessagesInThreads},
	{"use_embedded_activities", UseEmbeddedActivities},
	{"moderate_members", ModerateMembers},
	{"view_creator_monetization_analytics", ViewCreatorMonetizationAnalytics},
	{"use_soundboard", UseSoundboard},
	{"create_guild_expressions", CreateGuildExpressions},
	{"create_events", CreateEvents},
	{"use_external_sounds", UseExternalSounds},
	{"send_voice_messages", SendVoiceMessages},
	{"set_voice_channel_status", SetVoiceChannelStatus},
	{"send_polls", SendPolls},
	{"use_external_apps", UseExternalApps},
	{"pin_messages", PinMessages},
	{"bypass_slowmode", BypassSlowmode},
}

// permAliases maps the names the Discord app shows, and older API names, to
// the API name.
var permAliases = map[string]string{
	"manage_server":              "manage_guild",
	"read_messages":              "view_channel",
	"view_channels":              "view_channel",
	"read_message":               "view_channel",
	"admin":                      "administrator",
	"use_voice_activity":         "use_vad",
	"timeout_members":            "moderate_members",
	"manage_emojis":              "manage_guild_expressions",
	"manage_emojis_and_stickers": "manage_guild_expressions",
	"manage_expressions":         "manage_guild_expressions",
	"use_slash_commands":         "use_application_commands",
	"start_activities":           "use_embedded_activities",
	"create_invite":              "create_instant_invite",
	"send_tts":                   "send_tts_messages",
	"video":                      "stream",
	"manage_permissions":         "manage_roles",
	"create_polls":               "send_polls",
}

// retired bits Discord still sets in default permissions; not shown.
const retired Perm = 1 << 47

// AllPerms has every known flag set.
var AllPerms = func() Perm {
	var p Perm
	for _, n := range permNames {
		p |= n.Bit
	}
	return p
}()

// ParsePerm reads one permission name, in any case, with spaces or hyphens.
func ParsePerm(s string) (Perm, error) {
	key := strings.ToLower(strings.TrimSpace(s))
	key = strings.NewReplacer(" ", "_", "-", "_").Replace(key)
	if a, ok := permAliases[key]; ok {
		key = a
	}
	for _, n := range permNames {
		if n.Name == key {
			return n.Bit, nil
		}
	}
	return 0, fmt.Errorf("unknown permission %q; use names like view_channel, send_messages, manage_roles (see the list_roles output for more)", s)
}

// ParsePerms combines a list of permission names. A single decimal number is
// read as a raw bitfield.
func ParsePerms(names []string) (Perm, error) {
	var p Perm
	for _, s := range names {
		if s = strings.TrimSpace(s); s == "" {
			continue
		}
		if n, err := strconv.ParseUint(s, 10, 64); err == nil {
			p |= Perm(n)
			continue
		}
		b, err := ParsePerm(s)
		if err != nil {
			return 0, err
		}
		p |= b
	}
	return p, nil
}

// Names lists the flags set in p by API name. Unknown bits show as bit_N.
func (p Perm) Names() []string {
	out := []string{}
	for _, n := range permNames {
		if p&n.Bit != 0 {
			out = append(out, n.Name)
			p &^= n.Bit
		}
	}
	p &^= retired
	for p != 0 {
		b := bits.TrailingZeros64(uint64(p))
		out = append(out, "bit_"+strconv.Itoa(b))
		p &^= 1 << b
	}
	return out
}

// Has reports whether every flag in q is set in p.
func (p Perm) Has(q Perm) bool { return p&q == q }

// MarshalJSON writes the bitfield as a decimal string, as Discord expects.
func (p Perm) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strconv.FormatUint(uint64(p), 10) + `"`), nil
}

// UnmarshalJSON accepts a decimal string or number.
func (p *Perm) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*p = 0
		return nil
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fmt.Errorf("permission bitfield %s: %w", b, err)
	}
	*p = Perm(n)
	return nil
}

// Overwrite target types.
const (
	OverwriteRole   = 0
	OverwriteMember = 1
)

// Effective computes what a set of roles (and optionally one member) can do in
// a channel: the role permissions, then the @everyone, role and member
// overwrites in that order, as Discord documents. everyone is the @everyone
// role, roles are the other roles held, member is the member id or "".
func Effective(guildID string, everyone Perm, roles []Role, member string, ownerID string, ch *Channel) Perm {
	if member != "" && member == ownerID {
		return AllPerms
	}
	base := everyone
	for _, r := range roles {
		base |= r.Permissions
	}
	if base.Has(Administrator) {
		return AllPerms
	}
	if ch == nil {
		return base
	}
	perm := base
	for _, o := range ch.Overwrites {
		if o.Type == OverwriteRole && o.ID == guildID {
			perm = perm&^o.Deny | o.Allow
		}
	}
	var allow, deny Perm
	for _, o := range ch.Overwrites {
		if o.Type != OverwriteRole || o.ID == guildID {
			continue
		}
		for _, r := range roles {
			if r.ID == o.ID {
				allow |= o.Allow
				deny |= o.Deny
			}
		}
	}
	perm = perm&^deny | allow
	for _, o := range ch.Overwrites {
		if o.Type == OverwriteMember && member != "" && o.ID == member {
			perm = perm&^o.Deny | o.Allow
		}
	}
	// Without View Channel nothing else in the channel applies.
	if !perm.Has(ViewChannel) {
		return 0
	}
	// Implicit denials Discord applies on top of the overwrites.
	if !perm.Has(SendMessages) {
		perm &^= MentionEveryone | SendTTSMessages | AttachFiles | EmbedLinks
	}
	if (ch.Type == ChannelVoice || ch.Type == ChannelStage) && !perm.Has(Connect) {
		perm &^= voicePerms
	}
	return perm
}

// voicePerms are lost with Connect in a voice channel.
const voicePerms = Speak | Stream | UseVAD | PrioritySpeaker | MuteMembers | DeafenMembers | MoveMembers | UseSoundboard | UseExternalSounds | RequestToSpeak | UseEmbeddedActivities | SetVoiceChannelStatus
