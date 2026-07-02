# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## O que é este projeto

Sistema de monitoramento residencial via RTSP. Cada câmera configurada tem três processos ffmpeg rodando em paralelo: um grava chunks MP4 em disco, outro gera segmentos HLS para visualização ao vivo e um terceiro detecta movimento por diff de frames. O frontend React é embutido no binário Go via `go:embed`.

## Fluxo de trabalho

O fluxo completo de **desenvolvimento e publicação** — XP/TDD, estratégia de branches, CI/branch protection, ciclo por história (com os gates), slash commands, hooks, scripts de workflow e planejamento/corte de release — vive em **[`docs/workflow.md`](docs/workflow.md)**. **Leia esse arquivo antes de trabalhar** (o hook `session-start` lembra a cada sessão).

**Regras-gate inegociáveis (resumo — detalhe em `docs/workflow.md`):**
- `master` e `develop` são protegidos — nunca commit/push direto; tudo via PR.
- Toda história começa por `/story` (story file + branch a partir de `develop`); nada de código/teste antes. A story é **sempre preenchida** (Contexto + Solução investigados, **nunca em branco**) antes de pedir a revisão — escopo/ambiguidade se resolve no plano.
- **Gate de revisão:** não implemente antes de `[x] História revisada` na story. **Após `[x] História revisada` a ÚNICA interação com o navigator é o pedido de aprovação** — o driver vai até o fim sem perguntar nada e **sem confirmar nenhum comando/execução**.
- **Gate de aprovação:** nenhum commit antes de `[x] Aprovado`; o driver **não** marca os Critérios de Aceitação (só o navigator, via `scripts/story-approval.sh`).
- Após aprovação: `scripts/commit.sh` → `scripts/push-pr.sh` (push + PR + CI + merge). Story file e branch só são removidos quando a história fica `[✓]` no release file. **Corte de release:** `scripts/release-pr.sh` (via `/release-pr`) abre o PR `develop → master` (só com ok explícito do navigator).

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

### Frontend (`frontend/src/`)

SPA React/Vite/Tailwind. Páginas principais: `LoginPage` → `DashboardPage` → `CameraPage` / `StatsPage`. Seção de configurações em `/settings/*` com sidebar lateral (padrão GitHub Settings). Token JWT em `localStorage` (`auth.ts`). Em desenvolvimento, Vite faz proxy de `/api` e `/stream` para `localhost:8080`.

**Todo componente/elemento da UI deve ter um `id` único e estável.** Botões, painéis, itens de navegação, abas, o ponteiro da timeline etc. recebem um `id` descritivo (ex: `sidebar-settings`, `events-panel`, `timeline-pointer`, `theme-mode-dark`). Facilita testes, automação e referência inequívoca em revisões — ao criar ou alterar um elemento, garanta o `id`.

**Design tokens e tema (`src/index.css` → `src/styles/`).** Base estrutural organizada como **tema → modo → valores**, dividida em arquivos importados pelo `index.css` (que é só `@import`, nesta ordem: `tailwindcss` → `styles/primitives.css` → `styles/themes/default.css` → `styles/base.css`): **`primitives.css`** = `@theme` com tipografia + rampas cruas (gray/accents) base; **`themes/default.css`** = o tema default, com os papéis semânticos no `@theme` (modo dark) e o bloco `[data-mode="light"]` (override do claro + inversão da rampa + tints + logo) — o cabeçalho do arquivo documenta **como adicionar um tema** (`themes/<nome>.css` com `[data-theme="<nome>"]` e `[data-theme="<nome>"][data-mode="light"]`, + `ThemeContext` aplicando `data-theme`); **`base.css`** = keyframes, `body` e cursor. Os tokens vivem em blocos `@theme` do Tailwind v4 (geram utilitários):
- **Tipografia**: escala `text-display/h1/h2/h3/h4/body/caption` (size + line-height) → use os utilitários (`text-h2`, `text-h4`…) em vez de tamanhos soltos (`text-lg`/`text-xs`).
- **Cor semântica**: papéis `bg-background`, `bg-surface`, `bg-surface-2`, `text-foreground`, `text-muted`, `text-faint`, `border-border`, `bg-primary`/`text-on-primary`, `bg-danger/success/warning`. Os valores no `@theme` são o **modo dark** (padrão); `[data-mode="light"]` sobrescreve **só** os tokens semânticos que mudam (accents seguem vívidos). A migração das cores cruas (`bg-gray-900`/`text-white`) para esses papéis é **incremental** — boa parte do app ainda usa as classes Tailwind diretas, com o bloco legado `[data-mode="light"]` remapeando a rampa de cinzas.

