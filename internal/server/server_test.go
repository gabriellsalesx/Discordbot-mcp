package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gabriellsalesx/discord-mcp/internal/discord"
)

const (
	gid      = "100000000000000001"
	botID    = "200000000000000001"
	userID   = "200000000000000002"
	catID    = "300000000000000001"
	generalC = "300000000000000002"
	rulesC   = "300000000000000003"
	voiceC   = "300000000000000004"
	privC    = "300000000000000005"
	modRole  = "400000000000000001"
	botRole  = "400000000000000002"
)

// fakeDiscord keeps a tiny server in memory and answers like the API.
type fakeDiscord struct {
	mu       sync.Mutex
	calls    []string
	channels []discord.Channel
	roles    []discord.Role
	members  map[string]*discord.Member
	next     int
	posCalls [][]map[string]any
}

func newFake() *fakeDiscord {
	everyoneDeny := []discord.Overwrite{{ID: gid, Type: 0, Deny: discord.ViewChannel}}
	return &fakeDiscord{
		channels: []discord.Channel{
			{ID: catID, Type: discord.ChannelCategory, Name: "Comunidade", Position: 0},
			{ID: generalC, Type: discord.ChannelText, Name: "geral", Position: 0, ParentID: catID},
			{ID: rulesC, Type: discord.ChannelText, Name: "regras", Position: 1, ParentID: catID},
			{ID: voiceC, Type: discord.ChannelVoice, Name: "Sala", Position: 0, ParentID: catID},
			{ID: privC, Type: discord.ChannelText, Name: "staff", Position: 2, Overwrites: everyoneDeny},
		},
		roles: []discord.Role{
			{ID: gid, Name: "@everyone", Position: 0, Permissions: discord.ViewChannel | discord.SendMessages},
			{ID: modRole, Name: "Moderador", Position: 1, Permissions: discord.ManageMessages, Color: 0x5865F2},
			{ID: botRole, Name: "Gerente", Position: 2, Permissions: discord.ManageChannels | discord.ManageRoles, Managed: true},
		},
		members: map[string]*discord.Member{
			botID:  {User: discord.User{ID: botID, Username: "gerente", Bot: true}, Roles: []string{botRole}},
			userID: {User: discord.User{ID: userID, Username: "ana"}, Roles: []string{}},
		},
		next: 500000000000000000,
	}
}

func (f *fakeDiscord) id() string {
	f.next++
	return fmt.Sprint(f.next)
}

func (f *fakeDiscord) channel(id string) *discord.Channel {
	for i := range f.channels {
		if f.channels[i].ID == id {
			return &f.channels[i]
		}
	}
	return nil
}

var (
	reOverwrite  = regexp.MustCompile(`^/channels/(\d+)/permissions/(\d+)$`)
	reChannel    = regexp.MustCompile(`^/channels/(\d+)$`)
	reMemberRole = regexp.MustCompile(`^/guilds/\d+/members/(\d+)/roles/(\d+)$`)
	reMember     = regexp.MustCompile(`^/guilds/\d+/members/(\d+)$`)
	reRole       = regexp.MustCompile(`^/guilds/\d+/roles/(\d+)$`)
)

