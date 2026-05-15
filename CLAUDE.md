# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## O que é este projeto

Sistema de monitoramento residencial via RTSP. Cada câmera configurada tem três processos ffmpeg rodando em paralelo: um grava chunks MP4 em disco, outro gera segmentos HLS para visualização ao vivo e um terceiro detecta movimento por diff de frames. O frontend React é embutido no binário Go via `go:embed`.

## Comandos principais

### Backend (Go)

```bash
go test ./...                                         # todos os testes
go test ./internal/server/... -run TestLogin          # teste específico
make build                                            # binário local com versão git injetada
make run                                              # sobe Docker dev com live reload (camera-dev)
make all                                              # cross-compila para linux-amd64/arm64/arm e windows-amd64
make linux-amd64                                      # binário específico em dist/
make rpi                                              # alias para linux-arm64 (Raspberry Pi 3/4/5 64-bit)
./camera init                                         # wizard interativo → gera camera.yaml no diretório atual
./camera init --output /etc/camera/camera.yaml        # wizard → grava no caminho especificado
./camera version                                      # imprime versão, commit e data do build
```

### CI e branch protection

Todo PR para `master` dispara `.github/workflows/ci.yml` com dois jobs paralelos:
- **Backend**: `go test ./...` + `go build ./...`
- **Frontend**: `yarn lint` + `yarn test --run` + `yarn build`

O branch `master` está protegido: push direto bloqueado, PR obrigatório, 1 aprovação humana obrigatória, checks `Backend` e `Frontend` obrigatórios. Para reaplicar as regras (ex: em novo repositório):

```bash
gh api repos/{owner}/{repo}/branches/master/protection \
  --method PUT \
  --header "Accept: application/vnd.github+json" \
  --input - <<'EOF'
{
  "required_status_checks": { "strict": true, "contexts": ["Backend", "Frontend"] },
  "enforce_admins": true,
  "required_pull_request_reviews": { "dismiss_stale_reviews": true, "required_approving_review_count": 1 },
  "restrictions": null
}
EOF
```

### Release

```bash
./scripts/release.sh             # calcula bump, gera changelog, cria tag anotada e faz push
./scripts/release.sh --dry-run   # prévia sem criar nada
```

O script lê os commits convencionais desde a última tag, determina o bump (`feat` → minor, breaking → major, resto → patch), gera o changelog agrupado por tipo e cria uma tag no formato `vX.Y.Z-beta.N`. O push da tag dispara o GitHub Actions que publica a release. Todas as releases são beta enquanto o projeto não atingir estabilidade.

### Frontend (`frontend/src/`)

SPA React/Vite/Tailwind. Páginas principais: `LoginPage` → `DashboardPage` → `CameraPage` / `StatsPage`. Seção de configurações em `/settings/*` com sidebar lateral (padrão GitHub Settings). Token JWT em `localStorage` (`auth.ts`). Em desenvolvimento, Vite faz proxy de `/api` e `/stream` para `localhost:8080`.

Rotas de settings por câmera: `/settings/cameras/:id` (detalhes) → `/settings/cameras/:id/motion` (detecção de movimento) → `/settings/cameras/:id/motion/zones` (zonas de exclusão — sub-página de motion).

Componentes reutilizáveis: `AppLayout` (layout base; exibe footer com versão, data do build, uptime e versão do Go via `GET /api/about`), `SettingsLayout` (sidebar + conteúdo para `/settings/*`), `SettingsSection` (card com lista de campos label/valor), `HLSPlayer` (player HLS com alerta de movimento ao vivo, seek ao evento e botões de mudo/fullscreen), `ListPanel`, `StatCard`, `MotionScoreChart` (gráfico SVG em tempo real dos scores brutos via SSE, escala logarítmica, janela de 30 s), `ConfirmDialog` (modal fixo de confirmação para ações destrutivas; prop `danger` alterna botão vermelho/azul).

`NotificationContext` — contexto global que abre um `EventSource` por câmera (via `GET /api/cameras/{id}/motion/live`) e acumula notificações de movimento em `localStorage` (máx. 100). Expõe: `notifications`, `unreadCount`, `markRead`, `markAllRead`, `markSelectedRead`, `markAllUnread`, `remove`, `removeAll`, `removeSelected`. O `Header` exibe o sino com badge, painel de notificações com seleção múltipla, kebab de ações (marcar lidas/não lidas, excluir) e `ConfirmDialog` para confirmações.