**Color mode ≠ tema.** `dark`/`light`/`system` são **modos de cor**; o **tema** é a identidade (paleta + tipografia). `ThemeContext` (`contexts/ThemeContext.tsx`) expõe `mode`/`setMode` (modo) e `theme` (hoje só `'default'`); aplica `data-mode` (resolvido dark/light) no `<html>`. A preferência persistida (`users.theme`) guarda o **modo**; quando houver um 2º tema, entra um campo de tema. **Adicionar um tema** no futuro = novo conjunto de valores de tokens (sem refatorar componentes).

Rotas de settings por câmera seguem o padrão **seção antes do id** (`/settings/cameras/<seção>/:id`): `/settings/cameras/:id` (detalhes), `/settings/cameras/edit/:id` (edição — a URL é a fonte de verdade do modo de edição; `editing` é derivado da rota, não de estado, pois navegar para a URL de edição não remonta o componente), `/settings/cameras/motion/:id` (detecção de movimento), `/settings/cameras/zones/:id` (zonas de exclusão), `/settings/cameras/analysis/:id` (análise), `/settings/cameras/states/:id` (state classification — CRUD de classificadores: crop no `BboxCanvas` sobre o snapshot, classes, gatilho por intervalo, limiar). As rotas de API do backend mantêm o id antes do recurso (`/api/cameras/:id/motion/zones` etc.) — só as rotas do frontend usam seção-antes-do-id.

**`SidebarContext`** (`contexts/SidebarContext.tsx`) — contexto dividido em dois para evitar loop de re-render: `SidebarItemsContext` (leitura, usado apenas em `Sidebar`) e `SetSidebarItemsContext` (setter estável, nunca causa re-render em quem chama). O `SidebarItemsProvider` fica na raiz do `App.tsx`, fora de qualquer layout, para que `CameraPage` consiga registrar itens antes de montar o layout. `useSidebarItems()` lê os itens; `useSetSidebarItems()` retorna o setter. Nunca usar `useSidebarItems()` em componentes que também chamam `setItems()` — isso cria ciclo de re-render.

Componentes reutilizáveis: `AppLayout` (layout base; exibe footer com versão, data do build, uptime e versão do Go via `GET /api/about`, e o `FooterStates` — rodapé ao vivo dos classificadores: o hook `useFooterStates` (`hooks/useFooterStates.ts`) faz poll de `GET /api/me/footer-states` (~5 s) e lista `nome: estado` para cada classificador que o usuário marcou pra ver; quando um estado muda entre polls **pisca** ~1 s (keyframe `footer-state-flash`); sem itens não renderiza nada), `SettingsLayout` (sidebar + conteúdo para `/settings/*`; prop `wide` usa `max-w-7xl` em vez do padrão `max-w-4xl` — usado na página de zonas para canvas maior; a sidebar (`SettingsSidebar`) inclui um item **Estatísticas** → `/stats`, entre Aparência e Sobre, espelhando o link do `Sidebar` principal), `SettingsSection` (card com lista de campos label/valor), `HLSPlayer` (player de live — **WebRTC-first com fallback automático pro HLS**: tenta o transporte WebRTC de baixa latência via `lib/webrtc.negotiateWebRTC` (`POST /api/cameras/{id}/webrtc`) e, quando indisponível (`409`/câmera não-H.264) ou a negociação/conexão falha, cai pro `hls.js` inalterado — inclui um watchdog de conexão; a prop `transport` (o `live_transport` da câmera: `auto`/`webrtc`/`hls`) força HLS quando `hls` (não tenta WebRTC); o nome `HLSPlayer` é legado, hoje é o player de live genérico; alerta de movimento ao vivo, seek ao evento e botões de mudo/fullscreen), `ListPanel`, `MotionScoreChart` (gráfico SVG em tempo real dos scores brutos via SSE, escala logarítmica, janela de 30 s), `ConfirmDialog` (modal fixo de confirmação para ações destrutivas; prop `danger` alterna botão vermelho/azul), `CameraConfigMenu` (botão no header do player da câmera — `CameraPage`, só admin — que abre um dropdown com as seções de config da câmera: Câmera/detail, Editar, Movimento, Zonas, Análise, Estados, cada uma navegando à rota; fecha ao selecionar, clicar fora e no Esc; props `showIcon`/`showLabel` acompanham o `playerBtn`).

