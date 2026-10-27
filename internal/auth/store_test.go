package auth

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCleanToken(t *testing.T) {
	for in, want := range map[string]string{
		"  abc.def.ghi \n": "abc.def.ghi",
		`"Bot abc"`:        "abc",
		"bot abc":          "abc",
		"":                 "",
	} {
		if got := CleanToken(in); got != want {
			t.Errorf("CleanToken(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISCORD_MCP_CONFIG_DIR", dir)
	t.Setenv("DISCORD_TOKEN", "")
	t.Setenv("DISCORD_BOT_TOKEN", "")
	if _, err := Load(); err != ErrNoCredentials {
		t.Fatalf("want ErrNoCredentials, got %v", err)
	}
	if err := Save(&Credentials{Token: "tok", Server: "Lab"}); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		fi, _ := os.Stat(filepath.Join(dir, "credentials.json"))
		if fi.Mode().Perm() != 0o600 {
			t.Errorf("mode = %v", fi.Mode().Perm())
		}
	}
	c, err := Load()
	if err != nil || c.Token != "tok" || c.Server != "Lab" {
		t.Fatalf("Load = %+v, %v", c, err)
	}
	t.Setenv("DISCORD_TOKEN", "Bot env")
	if c, _ := Load(); c.Token != "env" {
		t.Errorf("env should win, got %q", c.Token)
	}
}
