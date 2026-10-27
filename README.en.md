# discord-mcp

An MCP server for managing Discord servers, written in Go. It is a single binary, with no Python or Node runtime inside, and works in any MCP client over stdio: Claude Code, Codex, OpenCode, Gemini CLI, Cursor, Claude Desktop and others.

Ask your agent to build and tidy up a server: create categories and channels, make channels private, create roles with colors and permissions, tune each channel's permissions per role or member, reorder everything, and check who can do what.

> Unofficial project, not affiliated with Discord Inc. It uses the official Discord API through a bot you own.

[Versão em português](README.md)

## Install

Pick one. None of them needs Go.

**With Node.js (npx), nothing to install.** The first run downloads the right binary for your system:

```bash
npx -y @gabriellsales/discord-mcp login
```

**Without Node, with the install script.** Downloads the binary from [GitHub Releases](https://github.com/gabriellsalesx/discord-mcp/releases), verifies the checksum and installs to `~/.local/bin` (Linux and macOS) or `%LOCALAPPDATA%\Programs\discord-mcp` (Windows):

```bash
curl -fsSL https://raw.githubusercontent.com/gabriellsalesx/discord-mcp/main/install.sh | sh
```

```powershell
irm https://raw.githubusercontent.com/gabriellsalesx/discord-mcp/main/install.ps1 | iex
```

Then run `discord-mcp login`.

**Other ways:**

```bash
npm install -g @gabriellsales/discord-mcp                  # or: pnpm add -g @gabriellsales/discord-mcp
go install github.com/gabriellsalesx/discord-mcp@latest    # with Go 1.26+
```

## Login

Discord only lets tools act on a server through a bot. Using your personal account token breaks Discord's terms and can get the account banned, so `discord-mcp` refuses it. Creating the bot takes a minute and `login` walks you through it:

```bash
npx -y @gabriellsales/discord-mcp login
```

1. The Developer Portal opens in your browser. Click **New Application**, name it (for example "Server Manager") and create it.
2. On the **Bot** tab, click **Reset Token**, then **Copy**.
3. Paste the token in the terminal (input is hidden). `login` checks it and saves it.
4. The invite link opens by itself. Pick the server, click **Authorize**, and the terminal tells you when the bot has joined.

Optional: on the same **Bot** tab, turn on **Server Members Intent** to look members up by name (`find_members`, and `set_member_roles` with a name). Without it, those tools work with user ids.

Once the bot is in, open **Server Settings > Roles** and drag the bot's role above the roles it should manage. Discord only lets a bot edit roles below its own highest role.

The token is stored in `~/.config/discord-mcp/credentials.json` (Linux) with mode `600`. Other commands:

```bash
discord-mcp invite          # add the bot to another server
discord-mcp invite --admin  # invite asking for Administrator
discord-mcp status          # bot, servers, and whether Server Members Intent is on
discord-mcp logout          # delete the stored token
```

In CI or containers, skip login and pass the token in the environment: `DISCORD_TOKEN=...`. You can also pipe it: `pass show discord-bot | discord-mcp login`.

### Which permissions the bot asks for

By default the invite asks for every permission except Administrator. That looks like a lot, but it is a Discord rule: a bot can only allow or deny in a channel, or grant to a role, the permissions it has itself. If you prefer, `discord-mcp invite --admin` asks for Administrator alone. Both work the same in practice; with the list you can untick what you do not want on the authorization screen.

## Client setup

The examples use `npx`, which works without touching `PATH`. If you installed with the script, `npm -g` or `go install`, replace `npx -y @gabriellsales/discord-mcp` with `discord-mcp`. `DISCORD_SERVER` is optional: without it, the server chosen at login is used, or the only one the bot is in.

**Claude Code**

```bash
claude mcp add -s user discord -- npx -y @gabriellsales/discord-mcp
```

**Codex CLI** (`~/.codex/config.toml`)

```toml
[mcp_servers.discord]
command = "npx"
args = ["-y", "@gabriellsales/discord-mcp"]
# Ask for approval on anything that is not read-only.
default_tools_approval_mode = "writes"
```

**OpenCode** (`opencode.json`)

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "discord": { "type": "local", "command": ["npx", "-y", "@gabriellsales/discord-mcp"], "enabled": true }
  }
}
```

**Gemini CLI**

```bash
gemini mcp add -s user discord npx -y @gabriellsales/discord-mcp
```

**Cursor** (`~/.cursor/mcp.json`) and **Claude Desktop** (`claude_desktop_config.json`)

```json
{
  "mcpServers": {
    "discord": {
      "command": "npx",
      "args": ["-y", "@gabriellsales/discord-mcp"]
    }
  }
}
```

On Windows, if the client cannot find `npx`, use `"command": "cmd"` and `"args": ["/c", "npx", "-y", "@gabriellsales/discord-mcp"]`.

Restart the client and try: *"create a Projects category with an announcements channel (read-only for members), a chat channel and a voice room, plus a private staff channel only Moderators can see"*.

### Commands

| Command | What it does |
| --- | --- |
| `discord-mcp` | Runs the MCP server over stdio (what clients call). |
| `discord-mcp login` | Connects the bot step by step and opens the invite. `--server` sets the default server; `--admin` asks for Administrator in the invite; `--no-invite` skips it. |
| `discord-mcp invite` | Opens the invite for another server and waits for the bot to join. `--admin` asks for Administrator; `--print` only prints the link. |
| `discord-mcp status` | Shows the bot, its servers and the Server Members Intent. |
| `discord-mcp logout` | Deletes the token stored on this computer. |
| `discord-mcp version` | Prints the version. |

## Tools

| Tool | What it does |
| --- | --- |
| `list_servers` | Servers the bot is in, with member counts and what it may manage. |
| `get_server` | Server settings, counts, and the bot's role and permissions. |
| `update_server` | Name, description, verification level, notifications, content filter, system/rules/AFK channels, locale. |
| `list_channels` | Channels as the sidebar shows them: categories in order, with type, topic, private flag and whether each is synced with its category. |
| `get_channel` | One channel with every permission override, per role or member. |
| `create_channel` | Creates a text, voice, announcement, stage, forum or media channel, or a category. Can start inside a category and private to chosen roles. |
| `update_channel` | Rename, topic, slowmode, NSFW, voice settings and position; move to a category or sync with it. |
| `delete_channel` | Deletes a channel or category. Needs confirmation. |
| `organize_channels` | Reorders channels or categories in one go and moves channels into a category. |
| `list_roles` | Roles from the top, with color, members, permissions and whether the bot can manage each. |
| `create_role` | Creates a role with name, color, permissions and hoisting. |
| `update_role` | Changes a role: name, color, permissions (replace, add or remove), display and position. Works on `@everyone` too. |
| `delete_role` | Deletes a role. Needs confirmation. |
| `set_channel_permissions` | Allows, denies or resets permissions for a role or member in a channel or category. On a category, synced channels follow. |
| `check_permissions` | Works out what a role or member can really do, server-wide or in a channel. |
| `invite_bot` | Returns the link that adds the bot to another server (preselected when you pass its id). |
| `find_members` | Finds members by the start of their name, with roles. |
| `set_member_roles` | Gives or takes roles from a member. |
| `send_message` | Posts a message as the bot, such as rules in a new channel. Needs confirmation. |
| `save_layout` | Saves a server's structure as a reusable layout. |
| `list_layouts` | Lists the saved layouts. |
| `get_layout` | Shows a layout in full. |
| `apply_layout` | Builds a saved layout in another server. Needs confirmation. |
| `delete_layout` | Deletes a saved layout. |

Servers, channels, roles and members can be given by name (`#general`, `@Moderator`, `everyone`) or id. When two channels share a name, use `category/channel`. Every tool that changes something takes an optional `reason`, written to the server's audit log.

