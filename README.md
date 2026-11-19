# Hero Requests

Aplicativo **leve e gratuito** que roda no seu computador, lê o chat da **Twitch** e da **Kick** e deixa seus viewers pedirem músicas para você tocar no **Clone Hero** ou no **YARG**.

- Os viewers pedem pelo chat: `!pedir artista - música`
- Você controla a fila em um **painel local** (tocar, pular, remover, reordenar)
- Um **overlay para o OBS** mostra a música atual e a fila
- Um **site estático no GitHub Pages** mostra a lista de músicas e a fila para os viewers
- Um único executável (~10 MB), sem instalar nada, sem servidor, sem mensalidade

```
 Chat Twitch ─┐                          ┌─► Painel local   (localhost:7777)
              ├─► Hero Requests (seu PC) ┼─► Overlay do OBS (localhost:7777/overlay)
 Chat Kick  ──┘        ▲                 └─► GitHub (branch "data") ─► Site no GitHub Pages
                       │
        JSON de músicas do jogo
```

> **Status:** versão inicial (MVP completo). Veja [Limitações conhecidas](#limitações-conhecidas).

---

## Sumário

1. [Instalação](#1-instalação)
2. [Primeiros passos (tela inicial)](#2-primeiros-passos-tela-inicial)
3. [Comandos do chat](#3-comandos-do-chat)
4. [Painel de controle](#4-painel-de-controle)
5. [Overlay no OBS](#5-overlay-no-obs)
6. [Lista de músicas](#6-lista-de-músicas)
7. [Site público no GitHub Pages](#7-site-público-no-github-pages)
8. [Bot que responde no chat da Twitch (opcional)](#8-bot-que-responde-no-chat-da-twitch-opcional)
9. [Configurações](#9-configurações)
10. [Solução de problemas](#10-solução-de-problemas)
11. [Implementação (para desenvolvedores)](#11-implementação-para-desenvolvedores)
12. [Limitações conhecidas](#limitações-conhecidas)
13. [Licença](#licença)

---

## 1. Instalação

### Opção A: baixar o executável (recomendado)

1. Abra a página de [Releases](../../releases) deste repositório.
2. Baixe o `.zip` do seu sistema (`windows-amd64`, `linux-amd64`, `darwin-arm64` para Mac com chip Apple ou `darwin-amd64` para Mac Intel).
3. Extraia em uma pasta, por exemplo `C:\HeroRequests`.
4. Execute `hero-requests.exe` (no Windows, dê dois cliques). Uma janela de terminal abre e o navegador mostra o painel.

A pasta `data/` é criada ao lado do executável e guarda `config.json` (suas configurações) e `queue.json` (a fila). Feche a janela do terminal para encerrar o app.

### Opção B: compilar

Precisa do [Go](https://go.dev/dl/) 1.22 ou mais novo.

```bash
git clone https://github.com/SEU_USUARIO/hero-requests.git
cd hero-requests
go build -o hero-requests .
./hero-requests
```

Opções de linha de comando:

| Opção | Para que serve |
|---|---|
| `-port 7777` | Porta do painel (padrão 7777) |
| `-data pasta` | Pasta de dados (padrão `data`) |
| `-no-browser` | Não abre o navegador automaticamente |
| `-version` | Mostra a versão |

---

## 2. Primeiros passos (tela inicial)

Na primeira vez o app abre um **assistente de configuração** em 4 passos. Tudo pode ser mudado depois no botão **Configurações**.

1. **Seus chats:** informe o canal da Twitch e/ou da Kick (aceita `nome` ou a URL do canal). Marque só os que você usa.
2. **Lista de músicas:** escolha o arquivo JSON exportado pelo jogo (ou a pasta de músicas) e clique em **Testar leitura**. O app mostra quantas músicas encontrou.
3. **Regras:** máximo de pedidos por pessoa, tamanho da fila, bloquear música repetida e prioridade para subs/VIPs/moderadores.
4. **Site e overlay (opcional):** repositório e token do GitHub para publicar o site, número de músicas e cor do overlay.

No fim, o assistente mostra o endereço do overlay para colar no OBS. Para testar sem estar ao vivo, use **Simular pedido** no painel.

---

## 3. Comandos do chat

| Comando | O que faz |
|---|---|
| `!pedir artista - música` | Pede uma música. Também aceita só palavras soltas (`!pedir everlong`) ou `música - artista`. |
| `!pedir 1`, `!pedir 2`... | Quando o app acha várias músicas, ele lista até 3 opções; responda com o número (vale por 90 segundos). |
| `!fila` | Mostra os próximos pedidos. |
| `!atual` | Mostra a música tocando. |
| `!lista` | Mostra o link do site com a lista de músicas. |
| `!sair` | Remove o último pedido do próprio viewer. |

Apelidos: `!sr`, `!request`, `!song` (pedir), `!queue` (fila), `!now`/`!np` (atual), `!list`/`!songs`/`!musicas` (lista), `!cancelar`/`!remover` (sair).

**Regras aplicadas automaticamente:** pedidos fechados, limite por pessoa, fila cheia e música repetida. O **streamer** (broadcaster) ignora os limites. Com *prioridade* ligada, subs, VIPs e moderadores entram à frente da fila (depois do último pedido prioritário).

> Por padrão o app **só lê** o chat. Para ele responder ("Pedido aceito!"), veja [a seção 8](#8-bot-que-responde-no-chat-da-twitch-opcional).

---

## 4. Painel de controle

Abre em `http://127.0.0.1:7777`. Dá para deixar em um segundo monitor ou em uma doca personalizada do OBS (*Docas > Docas de navegador personalizadas*).

- **Tocando agora:** *Concluir e chamar próxima* move a música atual para o histórico e chama a primeira da fila.
- **Fila:** *Tocar* (escolhe qualquer pedido), subir/descer, remover, *Limpar fila*.
- **Adicionar manualmente:** busca na lista e coloca na fila (ignora limites).
- **Pedidos abertos/fechados:** um clique.
- **Chat ao vivo:** só os comandos, com o resultado (✓ aceito / ✗ recusado e o motivo).
- **Simular pedido:** testa o fluxo como se fosse um viewer.

A fila é salva em disco a cada mudança: se o app fechar, ela volta como estava.

---

## 5. Overlay no OBS

1. No OBS: **Fontes > + > Navegador**.
2. URL: `http://127.0.0.1:7777/overlay`
3. Largura `1920`, altura `1080`. O fundo é transparente.

Parâmetros opcionais na URL:

| Parâmetro | Exemplo | Efeito |
|---|---|---|
| `items` | `?items=8` | Quantas músicas da fila aparecem |
| `accent` | `?accent=%2300c2ff` | Cor de destaque (o `#` vira `%23`) |
| `pos` | `?pos=left` | Posição à esquerda (padrão: direita) |
| `hint` | `?hint=0` | Esconde a dica "Peça com !pedir" |

Combine com `&`, por exemplo `/overlay?items=8&pos=left`.

---

## 6. Lista de músicas

O app lê a lista que o próprio jogo exporta, então você não precisa gerar nada.

**Modo JSON (recomendado):** aponte para o arquivo `.json` de músicas. O leitor é tolerante. Aceita:

- uma lista de objetos: `[{"artist": "...", "name": "..."}, ...]`
- um objeto com a lista dentro de `songs`, `entries`, `tracks`, `items`, `data`...
- um objeto de objetos: `{"id1": {...}, "id2": {...}}`
- campos aninhados (por exemplo dentro de `metadata`)

Nomes de campo reconhecidos (sem diferenciar maiúsculas, `_` ou `-`):

| Campo | Aliases |
|---|---|
| Artista | `artist`, `artists`, `band` |
| Música | `name`, `title`, `song`, `songname`, `track` |
| Álbum | `album` |
| Gênero | `genre` |
| Charter | `charter`, `frets`, `author` |
| Ano | `year` |

Músicas repetidas (mesmo artista e título, com charters diferentes) aparecem uma vez só.

**Modo pasta:** aponte para a pasta de músicas e o app lê todos os `song.ini` dentro dela (formato do Clone Hero/YARG).

> Se o seu arquivo não for reconhecido, abra uma [issue](../../issues) anexando as primeiras linhas dele (sem dados pessoais) que o leitor é ajustado rápido. O código está em [`songs.go`](songs.go).

---

## 7. Site público no GitHub Pages

O site fica na pasta [`docs/`](docs) deste repositório e é publicado de graça pelo GitHub Pages. **Nenhum servidor seu fica exposto na internet.** O app envia a lista de músicas e a fila para uma branch chamada `data` do seu repositório, e o site lê esses arquivos.

```
App no seu PC ──(API do GitHub)──► branch "data": songs.json + queue.json
                                              │
Viewer abre https://usuario.github.io/repo ◄─┘  (site em docs/)
```

### Passo a passo

1. **Crie o repositório.** Faça *Fork* deste projeto (ou *Use this template*) na sua conta. Ele precisa ser **público** para o Pages gratuito.
2. **Ative o Pages:** *Settings > Pages > Build and deployment > Source: Deploy from a branch*, branch `main`, pasta `/docs`. O endereço será `https://SEU_USUARIO.github.io/NOME_DO_REPO/`.
3. **Crie um token:** em *Settings (da sua conta) > Developer settings > Personal access tokens > Fine-grained tokens > Generate new token*.
   - *Repository access:* **Only select repositories** e escolha o seu repositório.
   - *Permissions > Repository permissions > Contents:* **Read and write**.
   - Copie o token (`github_pat_...`).
4. **No app:** *Configurações > Site público*. Informe `SEU_USUARIO/NOME_DO_REPO` e cole o token. Marque *Publicar a fila automaticamente*.
5. Clique em **Publicar agora** no painel. O app cria a branch `data` (se não existir) e envia `songs.json` e `queue.json`.

### Como funciona

- A **lista de músicas** é publicada quando você muda a configuração de músicas ou clica em *Publicar agora*.
- A **fila** é publicada automaticamente ~12 segundos depois de cada mudança (juntando várias mudanças em uma só).
- O site consulta a fila **a cada 60 segundos** (para ficar dentro do limite gratuito da API do GitHub). Por isso a fila do site tem alguns segundos ou até 1 minuto de atraso. **Para tempo real use o overlay do OBS.**
- O token fica só no seu computador (`data/config.json`, permissão restrita) e é usado apenas para falar com a API do GitHub.

---

## 8. Bot que responde no chat da Twitch (opcional)

Sem isso, o app só **lê** o chat e os viewers não recebem confirmação (mas veem o resultado no overlay e no site). Para ele responder:

1. Use uma conta para o bot (pode ser a sua).
2. Gere um token OAuth com os escopos `chat:read` e `chat:edit`, por exemplo em [twitchtokengenerator.com](https://twitchtokengenerator.com) (ferramenta de terceiros; use por sua conta e risco) ou pelo [fluxo oficial da Twitch](https://dev.twitch.tv/docs/authentication/).
3. No app: *Configurações > Seus chats > Avançado*. Marque *O bot responde no chat da Twitch*, informe o usuário do bot e cole o token.

O app respeita o limite de mensagens da Twitch (uma resposta a cada 1,5 s). Trate o token como uma senha: não compartilhe nem publique.

**Kick:** a resposta no chat da Kick exige a API oficial com OAuth e ainda **não** está implementada. A leitura da Kick funciona normalmente.

---

## 9. Configurações

Todas as opções ficam em `data/config.json` e podem ser editadas pelo botão **Configurações** do painel.

| Seção | Opção | Descrição |
|---|---|---|
| Chats | Canal da Twitch / Kick | Nome ou URL do canal |
| Chats | Bot (avançado) | Usuário e token OAuth para responder na Twitch |
| Chats | ID do chat da Kick | Só se a consulta automática falhar |
| Chats | Chave/cluster do Pusher | Só se a Kick trocar (ver Solução de problemas) |
| Músicas | Modo e caminho | JSON ou pasta |
| Regras | Máx. por pessoa, tamanho da fila | Limites |
| Regras | Bloquear repetidas, prioridade | Comportamentos |
| Site | Repositório, token, auto-publicar, URL | Publicação no GitHub |
| Overlay | Itens e cor | Aparência |
| Avançado | Porta | Reinicie o app depois de mudar |

---

## 10. Solução de problemas

| Sintoma | O que fazer |
|---|---|
| **Twitch: erro de conexão** | Confira o nome do canal. O app tenta reconectar sozinho com espera crescente. Firewall precisa liberar saída na porta 443. |
| **Kick: "bloqueou a consulta automática"** | A Kick às vezes bloqueia a busca do ID do chat. Abra `https://kick.com/api/v2/channels/SEU_CANAL` no navegador, copie o valor de `chatroom` > `id` e cole em *ID do chat da Kick*. |
| **Kick: "App key ... not in this cluster"** | A Kick mudou a chave pública do chat (Pusher). Em *Avançado* informe a nova chave e o cluster (veja nas ferramentas de rede do navegador, em uma conexão WebSocket em `kick.com`, o trecho `ws-CLUSTER.pusher.com/app/CHAVE`). Se puder, abra uma issue com os valores novos. |
| **"o arquivo não é um JSON válido"** | Confirme que escolheu o JSON certo. Veja [Lista de músicas](#6-lista-de-músicas). |
| **Músicas não aparecem no site** | Clique em *Publicar agora* e veja o erro no painel. Confira o nome `usuario/repo`, as permissões do token (*Contents: read and write*) e se o Pages está ativo em `main` / `docs`. |
| **Erro 403/404 ao publicar** | Token sem permissão ou de outro repositório. |
| **Site: "limite de consultas do GitHub"** | Muitos acessos do mesmo IP. Espere alguns minutos. |
| **Porta em uso** | Rode com `-port 7780` (ou outra). |
| **Windows SmartScreen avisa** | O executável não é assinado. Clique em *Mais informações > Executar assim mesmo*, ou compile você mesmo. |

---

## 11. Implementação (para desenvolvedores)

Go (somente biblioteca padrão + `gorilla/websocket`), sem CGO. A interface web é HTML/JS puro, embutida no executável com `go:embed`, sem etapa de build.

```
.
├── main.go          # início, flags, servidor, abre o navegador
├── app.go           # estado, hub de eventos (SSE), orquestra chats e fila
├── config.go        # Config, validação/normalização, tokens nunca saem para o navegador
├── songs.go         # leitor de JSON/song.ini, normalização e busca com pontuação
├── queue.go         # fila: regras, prioridade, persistência em data/queue.json
├── commands.go      # !pedir, !fila... (independente de plataforma)
├── twitch.go        # IRC sobre WebSocket (wss://irc-ws.chat.twitch.tv)
├── kick.go          # chat da Kick via Pusher (WebSocket)
├── publish.go       # envia songs.json/queue.json para a branch "data" (API do GitHub)
├── server.go        # API HTTP, proteção contra acesso externo, arquivos embutidos
├── web/             # painel, assistente, overlay (embutidos)
├── docs/            # site estático (GitHub Pages)
└── .github/workflows/  # CI e release
```

**Fluxo de uma mensagem:** `twitch.go`/`kick.go` → canal `msgs` → `App.process` → `Handler.Handle` (`commands.go`) → `Queue` → `App.changed` → SSE para painel e overlay + publicação debounced para o GitHub.

**API local** (usada pelo painel; só aceita `127.0.0.1`/`localhost` e bloqueia `Origin` externo):

| Método | Rota | Função |
|---|---|---|
| GET | `/api/events` | Estado completo por Server-Sent Events |
| GET | `/api/state` | Estado atual |
| GET/PUT | `/api/config` | Lê/salva configuração (tokens mascarados) |
| POST | `/api/queue/{next,done,play,remove,move,clear,add}` | Controle da fila |
| POST | `/api/open` | Abre/fecha pedidos |
| GET | `/api/songs/search?q=` | Busca na biblioteca |
| POST | `/api/songs/{test,reload}` | Testa/recarrega a lista |
| POST | `/api/simulate` | Simula uma mensagem de chat |
| POST | `/api/publish` | Publica no GitHub agora |

**Segurança:** o servidor só escuta em `127.0.0.1`; requisições com `Host` ou `Origin` que não sejam locais são recusadas (evita que uma página aberta no navegador mande comandos ao app). Nomes vindos do chat são sempre escapados antes de entrar no HTML. `config.json` é gravado com permissão `0600`.

**Desenvolvimento:**

```bash
go vet ./...
go test ./...            # busca, fila, comandos, IRC, Kick, config e publicação (com GitHub simulado)
go run . -no-browser     # roda local
```

**Release:** crie uma tag e o GitHub Actions compila para Windows, Linux e macOS e anexa os zips ao Release.

```bash
git tag v0.1.0
git push origin v0.1.0
```

**Contribuindo:** issues e PRs são bem-vindos. Ideias: resposta no chat da Kick (OAuth), votos para furar a fila, histórico/ranking, leitura automática da música atual do jogo, tradução da interface.

---

## Limitações conhecidas

- A fila do **site** tem atraso (publicação a cada mudança + consulta a cada 60 s). O overlay é em tempo real.
- O pedido é feito **somente pelo chat**; o site só mostra a lista e copia o comando.
- **Kick:** a leitura depende da chave pública do Pusher, que a Kick pode trocar (configurável em *Avançado*), e não há resposta no chat da Kick ainda.
- O leitor de músicas foi feito para ser tolerante, mas ainda não foi validado com todos os formatos de exportação do Clone Hero e do YARG. Se o seu não funcionar, abra uma issue.
- A interface está em português do Brasil.

## Licença

[MIT](LICENSE). Este projeto não é afiliado à Twitch, à Kick, ao Clone Hero ou ao YARG.
