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
make run                                              # sobe Docker dev (camera-dev)
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

### Instalação em servidor Linux

```bash
# Instalar (detecta arch, baixa release, cria serviço systemd)
curl -fsSL https://raw.githubusercontent.com/jacksonbicalho/camera/master/scripts/install.sh | sudo bash

# Caminhos customizáveis (todos opcionais; os defaults estão abaixo)
curl -fsSL https://raw.githubusercontent.com/jacksonbicalho/camera/master/scripts/install.sh \
  | sudo bash -s -- \
      --install-dir /usr/local/bin \
      --config-dir  /etc/camera \
      --data-dir    /data/recordings \
      --service-name camera

# Desinstalar (não requer internet — usa o desinstalador instalado localmente)
sudo camera-uninstall                        # mantém config e dados
sudo camera-uninstall --remove-config        # remove também /etc/camera/
sudo camera-uninstall --remove-data          # remove também /data/recordings/
sudo camera-uninstall --remove-config --remove-data
```

O script (`scripts/install.sh`) é POSIX sh, detecta a arquitetura (`amd64`, `arm64`, `arm`), baixa o binário da última release e cria um serviço systemd em `/etc/systemd/system/camera.service`. Ao final da instalação, copia a si mesmo para `/usr/local/share/camera/install.sh`, salva os caminhos usados em `/var/lib/camera/install.conf` e cria o comando `camera-uninstall` em `/usr/local/bin`. Config gerado em `/etc/camera/camera.yaml` — **não sobrescrito** se já existir.

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

### Docker

```bash
docker compose --profile development up camera-dev --build   # dev: yarn build + go run
docker compose --profile production up camera --build        # produção: binário estático
```

## Arquitetura

### Binários

| Binário | Responsabilidade |
|---|---|
| `cmd/camera` | Servidor principal: grava, faz streaming HLS, detecta movimento e serve a SPA. Suporta o subcomando `camera init` — wizard interativo que gera `camera.yaml` no diretório atual. |
| `cmd/mcp-ffprobe` | Servidor MCP (stdio) que expõe `probe_stream` — executa ffprobe em uma URL RTSP e retorna os metadados JSON do stream. Útil para inspeção de câmeras via ferramentas MCP. |

### Fluxo de inicialização (`cmd/camera/main.go`)

Para cada câmera configurada, `main` cria e inicia:
1. Um `recorder.Recorder` — grava RTSP → MP4 chunk
2. Um `streaming.HLSStreamer` — grava RTSP → segmentos HLS para live
3. Um `motion.Monitor` — detecta movimento via ffmpeg pipe raw (se `motion.enabled: true`)

O `server.Server` é levantado em goroutine separada e serve a SPA + API REST.

### Pacotes internos