`NotificationContext` — contexto global que abre um `EventSource` por câmera (via `GET /api/cameras/{id}/motion/live`) e acumula notificações de movimento em `localStorage` (máx. 100). Expõe: `notifications`, `unreadCount`, `markRead`, `markAllRead`, `markSelectedRead`, `markAllUnread`, `remove`, `removeAll`, `removeSelected`. O `Header` exibe o sino com badge, painel de notificações com seleção múltipla, kebab de ações (marcar lidas/não lidas, excluir) e `ConfirmDialog` para confirmações.

`CameraPage` — além das gravações e eventos, exibe abaixo do player uma barra de controles com: botão mudo (sincronizado com os controles nativos via `onVolumeChange`), seletor de velocidade 1×–32× (detecta limite do browser progressivamente e exibe aviso), toggle "Reprodução contínua" (avança automaticamente para a próxima gravação ou próximo evento ao terminar o clip atual). Usa refs (`continuousPlayRef`, `activeEventIdxRef`, `visibleEventsRef`, `recordingsRef`) para evitar closures stale nos handlers de vídeo. Cada evento de movimento com campo `frame` exibe um thumbnail 64×40px do snapshot JPEG; ao clicar abre modal com a imagem em tamanho completo. Ao receber um novo evento via SSE, a lista de eventos é re-fetched do servidor (em vez de adicionar o evento parcial), garantindo que o campo `frame` esteja presente. Layout da página: player → painel lateral condicional (gravações/eventos/calendário) → `VerticalTimeline` sempre visível à direita. Clicar em Gravações ou Eventos enquanto ao vivo auto-sai do modo live e inicia reprodução; clicar em Ao vivo fecha o painel lateral ativo. Quando `recording_enabled=false`: botão "Gravações" some da sidebar (condição: `cam?.recording_enabled !== false && recordings.length > 0`); painel de gravações exibe mensagem "Gravação desabilitada". Poll de gravações e eventos: intervalo de 5 s para o dia atual, 30 s para datas passadas; ambos (`loadRecordingsData` + `loadMotionEvents`) são atualizados juntos a cada tick para manter os painéis sincronizados após exclusão pelo cleaner. **Atalhos de teclado da régua (durante reprodução de gravação):** `Ctrl+↑/↓` navega entre gravações (chunk anterior/próximo), `Ctrl+Shift+↑/↓` avança/retrocede 1 segundo, `Ctrl+←/→` avança/retrocede **um frame** (vídeo pausado; duração do frame estimada via `requestVideoFrameCallback`, fallback 1/30s, `frameStepTime` em `cameraUtils.ts`). Os atalhos não agem em campo de texto nem em modo live. Ver `docs/live.md`.

`Sidebar` — seção inferior fixa com ícone de configurações (engrenagem), sino de notificações e usuário. Ícone ativo usa `bg-blue-600`. Itens injetados pela página corrente via `SidebarItemsProvider` ficam na seção superior.

`ChangePasswordPage` — quando `mustChangePassword() = true` (primeiro login), exibe formulário sem layout de app; quando o usuário já está logado e navega manualmente para `/change-password`, exibe dentro do `AppLayout` normal.

`CamerasSettingsPage` (`/settings/cameras`) — lista câmeras com thumbnail 80×48 px do snapshot (`GET /api/cameras/{id}/snapshot`). Nome da câmera em `font-medium` (sem `font-mono`), sem badge de ID. Badges compactos abaixo do nome: "motion" para câmeras com detecção ativa, "rec off" para câmeras com `recording_enabled=false`. Ações (editar/remover) como ícones SVG que aparecem via `group-hover:opacity-100` — sem texto. No modo admin, suporta drag-and-drop para reordenar: handle de arrastar (⠿) à esquerda de cada linha, destaque azul no alvo, atualização optimista do estado local e chamada a `PUT /api/settings/cameras/reorder` ao soltar.

Hooks customizados: `useEventSource(path, onMessage)` — abre um `EventSource` autenticado via `?token=` e chama `onMessage` a cada evento; `path = null` fecha sem abrir. `useScrollToPlayer(ref, key)` — faz scroll suave até o elemento quando `key` muda. `useStats(redirectTo)` — busca `/api/stats` com polling de 30 s. `useSettings(redirectTo)` / `useAbout(redirectTo)` — buscam `/api/settings` e `/api/about`.

