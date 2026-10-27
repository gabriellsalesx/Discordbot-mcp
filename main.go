// Command discord-mcp is an MCP server that manages Discord servers through
// your own bot. Run it without arguments from an MCP client (stdio), or use
// `discord-mcp login` once to connect the bot.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gabriellsalesx/discord-mcp/internal/server"
)

// version is set by the release build with -ldflags "-X main.version=...".
var version = ""

const usage = `discord-mcp: MCP server to manage Discord servers with your bot

Usage:
  discord-mcp            run the MCP server over stdio (what MCP clients call)
  discord-mcp login      connect your bot (guided, opens the Developer Portal)
  discord-mcp invite     add the bot to a server
  discord-mcp status     show the bot and the servers it is in
  discord-mcp logout     forget the stored token
  discord-mcp version    print the version

Run "discord-mcp login -h" or "discord-mcp invite -h" for options.

Environment:
  DISCORD_TOKEN           bot token; overrides the stored login
  DISCORD_SERVER          default server (id or name)
  DISCORD_MCP_CONFIG_DIR  where credentials.json lives
  DISCORD_DEBUG=1         log API calls to stderr
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	args := []string{}
	if len(os.Args) > 2 {
		args = os.Args[2:]
	}

	var err error
	switch cmd {
	case "serve":
		err = serve(ctx)
	case "login":
		err = login(ctx, args)
	case "invite":
		err = invite(ctx, args)
	case "status":
		err = status(ctx)
	case "logout":
		err = logout()
	case "version", "--version", "-v":
		fmt.Println(appVersion())
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func serve(ctx context.Context) error {
	// Quiet by default: MCP clients surface stderr as server logs.
	level := slog.LevelWarn
	if v := os.Getenv("DISCORD_DEBUG"); v != "" && v != "0" && v != "false" {
		level = slog.LevelDebug
	}
	// stdout carries the protocol; everything else goes to stderr.
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	s := server.New(server.Config{
		Version: appVersion(),
		Logger:  logger,
		Server:  server.EnvServer(),
	})
	return s.Run(ctx, &mcp.StdioTransport{})
}

func appVersion() string {
	if version != "" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}

func newFlagSet(name, help string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), help)
		fs.PrintDefaults()
	}
	return fs
}