Permissions use the API names in any style: `view_channel`, `Send Messages`, `manage-roles`. The app's names such as `manage_server` and `timeout_members` work too.

### Layouts: build once, replicate everywhere

Ask the agent to build the structure you want (roles, categories, channels, who sees what). When you like it, ask to save it: *"save this server as layout Community base"*. Later, in any other server the bot is in: *"apply layout Community base to server X"*.

A layout keeps:
- roles, with name, color, permissions, hierarchy order and display options;
- the server-wide permissions of `@everyone`;
- categories and channels in order, with type, topic, slowmode, NSFW and voice limit;
- each channel's permissions per role, and which channels follow their category.

Messages, members and permissions given to specific people are not saved.

`apply_layout` first shows the plan: what will be created and what already exists. It only acts after you confirm. Roles and channels that already exist with the same name are reused as they are, and `apply_layout` never deletes or changes anything. So applying again to a server that is missing only some parts fills in just what is missing. `@everyone`'s server-wide permissions are only replaced with `apply_everyone_permissions: true`. Permissions the bot cannot grant are skipped and listed as warnings.

Layouts live in `~/.config/discord-mcp/layouts/`, one JSON file each. Copy them, keep them in git or share them.

### Confirmation before deleting or posting

`delete_channel`, `delete_role`, `send_message` and `apply_layout` do nothing on the first call. Without `confirm: true` they return a preview (what will be deleted, how many members lose the role, the message text). The agent shows it, you approve, and it calls again with `confirm: true`. Tools also carry MCP annotations (`readOnlyHint`, `destructiveHint`) that clients such as Codex and Claude Code use to ask for permission.