| Pacote | Responsabilidade |
|---|---|
| `internal/exec` | Interfaces `Commander` / `Process` e implementação real com `os/exec`. Injetadas nos pacotes abaixo para permitir testes sem ffmpeg. |
| `internal/recorder` | Grava RTSP em chunks MP4. Armazena em `{storage}/{camera_id}/{YYYY/MM/DD}/{YYYYMMDDHHmmss}.mp4`. |
| `internal/streaming` | Gera playlist HLS ao vivo em `{segments_path}/{camera_id}/index.m3u8`. Modo padrão: janela de 5 segmentos de 2s. Modo DVR (quando `server.hls_dvr_seconds > 0`): mantém todos os segmentos da janela, adiciona `EXT-X-PROGRAM-DATE-TIME` para seek por timestamp. |
| `internal/motion` | Detecta movimento via ffmpeg pipe raw (frames grayscale em 1/4 da resolução). Expõe dois canais: `Events()` para eventos acima do limiar (gravados em `{storage}/{camera_id}/motion.ndjson`) e `RawScores()` para o score bruto de cada frame diff (usado na visualização em tempo real). Cooldown configurável (`motion.cooldown_seconds`) suprime eventos consecutivos dentro da janela. A cada evento, salva um JPEG anotado (`{YYYYMMDDHHmmss}_motion.jpg`) com: retângulo laranja de 2px ao redor da região de maior diferença (`computeBBox`) e o score no canto superior direito, também em laranja (`annotateFrame`). A entrada no `motion.ndjson` inclui os campos `frame` (nome do JPEG) e `bbox` (`{x,y,w,h}` normalizados). |
| `internal/storage` | `Cleaner` que apaga MP4s com retenção diferenciada por categoria (com/sem movimento) e monitora uso vs `max_size_gb`. Cada MP4 é classificado via `HasMotionInRange` que consulta o `motion.ndjson` do mesmo diretório. O timestamp de início do chunk é extraído do nome do arquivo (`YYYYMMDDHHmmss`). |
| `internal/ffprobe` | Executa e parseia saída JSON do ffprobe para detectar codec, áudio e dimensões do stream. |
| `internal/server` | HTTP server com JWT HS256 (segredo gerado a cada boot, expira em 24h). Serve API REST, arquivos de gravação (incluindo snapshots `_motion.jpg`), segmentos HLS e a SPA React. Inclui dois endpoints SSE de movimento: `/api/cameras/{id}/motion/live` (eventos acima do limiar) e `/api/cameras/{id}/motion/scores` (score bruto por frame). Endpoints de configuração: `GET /api/settings` (configuração completa sanitizada, autenticado) e `GET /api/about` (versão, commit, uptime, versão do Go, autenticado). |
| `internal/config` | Lê `camera.yaml`; variáveis de ambiente sobrescrevem campos específicos (ver abaixo). |
| `internal/logger` | `stdout`: JSON em stdout. `file`: um arquivo por nível (`debug.log`, `info.log`, `warn.log`, `error.log`) no diretório configurado. |
| `frontend/` | SPA React/Vite/Tailwind embutida via `go:embed all:dist`. |

### Autenticação

O JWT é assinado com um segredo aleatório gerado no boot — tokens não sobrevivem a reinicializações do servidor. O token é aceito via header `Authorization: Bearer <token>` ou query param `?token=<token>` (necessário para `<video src>` e `<HLSPlayer>`).

### Build info

`version`, `commit` e `builtAt` são injetados via `-ldflags` no `Makefile`. Em `main.go` são passados ao servidor via `WithVersion(version)` e `WithBuildInfo(commit, builtAt)`. O endpoint `GET /api/about` expõe esses valores junto com `uptime_seconds` e `go_version`.

## Variáveis de ambiente

| Variável | Campo sobrescrito |
|---|---|
| `CAMERA_STORAGE_PATH` | `storage.path` |
| `CAMERA_TIMEZONE` | `timezone` (fuso da instalação; usado pelo servidor para interpretar datas locais) |

## Forma de trabalho

O desenvolvimento segue **XP (Extreme Programming)** com **TDD red → green → refactor**:

- O **navigator** (usuário) define a história, revisa o código e aprova cada etapa.
- O **driver** (Claude) implementa, sempre guiado pelos testes.

### Histórias

Histórias ficam em `stories/` (gitignored). Ao iniciar uma nova história:
- Criar o arquivo `stories/STORY_<descricao>.md` com contexto, critérios de aceitação e notas técnicas.
- Ao concluir a implementação, adicionar uma seção `## Revisão` no arquivo com checklist do que foi feito.
- **Só proceder com commit e merge após o navigator aprovar marcando `[x] Aprovado` na seção Revisão.**

### Fluxo por história

1. Criar `stories/STORY_<descricao>.md` e abrir uma branch: `git checkout -b <tipo>/<descricao-curta>` a partir de `master`.
2. Escrever o teste que falha (**red**) — nunca escrever código de produção sem um teste falhando antes.
3. Implementar o mínimo para o teste passar (**green**).
4. Refatorar se necessário, mantendo os testes verdes (**refactor**).
5. Executar `yarn lint` e `yarn test` (frontend) ou `go test ./...` (backend).
6. Adicionar seção `## Revisão` na história e aguardar aprovação do navigator.
7. Commitar com mensagem semântica e mergear em `master` com `--no-ff`.

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

Testes usam `httptest.NewRecorder` (server), `fakeCommander` com `trackingProcess` (recorder/streamer) e implementações fake das interfaces de `internal/exec`. Não há banco de dados nem mocks externos — cada pacote é testado em isolamento via injeção de dependência.

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