```bash
make frontend # builda o frontend via Docker (node:20-alpine) — gera frontend/dist
cd frontend
yarn install  # apenas para desenvolvimento local do frontend
yarn dev      # Vite dev server na porta 5173 (proxy /api e /stream para :8080)
```

**Node/yarn não estão instalados no sistema host** — os testes e builds do frontend rodam via Docker. Para rodar os testes do frontend localmente use:

```bash
# Testes + lint + build do frontend via Docker (equivalente ao CI)
docker run --rm \
  --user "$(id -u):$(id -g)" \
  -v "$(pwd)/frontend":/app \
  -v camera-yarn-cache:/yarn-cache \
  -w /app \
  -e YARN_CACHE_FOLDER=/yarn-cache \
  -e HOME=/tmp \
  node:20-alpine \
  sh -c "yarn install --frozen-lockfile && yarn lint && yarn test --run && yarn build"
```

Nunca afirmar que os testes do frontend passaram sem ter rodado o comando acima (ou visto o CI verde).

### Serviço YOLO (`services/yolo/`)

Microserviço Python/FastAPI opcional para análise de gravações e fine-tuning. Expõe:
- `POST /analyze` — inferência YOLO em arquivo MP4
- `POST /finetune` / `GET /finetune/status/{id}` / `DELETE /finetune/{id}` — treino assíncrono (detecção)
- **State classification** (`yolov8n-cls`): `POST /classify` (imagem/crop → `{predictions:[{label,prob}], top}`), `POST /classify/train` (treina a partir de samples `{image_path,label}`, dataset em **pastas por classe**, assíncrono — status pelo mesmo `GET /finetune/status/{id}`; guard de tamanho barra `l`/`x`; treina com **`fliplr=0.0`** — o flip horizontal corromperia classes direcionais, ex.: pessoa entrando vs saindo), `GET /classify/models`.

**Testes do serviço:** `services/yolo/test_main.py` (pytest). As deps pesadas (torch/ultralytics/cv2) são **stubadas via `sys.modules`** antes de importar `main`, então os testes rodam numa imagem Python slim sem GPU. Rodam via `scripts/yolo-check.sh` (Docker), acionado pelo `scripts/check.sh` quando `services/yolo/` muda, e por um job dedicado no CI (`.github/workflows/ci.yml`).

**Subir o serviço:**

```bash
# CPU (qualquer hardware, incluindo Raspberry Pi)
docker compose --profile yolo up -d

# GPU NVIDIA (requer nvidia-container-toolkit no host)
docker compose -f docker-compose.yml -f docker-compose.nvidia.yml --profile yolo up -d
```

O padrão de **override files** mantém `docker-compose.yml` universal (funciona em RPi, AMD, CPU-only) e `docker-compose.nvidia.yml` adiciona apenas o device reservation NVIDIA. Nunca colocar configuração de GPU no `docker-compose.yml` base.

Modelos pré-baixados na imagem: `yolov8n` e `yolo11n`. Com GPU RTX 3050 (4GB VRAM): fine-tuning viável para variantes `n` e `s`; variantes `l` e `x` causam OOM no treino (inferência funciona). Ver `docs/analysis.md` para documentação completa.

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
   - Um `recorder.Recorder` — grava RTSP → MP4 chunk (somente se `recording_enabled=true`)
   - Um `streaming.HLSStreamer` — grava RTSP → segmentos HLS para live
   - Um `motion.Monitor` — detecta movimento via ffmpeg pipe raw (se motion habilitado)
5. O `server.Server` é levantado em goroutine separada e serve a SPA + API REST.
6. Câmeras adicionadas/removidas via API ativam callbacks `onCameraStart` / `onCameraStop` (em goroutine, para não bloquear o handler HTTP).

### Pacotes internos

