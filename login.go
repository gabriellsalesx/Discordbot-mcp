package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/gabriellsalesx/Discordbot-mcp/internal/auth"
	"github.com/gabriellsalesx/Discordbot-mcp/internal/discord"
	"github.com/gabriellsalesx/Discordbot-mcp/internal/server"
)

const portalURL = "https://discord.com/developers/applications"

const loginHelp = `Usage: discord-mcp login [flags]

Connects your Discord bot. Opens the Developer Portal, asks for the bot token
(hidden input, or piped on stdin), checks it, and then opens the link that
adds the bot to your server.

`

const loginSteps = `
Discord only lets tools act through a bot you own. It takes a minute:

  1. In the page that just opened, click "New Application", give it a name
     (for example "Server Manager") and create it.
  2. Open the "Bot" tab, click "Reset Token" and then "Copy".
  3. Optional, for find_members and naming members in tools: on the same tab
     turn on "Server Members Intent" and save.

`

func login(ctx context.Context, args []string) error {
	fs := newFlagSet("login", loginHelp)
	srv := fs.String("server", server.EnvServer(), "default server (id or name)")
	admin := fs.Bool("admin", false, "ask for Administrator when adding the bot to a server")
	noInvite := fs.Bool("no-invite", false, "do not open the invite link")
	_ = fs.Parse(args) // ExitOnError: never returns an error

	interactive := term.IsTerminal(int(os.Stdin.Fd()))
	var raw string
	if interactive {
		fmt.Fprint(os.Stderr, loginSteps)
		openURL(portalURL)
		fmt.Fprintf(os.Stderr, "(If no browser opened, go to %s)\n\nPaste the bot token (input is hidden): ", portalURL)
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return err
		}
		raw = string(b)
	} else {
		// Piped input, e.g. from a password manager.
		b, err := io.ReadAll(io.LimitReader(bufio.NewReader(os.Stdin), 4<<10))
		if err != nil {
			return err
		}
		raw = string(b)
	}
	token := auth.CleanToken(raw)
	if token == "" {
		return errors.New("no token given")
	}

	c := discord.New(discord.Options{Token: token, UserAgent: discord.UserAgent(appVersion())})
	me, err := c.Me(ctx)
	if err != nil {
		return fmt.Errorf("the token was not accepted: %w", err)
	}
	if !me.Bot {
		return errors.New("that is a user account token; use a bot token from the Bot tab of your application (user tokens break Discord's terms)")
	}
	app, err := c.Application(ctx)
	if err != nil {
		return err
	}
	creds := &auth.Credentials{Token: token, AppID: app.ID, BotID: me.ID, BotName: me.Username, Server: *srv}
	if err := auth.Save(creds); err != nil {
		return err
	}
	path, _ := auth.Path()
	fmt.Printf("Bot %s connected. Token saved to %s\n", me.Username, path)
	if !app.MembersIntent() {
		fmt.Println("Server Members Intent is off: find_members and member names will not work (ids still do).")
	}

	guilds, err := c.Guilds(ctx)
	if err != nil {
		return err
	}
	if len(guilds) > 0 {
		names := make([]string, len(guilds))
		for i, g := range guilds {
			names[i] = g.Name
		}
		fmt.Printf("Already in: %s\n", strings.Join(names, ", "))
	}
	if *noInvite || !interactive {
		if len(guilds) == 0 {
			fmt.Println("\nAdd the bot to a server with: discord-mcp invite")
		}
		return nil
	}
	if len(guilds) > 0 && !confirm("Add the bot to another server? [y/N] ") {
		return nil
	}
	joined, err := inviteAndWait(ctx, c, app.ID, *admin, guilds)
	if err != nil {
		return err
	}
	if joined != nil && creds.Server == "" && len(guilds) == 0 {
		creds.Server = joined.ID
		if err := auth.Save(creds); err != nil {
			return err
		}
	}
	return nil
}

const inviteHelp = `Usage: discord-mcp invite [flags]

Opens the link that adds your bot to a server and waits until it joins.

`