func (f *fakeDiscord) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, r.Method+" "+r.URL.Path)
	if r.Header.Get("Authorization") != "Bot test-token" {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"message":"401: Unauthorized","code":0}`))
		return
	}
	var body any
	_ = json.NewDecoder(r.Body).Decode(&body)
	obj, _ := body.(map[string]any)
	w.Header().Set("Content-Type", "application/json")
	reply := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	route := r.Method + " " + r.URL.Path

	switch {
	case route == "GET /applications/@me":
		reply(discord.Application{ID: botID})
	case route == "GET /users/@me":
		reply(f.members[botID].User)
	case route == "GET /users/@me/guilds":
		reply([]discord.PartialGuild{{ID: gid, Name: "Vought Lab", Permissions: discord.ManageChannels | discord.ManageRoles, ApproximateMemberCount: 2}})
	case route == "GET /guilds/"+gid:
		reply(discord.Guild{ID: gid, Name: "Vought Lab", OwnerID: userID, Roles: f.roles})
	case route == "GET /guilds/"+gid+"/channels":
		reply(f.channels)
	case route == "POST /guilds/"+gid+"/channels":
		var ch discord.Channel
		raw, _ := json.Marshal(obj)
		_ = json.Unmarshal(raw, &ch)
		ch.ID = f.id()
		f.channels = append(f.channels, ch)
		reply(ch)
	case route == "PATCH /guilds/"+gid+"/channels":
		var list []map[string]any
		raw, _ := json.Marshal(body)
		_ = json.Unmarshal(raw, &list)
		f.posCalls = append(f.posCalls, list)
		for _, e := range list {
			ch := f.channel(e["id"].(string))
			if p, ok := e["position"].(float64); ok {
				ch.Position = int(p)
			}
			if p, ok := e["parent_id"]; ok {
				ch.ParentID, _ = p.(string)
			}
		}
		w.WriteHeader(204)
	case reOverwrite.MatchString(r.URL.Path):
		m := reOverwrite.FindStringSubmatch(r.URL.Path)
		ch := f.channel(m[1])
		ch.Overwrites = slices.DeleteFunc(ch.Overwrites, func(o discord.Overwrite) bool { return o.ID == m[2] })
		if r.Method == http.MethodPut {
			var o discord.Overwrite
			raw, _ := json.Marshal(obj)
			_ = json.Unmarshal(raw, &o)
			o.ID = m[2]
			ch.Overwrites = append(ch.Overwrites, o)
		}
		w.WriteHeader(204)
	case reChannel.MatchString(r.URL.Path):
		ch := f.channel(reChannel.FindStringSubmatch(r.URL.Path)[1])
		switch r.Method {
		case http.MethodDelete:
			f.channels = slices.DeleteFunc(f.channels, func(c discord.Channel) bool { return c.ID == ch.ID })
		case http.MethodPatch:
			if v, ok := obj["name"].(string); ok {
				ch.Name = v
			}
			if v, ok := obj["parent_id"]; ok {
				ch.ParentID, _ = v.(string)
			}
			if v, ok := obj["permission_overwrites"]; ok {
				raw, _ := json.Marshal(v)
				_ = json.Unmarshal(raw, &ch.Overwrites)
			}
		}
		reply(ch)
	case route == "GET /guilds/"+gid+"/roles/member-counts":
		reply(map[string]int{modRole: 3})
	case route == "POST /guilds/"+gid+"/roles":
		var role discord.Role
		raw, _ := json.Marshal(obj)
		_ = json.Unmarshal(raw, &role)
		role.ID = f.id()
		if c, ok := obj["colors"].(map[string]any); ok {
			role.Color = int(c["primary_color"].(float64))
		}
		f.roles = append(f.roles, role)
		reply(role)
	case reRole.MatchString(r.URL.Path):
		id := reRole.FindStringSubmatch(r.URL.Path)[1]
		i := slices.IndexFunc(f.roles, func(x discord.Role) bool { return x.ID == id })
		if r.Method == http.MethodPatch {
			raw, _ := json.Marshal(obj)
			_ = json.Unmarshal(raw, &f.roles[i])
			reply(f.roles[i])
			return
		}
		f.roles = slices.Delete(f.roles, i, i+1)
		w.WriteHeader(204)
	case reMemberRole.MatchString(r.URL.Path):
		m := reMemberRole.FindStringSubmatch(r.URL.Path)
		mem := f.members[m[1]]
		if r.Method == http.MethodPut {
			mem.Roles = append(mem.Roles, m[2])
		} else {
			mem.Roles = slices.DeleteFunc(mem.Roles, func(id string) bool { return id == m[2] })
		}
		w.WriteHeader(204)
	case route == "GET /guilds/"+gid+"/members/search":
		var out []discord.Member
		for _, m := range f.members {
			if strings.HasPrefix(m.User.Username, r.URL.Query().Get("query")) {
				out = append(out, *m)
			}
		}
		reply(out)
	case reMember.MatchString(r.URL.Path):
		reply(f.members[reMember.FindStringSubmatch(r.URL.Path)[1]])
	default:
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"message":"Unknown route","code":0}`))
	}
}