| Pacote | Responsabilidade |
|---|---|
| `internal/exec` | Interfaces `Commander` / `Process` e implementação real com `os/exec`. Injetadas nos pacotes abaixo para permitir testes sem ffmpeg. |
| `internal/recorder` | Grava RTSP em chunks MP4. Armazena em `{storage}/{camera_id}/{YYYY/MM/DD}/{YYYYMMDDHHmmss}.mp4`. O ffmpeg congela o diretório do dia no padrão de saída ao iniciar (o `-strftime` só expande o nome do arquivo, não o diretório), então o `Run()` reinicia a sessão na virada da meia-noite UTC (`DurationUntilNextDay`) — sem isso, os chunks do novo dia continuariam caindo na pasta do dia em que o processo subiu. |
| `internal/streaming` | Gera playlist HLS ao vivo em `{segments_path}/{camera_id}/index.m3u8`. Modo padrão: janela de 5 segmentos de 2s. Modo DVR (quando `camera.hls_dvr_seconds > 0`): mantém todos os segmentos da janela DVR, adiciona `EXT-X-PROGRAM-DATE-TIME` para seek por timestamp. `hls_dvr_seconds`, `hls_segment_seconds` e `hls_list_size` são configurados por câmera via UI. |
| `internal/live` | Entrega o ao-vivo de **baixa latência via WebRTC** (sub-segundo), alternativa ao HLS (que tem piso de ~5–6s). Um `Publisher` por câmera puxa o RTSP **H.264** (`RTSPSource` sobre `gortsplib`, transporte TCP) e faz **repackage de RTP** — sem transcode, então CPU quase zero — para uma única `TrackLocalStaticRTP` (`pion/webrtc`) compartilhada, encaminhando cada pacote a todas as `PeerConnection` conectadas. A `Source` é uma interface injetável (fake nos testes → sem câmera/rede). `Negotiate(offer)` faz o handshake WHEP-style (offer→answer, ICE gather completo, sem trickle) e registra a sessão, que é fechada automaticamente ao desconectar/`ctx` cancelar. **Só H.264** (browser não toca H.265 via WebRTC) — câmeras não-H.264 não geram publisher e o front cai pro HLS. O `main.go` sobe um `Publisher` por câmera H.264 no `startCameraProcs` (sempre-ligado, paralelo ao HLS) e o registra no server via `WithLivePublisher`. O gating de quais câmeras publicam é `live.ShouldPublish(codec, transport)` — só H.264 **e** `live_transport != "hls"`; a preferência por câmera (`cameras.live_transport`: `auto`/`webrtc`/`hls`, default `auto`, acessor `CameraConfig.EffectiveLiveTransport()`) permite forçar HLS (não gera publisher → front cai pro HLS). **Fora de escopo (por ora):** áudio, transcode H.265→H.264, TURN e start-on-demand. |
| `internal/motion` | Detecta movimento via ffmpeg pipe raw. O pipe entrega frames **RGB full-res** (`format=rgb24`); cada frame é reduzido a cinza na resolução de diff (`downscaleRGBToGray`, default 1/4) em memória para o cálculo do diff/bbox, enquanto o frame full-res original é mantido para o snapshot. Expõe dois canais: `Events()` para eventos acima do limiar e `RawScores()` para o score bruto de cada frame diff. A cada evento persiste no banco (`motion_events`) e salva um JPEG anotado (`_motion.jpg`) com bounding box e score: o snapshot é o **próprio frame que disparou** (anotado in-place por `annotateRGBFrame`, full-res colorido), então o box sempre coincide com o sujeito — **não há mais grab RTSP assíncrono** (que capturava segundos depois e desalinhava o box). **`motion.ndjson` não é mais escrito** — todos os eventos vão direto para o banco; o arquivo é lido apenas como fallback legado. A URL RTSP que o motion decodifica é `cam.EffectiveMotionURL()` — o campo opcional `cameras.motion_rtsp_url` quando preenchido (ex.: apontar para o substream `subtype=1` corta o custo de decode em ~6–9×), senão a `rtsp_url` principal (recorder/HLS usam sempre a principal). Quando a URL de motion difere da principal, o `main.go` faz um `ffprobe.Resolve` dela para o `Monitor` usar as dimensões reais no pipe/snapshot; **trade-off**: o JPEG do evento sai na resolução do stream de motion (menor se for substream). O form de câmera tem um botão **"Detectar"** ao lado do campo que chama `POST /api/settings/cameras/detect-substream` (admin): o handler deriva candidatos por convenção (`substreamCandidates` — Dahua/Intelbras: troca `subtype=0`→`subtype=1`), roda `ffprobe.Resolve` em cada um e devolve `{motion_rtsp_url, width, height}` do primeiro que probar (ou vazio, sem erro, se nenhum) — o campo continua editável para câmeras fora do padrão. |
| `internal/storage` | `Cleaner` com retenção diferenciada por categoria e suporte a drives S3. Quando o banco está disponível: `syncRecordings()` varre o filesystem e sincroniza MP4s para a tabela `recordings` (`INSERT OR IGNORE`), atualizando `has_motion` via join com `motion_events`; `cleanFromDB()` consulta `recordings` para decisões de exclusão — inclui `AND ended_at IS NOT NULL` para nunca processar o arquivo em gravação. Ação configurável por categoria (`delete` ou `send_to_drive`): `uploadAndPurge()` faz upload S3, apaga o MP4 e chama `purgeMotionAssets()`. `purgeMotionAssets()` busca `frame_path` no banco, apaga os JPEGs do disco e deleta as linhas de `motion_events`. `slugify()` converte o nome da câmera para prefixo do objeto S3, transliterando acentos (`ã→a`, `ç→c`, etc.) via mapa estático. Sem banco: consulta `motion.ndjson` diretamente. |
| `internal/db` | Acesso ao SQLite (`modernc.org/sqlite`). Executa migrations em `internal/db/migrations/` na inicialização. Tabelas: `cameras`, `camera_recording_config`, `camera_motion_config`, `camera_motion_zones`, `users` (com `must_change_password`), `settings`, `recordings`, `motion_events`, `drives`, `retention_config`, `user_notifications` (notificações persistidas por usuário; FK `users` ON DELETE CASCADE), `camera_device_info` (metadados de hardware capturados pelo `internal/deviceinfo`, **EAV**: `(camera_id, key, value, collected_at)`, PK `(camera_id, key)`; cada captura é um snapshot completo — `SaveDeviceInfo` faz delete+insert numa tx, removendo chaves que sumiram; FK `cameras` ON DELETE CASCADE), `camera_state_classifiers`/`camera_state_classes`/`camera_state_history` (state classification — config do classificador por câmera, suas classes ≥2, e o histórico de estado; tipos em `internal/stateclass`, accessors em `state_classifiers.go`, todas FK ON DELETE CASCADE; o motor de inferência vive em `internal/stateclass` (`Tracker`: confirma após N leituras iguais ≥ threshold) + `internal/stateengine` (`Runner`: grab→`/classify`→tracker→`RecordStateTransition`+emit; `SnapshotGrabber` croppa a região do snapshot e grava sob `storage/tmp` — mesmo path que o YOLO lê em `/data`). `camera_state_classifiers` tem `notify_enabled` (default 1) e `footer_enabled` (default 0) — gate por classificador. O `main.go` sobe um `Runner` por classificador habilitado com intervalo>0 quando o serviço YOLO está configurado; o `emit` na transição chama `Server.PublishClassifierState` → `notifyStateTransition` que **só notifica quando `notify_enabled`** e apenas os usuários destinatários do canal notify (∩ acesso à câmera; admin sempre tem). A aba "Estados" mostra o estado atual de cada classificador em poll de `GET .../state`). `user_permissions` (**KV genérico de permissões/preferências por usuário**: `(user_id, key, value)`, PK `(user_id, key)`, FK `users` ON DELETE CASCADE; chaves `state_notify:{cid}`/`state_footer:{cid}` marcam destinatários por canal de um classificador — sem FK pro classificador, então o delete dele limpa as chaves na mão; `setChannelRecipients`/`loadChannelRecipients` em `state_classifiers.go`). Coluna `users.theme` (preferência de tema da UI, default `dark`). **Migrations: nunca use `;` em comentários** — `splitSQL` divide naïvemente em `;` e quebra. |
| `internal/ffprobe` | Executa e parseia saída JSON do ffprobe para detectar codec, áudio e dimensões do stream. |
| `internal/deviceinfo` | Captura metadados de hardware/manutenção da câmera logo após o cadastro. Saída é um **`map[string]string` plano de chaves namespaced** (`model`, `serial`, `firmware`, `mac`, `ntp.enabled`, `timezone`, `stream.main.gop`, `stream.sub.*`, e no futuro `capability.zoom`, `url.config`, `ptz.*`) + o dump bruto completo sob `raw.*` — EAV, então modelos diferentes guardam o que expõem sem mudar schema. Extensível por família via interface `Collector` (`Name`/`Detect`/`Collect`) — o "tipo". Hoje só o collector `dahua` (CGI Intelbras/Dahua, cliente HTTP digest em `cgi.go`); a função `Collect` escolhe o primeiro collector que dá `Detect`, marca `collector=<nome>` e mescla as chaves `stream.main.*` do `ffprobe` por cima (preenche só as ausentes; fallback `collector=generic` quando nenhum casa). |
| `internal/server` | HTTP server com JWT HS256 (segredo gerado a cada boot, expira em 24h). O claim `must_change_password=true` bloqueia todos os endpoints exceto `POST /api/auth/change-password`. Serve API REST, arquivos de gravação (incluindo snapshots `_motion.jpg`), segmentos HLS e a SPA React. Endpoints SSE de movimento: `/api/cameras/{id}/motion/live`, `/api/cameras/{id}/motion/scores` e `/api/cameras/{id}/motion/region-score` (score por zona/região, usado pelo canvas de zonas). **Sinalização WebRTC do ao-vivo:** `POST /api/cameras/{id}/webrtc` (`requireCameraAccess`) — corpo `{sdp}` (offer), resposta `{sdp}` (answer), delega ao `livePublisher` (`internal/live`) registrado via `WithLivePublisher`; sem publisher (câmera não-H.264 ou `live_transport=hls`) responde `409` para o front cair pro HLS. **Preferência de transporte por câmera:** o campo `live_transport` (`auto`/`webrtc`/`hls`, normalizado por `normalizeLiveTransport`) entra no create/update e volta no `GET /api/settings`. **Detecção pro cadastro:** `POST /api/settings/cameras/detect-streams` (admin) proba a URL principal e devolve `{codec,width,height,recommended}` — `recommended="webrtc"` sse H.264, senão `"hls"` (probe falho → `{}`). `GET /api/stats` usa `SUM(size_bytes)` da tabela `recordings` quando banco disponível. `GET /api/cameras/{id}/content-days?kind=recordings|events|all` (`requireCameraAccess`, default `all`) devolve `{"days":[...]}` — datas locais distintas (no `s.timezone`) com conteúdo do tipo `kind` (`db.ContentDays`, sobre `recordings`/`motion_events`); os calendários só habilitam esses dias. `GET /api/content-days?cameras=<ids>&kind=` (`requireFullAuth`) é o agregado multi-câmera (`db.ContentDaysMulti`, resolve as câmeras acessíveis como `handleGlobalRecordings` + filtro `cameras=`), usado pela `RecordingsPage`. **Frontend**: helpers puros `calendarContent`/`dateKey` em `lib/calendar.ts`; o componente `DatePicker` aceita `availableDays?: string[]` (desabilita dias sem conteúdo + `startMonth`/`endMonth`, preservando `disableFuture`) — usado por RecordingsPage (`kind=all`), ReportsPage e picker de Estados (`kind=events`). A `CameraPage` usa os mesmos helpers direto no `<Calendar>` (passado limitado ao 1º mês com conteúdo, futuro ao mês de hoje). `PUT /api/settings/cameras/reorder` (admin) — reordena câmeras em lote via `{"ids":[...]}`. A rota estática `/reorder` é registrada antes de `PUT /api/settings/cameras/{id}` para ter precedência no mux do Go 1.22+. `display_order` não é aceito nos endpoints de create/update; o update preserva a ordem existente da câmera. Notificações por usuário (escopadas ao usuário do JWT, não admin-only): `GET /api/notifications` (lista + `unread_count`), `POST /api/notifications/{id}/read`, `POST /api/notifications/read-all`, `DELETE /api/notifications/{id}`, `DELETE /api/notifications`. **Aviso de atualização disponível:** o `release.Checker` (`OnCheck`, ligado em `main.go`) chama `Server.NotifyUpdateAvailable` ao fim de cada checagem — quando há update e a versão `latest` mudou (dedup em memória, `updateNotified`), insere uma `user_notification` (`Type:info`, `Link:/settings/about`) **para cada admin** (viewers não veem a seção de updates). Na UI, a página **Sobre** (`AboutPage` → `UpdatesSection`) só renderiza a seção "Atualizações" **quando há update disponível** (sem update/erro/em-dia → nada); a `NotificationsPage` torna a notificação **clicável** quando tem `link` (navega + marca como lida). Preferências do usuário: `GET /api/me/preferences` (`{theme}`) e `PUT /api/me/preferences` (`{theme: dark|moderno}`, valida o valor). Device info: `GET /api/cameras/{id}/device-info` (responde `{collected_at, values:{...}}`; 404 se nunca capturado) e `POST /api/cameras/{id}/device-info/refresh` (admin; recoleta sob demanda). A coleta dispara automaticamente em background no cadastro da câmera (`captureDeviceInfoAsync` em `handleCreateCamera`). State classification (config admin + leitura do estado): `GET|POST /api/settings/cameras/{id}/classifiers`, `PUT|DELETE /api/settings/cameras/{id}/classifiers/{cid}` (admin), `POST /api/settings/cameras/{id}/classifiers/{cid}/train` (admin — com `samples` `{label, image_b64}` no corpo (form "Salvar e treinar") salva os crops em `{Storage.Path}/state_train/{cid}/{classe}/`; **sem corpo** (botões "Treinar agora"/"Treinar todos" da lista) recorta server-side as amostras já persistidas em `state_samples/{cid}` via `stateengine.BuildTrainSetFromSamples`. Em ambos exige ≥2 classes e dispara `/classify/train` no serviço YOLO → treina **um modelo por classificador** `custom-cls-{cid}` — `Classifier.ModelName()` —, NÃO um `custom-cls` compartilhado, senão os estados de classificadores diferentes se misturam), `GET|POST /api/settings/cameras/{id}/classifiers/{cid}/samples` (persistência das amostras = frames inteiros em `{Storage.Path}/state_samples/{cid}/`, reidrata o form ao editar — note que `Storage.Path` é o dir de gravações, ex.: `/data/recordings`), `GET /api/cameras/{id}/classifiers/{cid}/state` (cameraAccess; `null` até o runner escrever). Rodapé ao vivo: `GET /api/me/footer-states` (`requireFullAuth`) devolve, para o usuário do JWT, os classificadores `footer_enabled` em que ele é destinatário (`state_footer:{cid}` em `user_permissions`, via `db.FooterClassifiersForUser`), filtrados por acesso à câmera (admin todas; viewer via `user_cameras`), cada um com `{classifier_id, camera_id, name, state}` (`state` vazio até o runner escrever). |
| `internal/config` | Lê o arquivo de bootstrap (`camera.yaml`) com porta, `db_path`, storage e credenciais do admin. Variáveis de ambiente sobrescrevem campos específicos (ver abaixo). |
| `internal/logger` | `stdout`: JSON em stdout. `file`: um arquivo por nível (`debug.log`, `info.log`, `warn.log`, `error.log`) no diretório configurado, cada um com **rotação** via `gopkg.in/natefinch/lumberjack.v2`. Knobs no `camera.yaml` seção `log:` (`max_size_mb`, `max_age_days`, `max_backups`, `compress`); defaults 50 MB / 30 dias / 10 arquivos / gzip on, aplicados via accessors `…OrDefault()` em `config.LogConfig` (ponteiros distinguem ausente de `0`=ilimitado). Rotação só vale para `output: file`. |
| `frontend/` | SPA React/Vite/Tailwind embutida via `go:embed all:dist`. `ChangePasswordPage` — tela obrigatória no primeiro login; bloqueia acesso ao restante da UI enquanto `must_change_password=true` no JWT. |