`CameraPage` — além das gravações e eventos, exibe abaixo do player uma barra de controles com: botão mudo (sincronizado com os controles nativos via `onVolumeChange`), seletor de velocidade 1×–32× (detecta limite do browser progressivamente e exibe aviso), toggle "Reprodução contínua" (avança automaticamente para a próxima gravação ou próximo evento ao terminar o clip atual). Usa refs (`continuousPlayRef`, `activeEventIdxRef`, `visibleEventsRef`, `recordingsRef`) para evitar closures stale nos handlers de vídeo. Cada evento de movimento com campo `frame` exibe um thumbnail 64×40px do snapshot JPEG; ao clicar abre modal com a imagem em tamanho completo. Ao receber um novo evento via SSE, a lista de eventos é re-fetched do servidor (em vez de adicionar o evento parcial), garantindo que o campo `frame` esteja presente.

Hooks customizados: `useEventSource(path, onMessage)` — abre um `EventSource` autenticado via `?token=` e chama `onMessage` a cada evento; `path = null` fecha sem abrir. `useScrollToPlayer(ref, key)` — faz scroll suave até o elemento quando `key` muda. `useStats(redirectTo)` — busca `/api/stats` com polling de 30 s. `useSettings(redirectTo)` / `useAbout(redirectTo)` — buscam `/api/settings` e `/api/about`.

```bash
cd frontend
yarn install
yarn build    # gera frontend/dist (necessário antes do go build)
yarn dev      # Vite dev server na porta 5173 (proxy /api e /stream para :8080)
```

## Arquitetura

### Binários

| Binário | Responsabilidade |
|---|---|
| `cmd/camera` | Servidor principal: grava, faz streaming HLS, detecta movimento e serve a SPA. Suporta o subcomando `camera init` — wizard interativo que gera o arquivo de bootstrap (`camera.yaml`) com porta, `db_path`, storage e credenciais do admin inicial. |
| `cmd/mcp-ffprobe` | Servidor MCP (stdio) que expõe `probe_stream` — executa ffprobe em uma URL RTSP e retorna os metadados JSON do stream. Útil para inspeção de câmeras via ferramentas MCP. |

### Fluxo de inicialização (`cmd/camera/main.go`)

1. Lê o arquivo de bootstrap (`camera.yaml`) com porta, `db_path`, storage e credenciais do admin.
2. Abre o banco SQLite e executa as migrations pendentes.
3. Na primeira execução, cria o usuário admin com `must_change_password = true`.
4. Lê câmeras do banco e para cada câmera habilitada inicia:
   - Um `recorder.Recorder` — grava RTSP → MP4 chunk
   - Um `streaming.HLSStreamer` — grava RTSP → segmentos HLS para live
   - Um `motion.Monitor` — detecta movimento via ffmpeg pipe raw (se motion habilitado)
5. O `server.Server` é levantado em goroutine separada e serve a SPA + API REST.
6. Câmeras adicionadas/removidas via API ativam callbacks `onCameraStart` / `onCameraStop` (em goroutine, para não bloquear o handler HTTP).

### Pacotes internos