func connect(t *testing.T) (*mcp.ClientSession, *fakeDiscord) {
	t.Helper()
	fake := newFake()
	ts := httptest.NewServer(fake)
	t.Cleanup(ts.Close)
	srv := New(Config{Version: "test", NewClient: func() (*discord.Client, error) {
		c := discord.New(discord.Options{Token: "test-token"})
		c.Base = ts.URL
		return c, nil
	}})
	ct, st := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs, fake
}

func callRaw(t *testing.T, cs *mcp.ClientSession, name string, args any) (map[string]any, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if res.IsError {
		return map[string]any{"error": text}, true
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("%s: output is not JSON: %s", name, text)
	}
	return out, false
}

func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) map[string]any {
	t.Helper()
	out, isErr := callRaw(t, cs, name, args)
	if isErr {
		t.Fatalf("%s: %v", name, out["error"])
	}
	return out
}

// TestSchemasPortable guards against JSON Schema features that Gemini, Codex
// and other clients reject: type arrays (nullable), $ref/$defs and anyOf.
func TestSchemasPortable(t *testing.T) {
	cs, _ := connect(t)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Tools) != 24 {
		t.Errorf("got %d tools, want 24", len(res.Tools))
	}
	for _, tool := range res.Tools {
		raw, _ := json.Marshal(tool.InputSchema)
		s := string(raw)
		for _, bad := range []string{`"type":[`, `"$ref"`, `"$defs"`, `"anyOf"`, `"oneOf"`, `"null"`} {
			if strings.Contains(s, bad) {
				t.Errorf("%s input schema contains %s: %s", tool.Name, bad, s)
			}
		}
		if tool.Annotations == nil || tool.Annotations.DestructiveHint == nil {
			t.Errorf("%s has no explicit destructiveHint", tool.Name)
		}
		if tool.OutputSchema != nil {
			t.Errorf("%s has an output schema; keep outputs schema-less for portability", tool.Name)
		}
		if len(tool.Description) > 400 {
			t.Errorf("%s description is %d chars; keep it short", tool.Name, len(tool.Description))
		}
	}
	if len(instructions) > 512 {
		t.Errorf("instructions are %d chars; keep them under 512", len(instructions))
	}
}

func TestListChannelsTree(t *testing.T) {
	cs, _ := connect(t)
	out := call(t, cs, "list_channels", map[string]any{})
	cats := out["categories"].([]any)
	if len(cats) != 1 {
		t.Fatalf("categories = %v", cats)
	}
	chs := cats[0].(map[string]any)["channels"].([]any)
	var names []string
	for _, c := range chs {
		names = append(names, c.(map[string]any)["name"].(string))
	}
	if strings.Join(names, ",") != "geral,regras,Sala" {
		t.Errorf("order = %v, want text channels by position, then voice", names)
	}
	unc := out["uncategorized"].([]any)
	if len(unc) != 1 || unc[0].(map[string]any)["private"] != true {
		t.Errorf("staff should be uncategorized and private: %v", unc)
	}
}