func invite(ctx context.Context, args []string) error {
	fs := newFlagSet("invite", inviteHelp)
	admin := fs.Bool("admin", false, "ask for Administrator instead of the individual permissions")
	printOnly := fs.Bool("print", false, "only print the link")
	_ = fs.Parse(args)

	creds, err := auth.Load()
	if err != nil {
		return err
	}
	c := discord.New(discord.Options{Token: creds.Token, UserAgent: discord.UserAgent(appVersion())})
	appID := creds.AppID
	if appID == "" {
		app, err := c.Application(ctx)
		if err != nil {
			return err
		}
		appID = app.ID
	}
	if *printOnly {
		fmt.Println(inviteURL(appID, *admin))
		return nil
	}
	before, err := c.Guilds(ctx)
	if err != nil {
		return err
	}
	_, err = inviteAndWait(ctx, c, appID, *admin, before)
	return err
}

func inviteURL(appID string, admin bool) string {
	perms := discord.InvitePerms
	if admin {
		perms = discord.Administrator
	}
	return discord.InviteURL(appID, perms, "")
}

// inviteAndWait opens the invite link and polls until the bot shows up in a
// server it was not in before.
func inviteAndWait(ctx context.Context, c *discord.Client, appID string, admin bool, before []discord.PartialGuild) (*discord.PartialGuild, error) {
	link := inviteURL(appID, admin)
	fmt.Printf("\nOpening the invite link. Pick your server and click Authorize:\n%s\n\nWaiting for the bot to join (Ctrl+C to stop)...\n", link)
	openURL(link)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	tick := time.NewTicker(3 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				fmt.Println("Stopped waiting. Run `discord-mcp invite` when you are ready.")
				return nil, nil
			}
			return nil, ctx.Err()
		case <-tick.C:
		}
		now, err := c.Guilds(ctx)
		if err != nil {
			continue
		}
		for _, g := range now {
			if !slices.ContainsFunc(before, func(b discord.PartialGuild) bool { return b.ID == g.ID }) {
				fmt.Printf("The bot joined %s. It is ready to use from your MCP client.\n", g.Name)
				fmt.Println("Tip: in Server Settings > Roles, keep the bot's role above the roles it should manage.")
				return &g, nil
			}
		}
	}
}

func status(ctx context.Context) error {
	creds, err := auth.Load()
	if err != nil {
		return err
	}
	source := "environment"
	if auth.FromEnv() == nil {
		source, _ = auth.Path()
	}
	c := discord.New(discord.Options{Token: creds.Token, UserAgent: discord.UserAgent(appVersion())})
	me, err := c.Me(ctx)
	if err != nil {
		return err
	}
	app, err := c.Application(ctx)
	if err != nil {
		return err
	}
	guilds, err := c.Guilds(ctx)
	if err != nil {
		return err
	}
	intent := "off (find_members needs it)"
	if app.MembersIntent() {
		intent = "on"
	}
	def := server.EnvServer()
	if def == "" {
		def = creds.Server
	}
	fmt.Printf("Bot: %s (id %s)\nToken from: %s\nServer Members Intent: %s\nDefault server: %s\n\nServers:\n", me.Username, me.ID, source, intent, orNone(def))
	if len(guilds) == 0 {
		fmt.Println("  (none yet; run `discord-mcp invite`)")
	}
	for _, g := range guilds {
		role := "limited"
		switch {
		case g.Owner || g.Permissions.Has(discord.Administrator):
			role = "administrator"
		case g.Permissions.Has(discord.ManageChannels | discord.ManageRoles):
			role = "manage channels and roles"
		}
		fmt.Printf("  - %s  (id %s, %d members, %s)\n", g.Name, g.ID, g.ApproximateMemberCount, role)
	}
	return nil
}

func logout() error {
	if err := auth.Delete(); err != nil {
		return err
	}
	fmt.Println("Stored token removed. The bot stays in your servers; reset its token in the Developer Portal to revoke it.")
	return nil
}

func confirm(label string) bool {
	fmt.Fprint(os.Stderr, label)
	s, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "y" || s == "yes" || s == "s" || s == "sim"
}

// openURL opens a link in the default browser, best effort.
func openURL(u string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	cmd.Stdout, cmd.Stderr = nil, nil
	if err := cmd.Start(); err == nil {
		go func() { _ = cmd.Wait() }()
	}
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
