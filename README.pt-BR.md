# discord-mcp

Servidor MCP para gerenciar servidores do Discord, escrito em Go. É um binário único, sem Python ou Node embutido, e funciona em qualquer cliente MCP via stdio: Claude Code, Codex, OpenCode, Gemini CLI, Cursor, Claude Desktop e outros.

Com ele você pede ao seu agente para montar e organizar um servidor: criar categorias e canais, deixar canais privados, criar cargos com cores e permissões, ajustar as permissões de cada canal por cargo ou membro, reordenar tudo e conferir quem pode fazer o quê.

> Projeto não oficial, sem afiliação com a Discord Inc. Ele usa a API oficial do Discord através de um bot seu.

[English version](README.md)

## Instalação

Escolha um caminho. Nenhum deles exige Go.

**Com Node.js (npx), sem instalar nada.** O primeiro uso baixa o binário certo para o seu sistema:

```bash
npx -y @gabriellsales/discord-mcp login
```

**Sem Node, com o script de instalação.** Baixa o binário do [GitHub Releases](https://github.com/gabriellsalesx/Discordbot-mcp/releases), confere o checksum e instala em `~/.local/bin` (Linux e macOS) ou em `%LOCALAPPDATA%\Programs\discord-mcp` (Windows):

```bash
curl -fsSL https://raw.githubusercontent.com/gabriellsalesx/Discordbot-mcp/main/install.sh | sh
```

```powershell
irm https://raw.githubusercontent.com/gabriellsalesx/Discordbot-mcp/main/install.ps1 | iex
```

Depois, `discord-mcp login`.

**Outras formas:**

```bash
npm install -g @gabriellsales/discord-mcp                  # ou: pnpm add -g @gabriellsales/discord-mcp
go install github.com/gabriellsalesx/Discordbot-mcp@latest    # com Go 1.26+
```

## Login

O Discord só permite que ferramentas ajam no servidor através de um bot. Usar o token da sua conta pessoal viola os termos do Discord e pode banir a conta, então o `discord-mcp` não aceita esse caminho. Criar o bot leva um minuto, e o `login` guia cada passo:

```bash
npx -y @gabriellsales/discord-mcp login
```

1. O Developer Portal abre no navegador. Clique em **New Application**, dê um nome (por exemplo "Gerente do Servidor") e crie.
2. Na aba **Bot**, clique em **Reset Token** e depois em **Copy**.
3. Cole o token no terminal (a digitação fica oculta). O `login` confere o token e salva.
4. O link de convite abre sozinho. Escolha o servidor, clique em **Authorize** e o terminal avisa quando o bot entrar.

Opcional: na mesma aba **Bot**, ligue **Server Members Intent** para buscar membros pelo nome (`find_members` e `set_member_roles` com nome). Sem isso, essas ferramentas funcionam passando o id do usuário.

Depois que o bot entrar, vá em **Configurações do servidor > Cargos** e arraste o cargo do bot para cima dos cargos que ele vai gerenciar. O Discord só deixa um bot editar cargos abaixo do cargo mais alto dele.

O token fica em `~/.config/discord-mcp/credentials.json` (Linux), com permissão `600`. Outros comandos:

```bash
discord-mcp invite          # adiciona o bot a outro servidor
discord-mcp invite --admin  # convite pedindo Administrador
discord-mcp status          # bot, servidores e se o Server Members Intent está ligado
discord-mcp logout          # apaga o token salvo
```

Em CI ou contêiner, pule o login e passe o token por variável: `DISCORD_TOKEN=...`. Também dá para mandar o token por pipe: `pass show discord-bot | discord-mcp login`.

### Que permissões o bot pede

Por padrão o convite pede todas as permissões, exceto Administrador. Parece muito, mas é uma regra do Discord: um bot só consegue liberar ou negar num canal, ou dar a um cargo, as permissões que ele mesmo tem. Se você prefere, `discord-mcp invite --admin` pede só Administrador. As duas formas dão o mesmo resultado prático; a diferença é que, com a lista, você pode desmarcar o que não quiser na tela de autorização.

## Configuração nos clientes

Os exemplos usam `npx`, que funciona sem mexer no `PATH`. Se você instalou pelo script, pelo `npm -g` ou pelo `go install`, troque `npx -y @gabriellsales/discord-mcp` por `discord-mcp`. A variável `DISCORD_SERVER` é opcional: sem ela, vale o servidor escolhido no login, ou o único em que o bot estiver.

**Claude Code**

```bash
claude mcp add -s user discord -- npx -y @gabriellsales/discord-mcp
```

**Codex CLI** (`~/.codex/config.toml`)

```toml
[mcp_servers.discord]
command = "npx"
args = ["-y", "@gabriellsales/discord-mcp"]
# Pede aprovação para tudo que não é somente leitura.
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

**Cursor** (`~/.cursor/mcp.json`) e **Claude Desktop** (`claude_desktop_config.json`)

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

No Windows, se o cliente não encontrar o `npx`, use `"command": "cmd"` e `"args": ["/c", "npx", "-y", "@gabriellsales/discord-mcp"]`.

Reinicie o cliente e peça algo como: *"crie uma categoria Projetos com os canais anúncios (só leitura para membros), chat e uma sala de voz, e um canal privado staff visível só para Moderador"*.

### Comandos

| Comando | O que faz |
| --- | --- |
| `discord-mcp` | Roda o servidor MCP via stdio (é o que o cliente chama). |
| `discord-mcp login` | Conecta o bot com o passo a passo e abre o convite. `--server` define o servidor padrão; `--admin` pede Administrador no convite; `--no-invite` não abre o convite. |
| `discord-mcp invite` | Abre o convite para outro servidor e espera o bot entrar. `--admin` pede Administrador; `--print` só mostra o link. |
| `discord-mcp status` | Mostra o bot, os servidores e o Server Members Intent. |
| `discord-mcp logout` | Apaga o token salvo neste computador. |
| `discord-mcp version` | Mostra a versão. |

## Ferramentas

| Ferramenta | O que faz |
| --- | --- |
| `list_servers` | Servidores em que o bot está, com membros e o que ele pode gerenciar. |
| `get_server` | Configurações do servidor, contagens e o cargo e as permissões do bot. |
| `update_server` | Nome, descrição, nível de verificação, notificações, filtro de conteúdo, canais de sistema, regras e AFK, idioma. |
| `list_channels` | Canais como na barra lateral: categorias em ordem, com tipo, tópico, se é privado e se está sincronizado com a categoria. |
| `get_channel` | Um canal com todas as permissões por cargo ou membro. |
| `create_channel` | Cria canal de texto, voz, anúncios, palco, fórum ou mídia, ou uma categoria. Pode nascer dentro de uma categoria e privado para cargos escolhidos. |
| `update_channel` | Renomeia, muda tópico, modo lento, NSFW, voz e posição, move de categoria ou sincroniza com ela. |
| `delete_channel` | Apaga canal ou categoria. Exige confirmação. |
| `organize_channels` | Reordena canais ou categorias de uma vez e move canais para uma categoria. |
| `list_roles` | Cargos do topo para baixo, com cor, membros, permissões e se o bot consegue gerenciar. |
| `create_role` | Cria cargo com nome, cor, permissões e exibição separada. |
| `update_role` | Altera cargo: nome, cor, permissões (substituir, somar ou tirar), exibição e posição. Também vale para `@everyone`. |
| `delete_role` | Apaga um cargo. Exige confirmação. |
| `set_channel_permissions` | Libera, nega ou volta ao padrão permissões de um cargo ou membro num canal ou categoria. Numa categoria, atualiza junto os canais sincronizados. |
| `check_permissions` | Calcula o que um cargo ou membro realmente pode fazer, no servidor ou num canal. |
| `invite_bot` | Gera o link para adicionar o bot a outro servidor (com o servidor já escolhido, se passar o id). |
| `find_members` | Busca membros pelo começo do nome, com os cargos. |
| `set_member_roles` | Dá ou tira cargos de um membro. |
| `send_message` | Posta uma mensagem como o bot, por exemplo as regras num canal novo. Exige confirmação. |
| `save_layout` | Salva a estrutura de um servidor como layout reutilizável. |
| `list_layouts` | Lista os layouts salvos. |
| `get_layout` | Mostra um layout inteiro. |
| `apply_layout` | Monta um layout salvo em outro servidor. Exige confirmação. |
| `delete_layout` | Apaga um layout salvo. |

Servidores, canais, cargos e membros podem ser passados pelo nome (`#geral`, `@Moderador`, `everyone`) ou pelo id. Quando dois canais têm o mesmo nome, use `categoria/canal`. Toda ferramenta que altera algo aceita `reason`, que vai para o registro de auditoria do servidor.

Permissões usam os nomes da API em qualquer formato: `view_channel`, `Send Messages`, `manage-roles`. Também valem os nomes do app, como `manage_server` e `timeout_members`.

### Layouts: monte uma vez, replique em outros servidores

Peça ao agente para montar a estrutura que você quer (cargos, categorias, canais, quem vê o quê). Quando gostar do resultado, peça para salvar: *"salve esse servidor como layout Comunidade padrão"*. Depois, em qualquer outro servidor onde o bot esteja: *"aplique o layout Comunidade padrão no servidor X"*.

O layout guarda:
- os cargos, com nome, cor, permissões, ordem na hierarquia e as opções de exibição;
- as permissões gerais de `@everyone`;
- as categorias e os canais em ordem, com tipo, tópico, modo lento, NSFW e limite de voz;
- as permissões de cada canal por cargo, e quais canais seguem a categoria.

Mensagens, membros e permissões dadas a pessoas específicas não entram no layout.

O `apply_layout` primeiro mostra o plano: o que será criado e o que já existe. Ele só age com a sua confirmação. Cargos e canais que já existem com o mesmo nome são aproveitados como estão, e o `apply_layout` nunca apaga nem altera nada. Assim, aplicar de novo num servidor onde faltam só algumas partes completa apenas o que falta. As permissões gerais de `@everyone` só são trocadas com `apply_everyone_permissions: true`. Se o bot não tiver alguma permissão para conceder, ela é pulada e aparece como aviso.

Os layouts ficam em `~/.config/discord-mcp/layouts/`, um arquivo JSON por layout. Dá para copiar, versionar no git ou mandar para outra pessoa.

### Confirmação antes de apagar ou publicar

`delete_channel`, `delete_role`, `send_message` e `apply_layout` não agem na primeira chamada. Sem `confirm: true`, devolvem uma prévia (o que será apagado, quantos membros perdem o cargo, o texto da mensagem) e nada muda. O agente mostra a prévia, você aprova e ele chama de novo com `confirm: true`. As ferramentas também trazem as anotações MCP (`readOnlyHint`, `destructiveHint`), que clientes como Codex e Claude Code usam para pedir permissão.

Mensagens não notificam ninguém por padrão: `@everyone` e menções aparecem no texto, mas só disparam aviso com `allow_mentions: true`.

## Variáveis de ambiente

| Variável | Uso |
| --- | --- |
| `DISCORD_TOKEN` | Token do bot; tem prioridade sobre o login salvo. `DISCORD_BOT_TOKEN` também vale. |
| `DISCORD_SERVER` | Servidor padrão (id ou nome). `DISCORD_GUILD_ID` também vale. |
| `DISCORD_MCP_CONFIG_DIR` | Pasta do `credentials.json`. |
| `DISCORD_DEBUG=1` | Loga as chamadas à API no stderr (sem o token). |

## Limitações conhecidas

- O bot só gerencia cargos abaixo do cargo mais alto dele e só concede permissões que ele mesmo tem. Os erros do Discord vêm com essa dica.
- Buscar membros pelo nome exige o Server Members Intent ligado no Developer Portal.
- A partir de 16/11/2026, o Discord deixa de listar para o bot os canais que ele não consegue ver. Canais privados criados pelo `create_channel` já incluem o bot; para os outros, dê ao cargo do bot a permissão de ver o canal (ou use o convite com Administrador).
- Tópicos de fórum, threads, eventos, emojis e moderação (banir, expulsar, castigo) ainda não fazem parte das ferramentas.

## Desenvolvimento

```bash
go test ./...
go build -o discord-mcp .
```

`internal/discord` é o cliente REST (API v10), com os limites de taxa por bucket e as permissões; `internal/server` registra as ferramentas MCP com o [SDK oficial em Go](https://github.com/modelcontextprotocol/go-sdk); `internal/auth` guarda o token.

### Publicar uma versão

```bash
git tag v0.1.0 && git push origin v0.1.0
```

O workflow `release` compila os binários com o GoReleaser, cria o Release no GitHub com `checksums.txt` e publica no npm o pacote principal mais um pacote por plataforma (`npm/build.mjs` e `npm/publish.mjs`). O CI roda testes em Linux, macOS e Windows, `golangci-lint` e `govulncheck`. Para testar localmente: `node npm/build.mjs 0.1.0 && node npm/publish.mjs --dry-run`.

## Autor

Criado e mantido por **Gabriell Sales**.

- GitHub: [@gabriellsalesx](https://github.com/gabriellsalesx)
- Substack: [GS Lab](https://gabriellsales.substack.com)

Contribuições são bem-vindas: abra uma issue ou um pull request.

## Licença

MIT. Veja [LICENSE](LICENSE).