func TestCreatePrivateChannelInCategory(t *testing.T) {
	cs, fake := connect(t)
	out := call(t, cs, "create_channel", map[string]any{
		"name": "moderacao", "category": "comunidade", "private": true, "visible_to": []string{"@Moderador"},
	})
	if out["category"] != "Comunidade" {
		t.Errorf("category = %v", out["category"])
	}
	ch := fake.channel(out["id"].(string))
	get := func(id string) discord.Overwrite {
		o, _ := findOverwrite(ch.Overwrites, id)
		return o
	}
	if !get(gid).Deny.Has(discord.ViewChannel) {
		t.Error("@everyone should be denied view_channel")
	}
	if !get(modRole).Allow.Has(discord.ViewChannel) {
		t.Error("Moderador should be allowed view_channel")
	}
	if o := get(botID); o.Type != discord.OverwriteMember || !o.Allow.Has(discord.ViewChannel|discord.ManageChannels) {
		t.Errorf("the bot must keep access to the private channel: %+v", o)
	}
}

func TestSetPermissionsMergesAndSyncsChildren(t *testing.T) {
	cs, fake := connect(t)
	// regras gets its own override, so it is no longer synced with the category.
	call(t, cs, "set_channel_permissions", map[string]any{"channel": "#regras", "target": "everyone", "deny": []string{"send_messages"}})
	out := call(t, cs, "set_channel_permissions", map[string]any{
		"channel": "Comunidade", "target": "Moderador", "allow": []string{"manage messages", "Manage Threads"},
	})
	updated := out["synced_channels_updated"].([]any)
	if len(updated) != 2 || slices.Contains(updated, any("regras")) {
		t.Errorf("only synced channels should follow the category, got %v", updated)
	}
	if o, _ := findOverwrite(fake.channel(generalC).Overwrites, modRole); !o.Allow.Has(discord.ManageMessages | discord.ManageThreads) {
		t.Errorf("geral should have the new override: %+v", o)
	}

	// Merge: denying one permission keeps the other allowed.
	call(t, cs, "set_channel_permissions", map[string]any{"channel": "geral", "target": "Moderador", "deny": []string{"manage_threads"}})
	o, _ := findOverwrite(fake.channel(generalC).Overwrites, modRole)
	if !o.Allow.Has(discord.ManageMessages) || o.Allow.Has(discord.ManageThreads) || !o.Deny.Has(discord.ManageThreads) {
		t.Errorf("merge failed: %+v", o)
	}

	// Resetting everything removes the override.
	call(t, cs, "set_channel_permissions", map[string]any{"channel": "geral", "target": "Moderador", "remove": true})
	if _, ok := findOverwrite(fake.channel(generalC).Overwrites, modRole); ok {
		t.Error("override should be gone")
	}
}

func TestDeleteNeedsConfirm(t *testing.T) {
	cs, fake := connect(t)
	out := call(t, cs, "delete_channel", map[string]any{"channel": "regras"})
	if out["confirmation_required"] != true {
		t.Fatalf("want a preview, got %v", out)
	}
	if fake.channel(rulesC) == nil {
		t.Fatal("deleted without confirm")
	}
	call(t, cs, "delete_channel", map[string]any{"channel": "regras", "confirm": true})
	if fake.channel(rulesC) != nil {
		t.Fatal("not deleted")
	}

	out = call(t, cs, "delete_role", map[string]any{"role": "Moderador"})
	if out["confirmation_required"] != true || out["details"].(map[string]any)["members_losing_it"].(float64) != 3 {
		t.Fatalf("role preview = %v", out)
	}
}

func TestOrganizeChannels(t *testing.T) {
	cs, fake := connect(t)
	out := call(t, cs, "organize_channels", map[string]any{"channels": []string{"staff", "regras"}, "category": "Comunidade", "sync_permissions": true})
	if fmt.Sprint(out["moved"]) != "[staff]" {
		t.Errorf("moved = %v", out["moved"])
	}
	// One request per parent change, then a single bulk reorder.
	if len(fake.posCalls) != 2 || len(fake.posCalls[0]) != 1 || fake.posCalls[0][0]["lock_permissions"] != true {
		t.Fatalf("position calls = %v", fake.posCalls)
	}
	if got := fmt.Sprint(out["order"]); got != "[staff regras geral Sala]" {
		t.Errorf("order = %s", got)
	}
	if fake.channel(privC).ParentID != catID || fake.channel(privC).Position != 0 || fake.channel(generalC).Position != 2 {
		t.Errorf("state not applied: %+v", fake.channels)
	}

	_, isErr := callRaw(t, cs, "organize_channels", map[string]any{"channels": []string{"Comunidade", "geral"}})
	if !isErr {
		t.Error("mixing categories and channels should fail")
	}
}