Messages notify nobody by default: `@everyone` and mentions render, but only ping with `allow_mentions: true`.

## Environment variables

| Variable | Use |
| --- | --- |
| `DISCORD_TOKEN` | Bot token; overrides the stored login. `DISCORD_BOT_TOKEN` works too. |
| `DISCORD_SERVER` | Default server (id or name). `DISCORD_GUILD_ID` works too. |
| `DISCORD_MCP_CONFIG_DIR` | Folder for `credentials.json`. |
| `DISCORD_DEBUG=1` | Logs API calls to stderr (never the token). |

## Known limitations

- The bot can only manage roles below its highest role and grant permissions it has. Discord errors come back with that hint.
- Looking members up by name needs the Server Members Intent turned on in the Developer Portal.
- From November 16, 2026, Discord stops listing channels the bot cannot see. Private channels made with `create_channel` already include the bot; for others, give the bot's role View Channel there (or invite it with Administrator).
- Forum posts, threads, events, emojis and moderation (ban, kick, timeout) are not covered yet.

## Development

```bash
go test ./...
go build -o discord-mcp .
```

`internal/discord` is the REST client (API v10) with per-bucket rate limiting and the permission flags; `internal/server` registers the MCP tools with the [official Go SDK](https://github.com/modelcontextprotocol/go-sdk); `internal/auth` stores the token.

### Releasing

```bash
git tag v0.1.0 && git push origin v0.1.0
```

The `release` workflow builds the binaries with GoReleaser, creates the GitHub Release with `checksums.txt`, and publishes the main npm package plus one package per platform (`npm/build.mjs` and `npm/publish.mjs`). CI runs tests on Linux, macOS and Windows, `golangci-lint` and `govulncheck`. To try locally: `node npm/build.mjs 0.1.0 && node npm/publish.mjs --dry-run`.

## Author

Created and maintained by **Gabriell Sales**.

- GitHub: [@gabriellsalesx](https://github.com/gabriellsalesx)
- Substack: [GS Lab](https://gabriellsales.substack.com)

Contributions are welcome: open an issue or a pull request.

## License

MIT. See [LICENSE](LICENSE).