| Pacote | Responsabilidade |
|---|---|
| `internal/exec` | Interfaces `Commander` / `Process` e implementação real com `os/exec`. Injetadas nos pacotes abaixo para permitir testes sem ffmpeg. |
| `internal/recorder` | Grava RTSP em chunks MP4. Armazena em `{storage}/{camera_id}/{YYYY/MM/DD}/{YYYYMMDDHHmmss}.mp4`. |
| `internal/streaming` | Gera playlist HLS ao vivo em `{segments_path}/{camera_id}/index.m3u8`. Modo padrão: janela de 5 segmentos de 2s. Modo DVR (quando `server.hls_dvr_seconds > 0`): mantém todos os segmentos da janela, adiciona `EXT-X-PROGRAM-DATE-TIME` para seek por timestamp. |
| `internal/motion` | Detecta movimento via ffmpeg pipe raw (frames grayscale em 1/4 da resolução). Expõe dois canais: `Events()` para eventos acima do limiar e `RawScores()` para o score bruto de cada frame diff. A cada evento persiste no banco (`motion_events`) e em `motion.ndjson`, e salva um JPEG anotado com bounding box e score. |
| `internal/storage` | `Cleaner` com retenção diferenciada por categoria. Quando o banco está disponível: `syncRecordings()` varre o filesystem e sincroniza MP4s para a tabela `recordings` (`INSERT OR IGNORE`), atualizando `has_motion` via join com `motion_events`; `cleanFromDB()` consulta `recordings` para decisões de exclusão. Sem banco: consulta `motion.ndjson` diretamente. |
| `internal/db` | Acesso ao SQLite (`modernc.org/sqlite`). Executa migrations em `internal/db/migrations/` na inicialização. Tabelas: `cameras`, `camera_recording_config`, `camera_motion_config`, `camera_motion_zones`, `users` (com `must_change_password`), `settings`, `recordings`, `motion_events`. |
| `internal/ffprobe` | Executa e parseia saída JSON do ffprobe para detectar codec, áudio e dimensões do stream. |
| `internal/server` | HTTP server com JWT HS256 (segredo gerado a cada boot, expira em 24h). O claim `must_change_password=true` bloqueia todos os endpoints exceto `POST /api/auth/change-password`. Serve API REST, arquivos de gravação (incluindo snapshots `_motion.jpg`), segmentos HLS e a SPA React. Endpoints SSE de movimento: `/api/cameras/{id}/motion/live` e `/api/cameras/{id}/motion/scores`. `GET /api/stats` usa `SUM(size_bytes)` da tabela `recordings` quando banco disponível. |
| `internal/config` | Lê o arquivo de bootstrap (`camera.yaml`) com porta, `db_path`, storage e credenciais do admin. Variáveis de ambiente sobrescrevem campos específicos (ver abaixo). |
| `internal/logger` | `stdout`: JSON em stdout. `file`: um arquivo por nível (`debug.log`, `info.log`, `warn.log`, `error.log`) no diretório configurado. |
| `frontend/` | SPA React/Vite/Tailwind embutida via `go:embed all:dist`. `ChangePasswordPage` — tela obrigatória no primeiro login; bloqueia acesso ao restante da UI enquanto `must_change_password=true` no JWT. |

### Autenticação

O JWT é assinado com um segredo aleatório gerado no boot — tokens não sobrevivem a reinicializações do servidor. O token é aceito via header `Authorization: Bearer <token>` ou query param `?token=<token>` (necessário para `<video src>` e `<HLSPlayer>`).

Fluxo de primeiro acesso: o admin inicial é criado com `must_change_password = true`. No primeiro login o servidor emite um token com esse claim; o frontend redireciona obrigatoriamente para `ChangePasswordPage`. Após a troca a flag é zerada no banco e o acesso normal é liberado. A senha do bootstrap não precisa ser atualizada — serve apenas na criação inicial.

### Build info

`version`, `commit` e `builtAt` são injetados via `-ldflags` no `Makefile`. Em `main.go` são passados ao servidor via `WithVersion(version)` e `WithBuildInfo(commit, builtAt)`. O endpoint `GET /api/about` expõe esses valores junto com `uptime_seconds` e `go_version`.

## Variáveis de ambiente

| Variável | Campo sobrescrito |
|---|---|
| `CAMERA_STORAGE_PATH` | `storage.path` |
| `CAMERA_TIMEZONE` | `timezone` (fuso da instalação; usado pelo servidor para interpretar datas locais) |
| `CAMERA_SERVER_JWT_SECRET` | `server.jwt_secret` (segredo JWT fixo; vazio = gerado aleatoriamente a cada boot) |

## Forma de trabalho

O desenvolvimento segue **XP (Extreme Programming)** com **TDD red → green → refactor**:

- O **navigator** (usuário) define a história, revisa o código e aprova cada etapa.
- O **driver** (Claude) implementa, sempre guiado pelos testes.

> ⚠️ **`master` é protegido.** Push direto é bloqueado pelo GitHub. Todo código entra via Pull Request — nunca commite ou force-push diretamente em `master`.

### Histórias

Histórias ficam em `stories/` (gitignored). O nome do arquivo usa timestamp no formato
`YYYYMMDDHHmm_<descricao>.md` — igual às migrations de banco — garantindo ordenação
cronológica natural ao listar o diretório.

Ao iniciar uma nova história:
- Criar o arquivo `stories/YYYYMMDDHHmm_<descricao>.md` com contexto, critérios de aceitação e notas técnicas.
- Ao concluir a implementação, adicionar uma seção `## Revisão` no arquivo com checklist do que foi feito.
- **Só proceder com PR após o navigator aprovar marcando `[x] Aprovado` na seção Revisão.**