func TestRolesAndMembers(t *testing.T) {
	cs, fake := connect(t)
	out := call(t, cs, "create_role", map[string]any{"name": "Membro", "color": "#1ABC9C", "permissions": []string{"send_messages", "add_reactions"}})
	if out["color"] != "#1ABC9C" || fmt.Sprint(out["permissions"]) != "[add_reactions send_messages]" {
		t.Errorf("create_role = %v", out)
	}
	out = call(t, cs, "update_role", map[string]any{"role": "Membro", "add_permissions": []string{"attach_files"}, "remove_permissions": []string{"add_reactions"}})
	if fmt.Sprint(out["added"]) != "[attach_files]" || fmt.Sprint(out["removed"]) != "[add_reactions]" {
		t.Errorf("update_role = %v", out)
	}

	out = call(t, cs, "list_roles", map[string]any{})
	roles := out["roles"].([]any)
	top := roles[0].(map[string]any)
	if top["name"] != "@Gerente" || top["bot_can_manage"] != false {
		t.Errorf("first role should be the bot's own, not manageable: %v", top)
	}
	if out["bot_top_role"] != "Gerente" {
		t.Errorf("bot_top_role = %v", out["bot_top_role"])
	}

	out = call(t, cs, "set_member_roles", map[string]any{"member": "ana", "add": []string{"Moderador"}})
	if fmt.Sprint(out["roles"]) != "[Moderador]" {
		t.Errorf("set_member_roles = %v", out)
	}
	if !slices.Contains(fake.members[userID].Roles, modRole) {
		t.Error("role not given")
	}
}

func TestCheckPermissions(t *testing.T) {
	cs, _ := connect(t)
	out := call(t, cs, "check_permissions", map[string]any{"target": "everyone", "channel": "staff"})
	if fmt.Sprint(out["permissions"]) != "[]" || out["summary"].(map[string]any)["view_channel"] != false {
		t.Errorf("@everyone should not see staff: %v", out)
	}
	out = call(t, cs, "check_permissions", map[string]any{"target": "Moderador", "channel": "geral"})
	if s := out["summary"].(map[string]any); s["manage_messages"] != true || s["send_messages"] != true {
		t.Errorf("Moderador in geral: %v", out)
	}
}

// TestSnowflakeNumbers: ids sent as JSON numbers must not lose precision.
func TestSnowflakeNumbers(t *testing.T) {
	cs, _ := connect(t)
	args := json.RawMessage(`{"channel": 300000000000000003}`)
	out, isErr := callRaw(t, cs, "get_channel", args)
	if isErr || out["name"] != "regras" {
		t.Fatalf("numeric id: %v", out)
	}
}

func TestSendMessageNeedsConfirm(t *testing.T) {
	cs, fake := connect(t)
	out := call(t, cs, "send_message", map[string]any{"channel": "geral", "content": "Bem-vindos!"})
	if out["confirmation_required"] != true {
		t.Fatalf("want a preview: %v", out)
	}
	for _, c := range fake.calls {
		if strings.HasSuffix(c, "/messages") {
			t.Fatal("sent without confirm")
		}
	}
}

func TestOrganizeCategoriesSharingAChannelName(t *testing.T) {
	cs, fake := connect(t)
	fake.channels = append(fake.channels,
		discord.Channel{ID: "300000000000000009", Type: discord.ChannelCategory, Name: "Geral", Position: 1},
	)
	out := call(t, cs, "organize_channels", map[string]any{"channels": []string{"Geral", "Comunidade"}})
	if fmt.Sprint(out["order"]) != "[Geral Comunidade]" {
		t.Errorf("order = %v", out["order"])
	}
}