### Autenticação

O JWT é assinado com um segredo aleatório gerado no boot — tokens não sobrevivem a reinicializações do servidor. O token é aceito via header `Authorization: Bearer <token>` ou query param `?token=<token>` (necessário para `<video src>` e `<HLSPlayer>`).

Fluxo de primeiro acesso: o admin inicial é criado com `must_change_password = true`. No primeiro login o servidor emite um token com esse claim; o frontend redireciona obrigatoriamente para `ChangePasswordPage`. Após a troca a flag é zerada no banco e o acesso normal é liberado. A senha do bootstrap não precisa ser atualizada — serve apenas na criação inicial.

### Build info

`version`, `commit` e `builtAt` são injetados via `-ldflags` no `Makefile`. Em `main.go` são passados ao servidor via `WithVersion(version)` e `WithBuildInfo(commit, builtAt)`. O endpoint `GET /api/about` expõe esses valores junto com `uptime_seconds` e `go_version`.

## Variáveis de ambiente

| Variável | Campo sobrescrito |
|---|---|
| `CAMERA_TIMEZONE` | `timezone` (fuso da instalação; usado pelo servidor para interpretar datas locais) |
| `CAMERA_SERVER_JWT_SECRET` | `server.jwt_secret` (segredo JWT fixo; vazio = gerado aleatoriamente a cada boot) |

## Diretório `amostras/`

O diretório `amostras/` (listado no `.gitignore`) é reservado para arquivos que o navigator coloca para análise contextual — screenshots, logs, dumps de banco, exemplos de vídeo ou qualquer artefato que ajude a diagnosticar um problema. Claude deve inspecionar o conteúdo desse diretório quando o navigator mencionar que colocou algo lá, ou quando precisar de evidência concreta para uma investigação.

## Manutenção contínua

- **Decisões de fluxo se registram neste `CLAUDE.md`** — ele é a fonte canônica. A memória do Claude é só atalho/ponteiro: nunca deixe uma regra de workflow apenas na memória.
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
