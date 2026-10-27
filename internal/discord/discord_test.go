package discord

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPermissionBits(t *testing.T) {
	// Spot-check against the table in the Discord docs.
	for p, want := range map[Perm]uint64{
		ViewChannel:           1 << 10,
		ManageRoles:           1 << 28,
		SendMessagesInThreads: 1 << 38,
		SendVoiceMessages:     1 << 46,
		SetVoiceChannelStatus: 1 << 48,
		BypassSlowmode:        1 << 52,
	} {
		if uint64(p) != want {
			t.Errorf("%v = %d, want %d", p.Names(), uint64(p), want)
		}
	}
	if len(permNames) != 52 {
		t.Errorf("permNames has %d entries", len(permNames))
	}
}

func TestParsePerms(t *testing.T) {
	p, err := ParsePerms([]string{"View Channel", "manage-server", "SEND_MESSAGES", "timeout members", "8"})
	if err != nil {
		t.Fatal(err)
	}
	if p != ViewChannel|ManageGuild|SendMessages|ModerateMembers|Administrator {
		t.Errorf("got %v", p.Names())
	}
	if _, err := ParsePerms([]string{"fly"}); err == nil {
		t.Error("unknown name should fail")
	}
	if got := Perm(1<<10 | 1<<60).Names(); strings.Join(got, ",") != "view_channel,bit_60" {
		t.Errorf("Names = %v", got)
	}
	b, _ := json.Marshal(struct{ P Perm }{ManageRoles})
	if string(b) != `{"P":"268435456"}` {
		t.Errorf("marshal = %s", b)
	}
}

func TestEffective(t *testing.T) {
	const g = "1"
	mod := Role{ID: "2", Permissions: ManageMessages}
	ch := &Channel{Overwrites: []Overwrite{
		{ID: g, Type: OverwriteRole, Deny: SendMessages},
		{ID: "2", Type: OverwriteRole, Allow: SendMessages},
		{ID: "9", Type: OverwriteMember, Deny: ManageMessages},
	}}
	everyone := ViewChannel | SendMessages
	if p := Effective(g, everyone, nil, "", "", ch); p.Has(SendMessages) {
		t.Error("@everyone should be denied")
	}
	if p := Effective(g, everyone, []Role{mod}, "", "", ch); !p.Has(SendMessages | ManageMessages) {
		t.Error("role allow should win over @everyone deny")
	}
	if p := Effective(g, everyone, []Role{mod}, "9", "", ch); p.Has(ManageMessages) {
		t.Error("member deny should win over role")
	}
	if p := Effective(g, 0, nil, "", "", ch); p != 0 {
		t.Error("without view_channel nothing applies")
	}
	muted := &Channel{Overwrites: []Overwrite{{ID: g, Type: OverwriteRole, Deny: SendMessages}}}
	if p := Effective(g, everyone|AttachFiles, nil, "", "", muted); p.Has(AttachFiles) {
		t.Error("no send_messages means no attach_files")
	}
	voice := &Channel{Type: ChannelVoice, Overwrites: []Overwrite{{ID: g, Type: OverwriteRole, Deny: Connect}}}
	if p := Effective(g, ViewChannel|Connect|Speak, nil, "", "", voice); p.Has(Speak) {
		t.Error("no connect means no speak")
	}
	if p := Effective(g, 0, nil, "9", "9", ch); p != AllPerms {
		t.Error("owner has everything")
	}
}

func TestRateLimitRetry(t *testing.T) {
	var n atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) == 1 {
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`{"message":"You are being rate limited.","retry_after":0.01,"global":false}`))
			return
		}
		if r.Header.Get("X-Audit-Log-Reason") != "nova%20estrutura" {
			t.Errorf("reason header = %q", r.Header.Get("X-Audit-Log-Reason"))
		}
		_, _ = w.Write([]byte(`{"id":"5","name":"x","type":0}`))
	}))
	defer ts.Close()
	c := New(Options{Token: "t"})
	c.Base = ts.URL
	ch, err := c.CreateChannel(context.Background(), "1", map[string]any{"name": "x"}, "nova estrutura")
	if err != nil || ch.ID != "5" || n.Load() != 2 {
		t.Fatalf("ch=%v err=%v calls=%d", ch, err, n.Load())
	}
}

func TestErrorMessages(t *testing.T) {
	err := parseError(403, "PUT", "/x", []byte(`{"message":"Missing Permissions","code":50013}`))
	if !IsCode(err, CodeMissingPermissions) || !strings.Contains(err.Error(), "drag the bot's role") {
		t.Errorf("err = %v", err)
	}
	err = parseError(400, "POST", "/x", []byte(`{"code":50035,"message":"Invalid Form Body","errors":{"name":{"_errors":[{"code":"BASE_TYPE_REQUIRED","message":"This field is required"}]}}}`))
	if !strings.Contains(err.Error(), "name: This field is required") {
		t.Errorf("err = %v", err)
	}
}