func TestInviteBot(t *testing.T) {
	cs, _ := connect(t)
	out := call(t, cs, "invite_bot", map[string]any{"server_id": "900000000000000001"})
	u := out["url"].(string)
	if !strings.Contains(u, "client_id="+botID) || !strings.Contains(u, "guild_id=900000000000000001") || !strings.Contains(u, "disable_guild_select=true") {
		t.Errorf("url = %s", u)
	}
}

func TestMemberNeedsExactName(t *testing.T) {
	cs, _ := connect(t)
	if out, isErr := callRaw(t, cs, "set_member_roles", map[string]any{"member": "an", "add": []string{"Moderador"}}); !isErr {
		t.Errorf("a prefix must not pick a member: %v", out)
	}
}

func TestSaveAndApplyLayout(t *testing.T) {
	t.Setenv("DISCORD_MCP_CONFIG_DIR", t.TempDir())
	cs, fake := connect(t)
	fake.channel(privC).Overwrites = append(fake.channel(privC).Overwrites, discord.Overwrite{ID: modRole, Type: 0, Allow: discord.ViewChannel})
	out := call(t, cs, "save_layout", map[string]any{"name": "Comunidade padrão"})
	if out["roles"].(float64) != 1 || out["channels"].(float64) != 4 {
		t.Fatalf("save_layout = %v", out)
	}
	if out := call(t, cs, "list_layouts", map[string]any{}); len(out["layouts"].([]any)) != 1 {
		t.Fatalf("list_layouts = %v", out)
	}

	// Same server: everything is already there.
	out = call(t, cs, "apply_layout", map[string]any{"name": "comunidade padrao", "confirm": true})
	res := out["result"].(map[string]any)
	if len(res["roles_created"].([]any)) != 0 || len(res["channels_created"].([]any)) != 0 {
		t.Fatalf("nothing should be created: %v", res)
	}

	// Remove the role and the private channel, then rebuild them.
	fake.mu.Lock()
	fake.roles = slices.DeleteFunc(fake.roles, func(r discord.Role) bool { return r.ID == modRole })
	fake.channels = slices.DeleteFunc(fake.channels, func(c discord.Channel) bool { return c.ID == privC })
	fake.mu.Unlock()
	out = call(t, cs, "apply_layout", map[string]any{"name": "Comunidade padrão"})
	if out["confirmation_required"] != true {
		t.Fatalf("want a preview: %v", out)
	}
	if len(fake.channels) != 4 {
		t.Fatal("preview created channels")
	}
	out = call(t, cs, "apply_layout", map[string]any{"name": "Comunidade padrão", "confirm": true})
	res = out["result"].(map[string]any)
	if fmt.Sprint(res["roles_created"]) != "[Moderador]" || fmt.Sprint(res["channels_created"]) != "[staff]" {
		t.Fatalf("apply = %v", res)
	}
	staff := fake.channels[len(fake.channels)-1]
	newMod := fake.roles[len(fake.roles)-1]
	if o, _ := findOverwrite(staff.Overwrites, newMod.ID); !o.Allow.Has(discord.ViewChannel) {
		t.Errorf("staff should be visible to the new Moderador role: %+v", staff.Overwrites)
	}
	if o, _ := findOverwrite(staff.Overwrites, botID); !o.Allow.Has(discord.ViewChannel) {
		t.Errorf("the bot must keep access to the rebuilt private channel: %+v", staff.Overwrites)
	}
	if newMod.Color != 0x5865F2 {
		t.Errorf("color not kept: %+v", newMod)
	}
	// The fake bot lacks manage_messages, so it cannot grant it and says so.
	if newMod.Permissions.Has(discord.ManageMessages) || !strings.Contains(fmt.Sprint(res["warnings"]), "manage_messages") {
		t.Errorf("want manage_messages skipped with a warning: %+v %v", newMod, res["warnings"])
	}
}