### Fluxo por história

1. Criar `stories/YYYYMMDDHHmm_<descricao>.md` e abrir uma branch: `git checkout -b <tipo>/<descricao-curta>` a partir de `master`.
2. Escrever o teste que falha (**red**) — nunca escrever código de produção sem um teste falhando antes.
3. Implementar o mínimo para o teste passar (**green**).
4. Refatorar se necessário, mantendo os testes verdes (**refactor**).
5. Executar `yarn lint` e `yarn test` (frontend) ou `go test ./...` (backend).
6. Adicionar seção `## Revisão` na história e aguardar aprovação do navigator.
7. Commitar com mensagem semântica na branch, fazer `git push origin <branch>`, abrir PR para `master` com `gh pr create` e aguardar CI verde. O merge é feito pelo GitHub após aprovação — nunca localmente em `master`.

### Commits semânticos

Formato: `<tipo>(<escopo opcional>): <descrição curta em inglês>`

| Tipo | Quando usar |
|---|---|
| `feat` | nova funcionalidade |
| `fix` | correção de bug |
| `refactor` | refatoração sem mudança de comportamento |
| `test` | adição ou correção de testes |
| `docs` | documentação |
| `chore` | configuração, build, dependências |

## Manutenção contínua

- **Ao adicionar ou alterar qualquer funcionalidade**, revise este `CLAUDE.md` e atualize as seções afetadas.
- **Ao adicionar ou alterar qualquer campo de configuração**, atualize `camera.yaml.example` com o novo campo, valor de exemplo e comentário com a variável de ambiente correspondente (se houver).

## Convenções de teste

Testes usam `httptest.NewRecorder` (server), `fakeCommander` com `trackingProcess` (recorder/streamer) e implementações fake das interfaces de `internal/exec`. O banco SQLite é criado em memória (`:memory:`) nos testes de server e db — nenhum mock externo. Cada pacote é testado em isolamento via injeção de dependência.

## Diretrizes de Desenvolvimento Go

Sempre priorize a simplicidade e a legibilidade conforme os provérbios do Go ("Effective Go").

### 1. Princípio DRY e Abstração
- **Evite Abstração Precoce:** Siga a "Regra de Três". Não crie abstrações ou interfaces até que haja pelo menos três casos de uso concretos.
- **Cópia vs. Dependência:** Prefira duplicar uma pequena função utilitária do que introduzir uma dependência externa desnecessária.
- **Interfaces:** Defina interfaces no lado do consumidor (onde são usadas) e não no lado do produtor. Mantenha as interfaces pequenas (1 ou 2 métodos).

### 2. Estilo e Convenções de Código
- **Alinhamento do "Happy Path":** Mantenha o fluxo principal de sucesso alinhado à esquerda. Use *guard clauses* para tratar erros e retorne o mais cedo possível.
- **Nomenclatura:**
    - Variáveis de escopo curto: Curtas (ex: `ctx`, `w`, `r`, `i`).
    - Variáveis globais/longas: Descritivas.
    - Interfaces: Sufixo "-er" para interfaces de um único método (ex: `Formatter`, `Storer`).
- **Zero Value:** Projete structs para que o valor zero (`var s MyStruct`) seja útil e seguro para uso imediato.

### 3. Tratamento de Erros
- **Erros são Valores:** Sempre verifique erros explicitamente logo após a chamada: `if err != nil { return err }`.
- **Contexto de Erro:** Utilize `fmt.Errorf("contexto do erro: %w", err)` para adicionar contexto sem perder o erro original (wrapping).
- **Sem Panics:** Nunca use `panic` para controle de fluxo. Reserve-o apenas para erros catastróficos de inicialização ou bugs lógicos irrecuperáveis.

### 4. Concorrência e Performance
- **Canais vs. Mutex:** "Não comunique compartilhando memória; compartilhe memória comunicando". Use canais para orquestração e `sync.Mutex` para proteção de estado simples.
- **Goroutines:** Sempre saiba como uma goroutine vai terminar antes de iniciá-la para evitar vazamentos de memória.
- **Ponteiros:** Use ponteiros apenas quando precisar mutar o estado ou para evitar cópias de structs muito grandes (> 64-128 bytes).

### 5. Tooling Obrigatório
- Todo código gerado deve ser compatível com o `gofmt`.