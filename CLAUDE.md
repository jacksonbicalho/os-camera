# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## O que é este projeto

Sistema de monitoramento residencial via RTSP. Cada câmera configurada tem três processos ffmpeg rodando em paralelo: um grava chunks MP4 em disco, outro gera segmentos HLS para visualização ao vivo e um terceiro detecta movimento por diff de frames. O frontend React é embutido no binário Go via `go:embed`.

## Fluxo de trabalho

O desenvolvimento segue **XP (Extreme Programming)** com **TDD red → green → refactor**:

- O **navigator** (usuário) define a história, revisa o código e aprova cada etapa.
- O **driver** (Claude) implementa, sempre guiado pelos testes.

> ⚠️ **`master` e `develop` são protegidos.** Push direto é bloqueado em ambos. Features entram via PR para `develop`; o PR `develop → master` acontece apenas no momento de release. Nunca commite ou force-push diretamente em nenhum dos dois.

### Estratégia de branches

```
master   ← PRs de release (develop → master)
  ↑
develop  ← PRs de feature/fix/chore
  ↑
feat/xyz · fix/abc · chore/def  ← branches de história
```

- Branches de história partem **sempre de `develop`**: `git checkout -b <tipo>/<desc> develop`
- PRs de história têm **`develop` como base**: `gh pr create --base develop`
- PRs de release têm **`master` como base**: `gh pr create --base master --head develop`
- **Ciclo de vida da branch:** deletada imediatamente após o merge em `develop` — remoto automaticamente pelo GitHub, local com `git branch -d <branch>` no passo de merge em lote.

### CI e branch protection

Todo PR para `master` ou `develop` dispara `.github/workflows/ci.yml` com dois jobs paralelos:
- **Backend**: `go test ./...` + `go build ./...`
- **Frontend**: `yarn lint` + `yarn test --run` + `yarn build`

Ambos os branches estão protegidos: push direto bloqueado, PR obrigatório, checks `Backend` e `Frontend` obrigatórios. `master` exige 1 aprovação humana; `develop` não exige aprovação (projeto solo — CI já é a barreira de qualidade). Para reaplicar as regras (ex: em novo repositório):

```bash
# master — só aceita PRs vindos de develop (releases); exige 1 aprovação humana
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

# develop — aceita PRs de feature branches; CI obrigatório, sem aprovação humana
gh api repos/{owner}/{repo}/branches/develop/protection \
  --method PUT \
  --header "Accept: application/vnd.github+json" \
  --input - <<'EOF'
{
  "required_status_checks": { "strict": true, "contexts": ["Backend", "Frontend"] },
  "enforce_admins": false,
  "required_pull_request_reviews": { "dismiss_stale_reviews": true, "required_approving_review_count": 0 },
  "restrictions": null
}
EOF
```

### Fluxo por história

> ⚠️ **OBRIGATÓRIO:** Antes de escrever qualquer linha de código ou teste, o driver DEVE criar o arquivo de história E abrir a branch (use `/story`). Sem exceção — nem para bugs simples, nem para "pequenas correções".

1. Criar `stories/YYYYMMDDHHmm_<descricao>.md` e abrir uma branch a partir de `develop`: `git checkout -b <tipo>/<descricao-curta> develop`. Se a história cobrir mais de um assunto independente, questionar o navigator antes de continuar — histórias devem ser pequenas e focadas. O 1º Critério de Aceitação é **sempre** `- [] Backend e frontend verdes (auto: scripts/check.sh)` (sem "se aplicável").
2. **Revisão da história (antes de implementar).** A história inclui o campo `- [] História revisada`. O driver apresenta o plano e **aguarda o navigator marcar `[x] História revisada`** — monitorando o arquivo da story em background (grep case-insensitive). **Nenhuma linha de código ou teste antes disso.**

   > ⚠️ **O driver NÃO inicia o red phase enquanto `História revisada` não estiver `[x]`.** O navigator revisa o plano (Contexto/Solução/Critérios) primeiro; só então a implementação começa.

3. Escrever o teste que falha (**red**) — nunca escrever código de produção sem um teste falhando antes.
4. Implementar o mínimo para o teste passar (**green**).
5. Refatorar se necessário, mantendo os testes verdes (**refactor**).
6. Executar a suíte de testes (obrigatório antes de apresentar ao navigator):
   - Backend: `go test ./...` + `go build ./...`
   - Frontend: `yarn lint` + `yarn test --run` + `yarn build` (em `frontend/`)
   - Nunca prosseguir se qualquer um desses falhar.
7. Adicionar seção `## Revisão` na história e **aguardar aprovação explícita do navigator**.

   > ⚠️ **Nenhum commit pode ser feito antes de o navigator marcar `[x] Aprovado`.** O driver apresenta o resultado, o navigator revisa o código e aprova — só então o commit acontece.

   > ⚠️ **O driver NÃO marca os Critérios de Aceitação.** Apenas o **1º critério** (verdes) é marcado automaticamente pelo `scripts/check.sh`. **Todos os demais critérios** e o `[x] Aprovado` são marcados pelo **navigator** via `scripts/story-approval.sh`. O driver pode preencher a seção `## Revisão`, mas **sem tocar nos checkboxes dos critérios**.

8. Com a aprovação do navigator, o ciclo roda **direto, sem novas perguntas** — mas o driver **só segue quando TODOS os Critérios de Aceitação E `[x] Aprovado` estiverem marcados** (apenas monitorar o arquivo após pedir `scripts/story-approval.sh`): commitar (mensagem semântica) → `scripts/push-pr.sh`, que **orquestra o ciclo inteiro** — push + abre o PR (**sempre `--base develop`**, nunca `master`) + aguarda o CI + mergeia quando verde (chama o `merge-when-green.sh` internamente). A aprovação da story autoriza, de uma vez, commit + push + PR + merge-quando-verde. **CI vermelho:** o `push-pr.sh` propaga o erro sem mergear (PR fica aberto) — **Política A**: um fix trivial pós-aprovação (deixar o CI verde) não exige nova aprovação; o driver corrige, avisa o que mudou, commita e roda `push-pr.sh` de novo (idempotente: não recria o PR, só reespera o CI). Se o fix mexer em lógica de produção, volta pro navigator. Use `scripts/push-pr.sh --no-merge` para só abrir o PR sem mergear. **Apenas o corte de release** (`develop → master`) depende de autorização explícita do navigator.
9. Atualizar o arquivo de release correspondente em `releases/`: preencher a branch e o número do PR na tabela, marcar `[~]`; o `merge-when-green.sh` marca `[~]→[✓]` ao mergear em `develop`. O **corte de release** (PR `develop → master`) é o passo que aguarda o navigator liberar.

### Histórias

Histórias ficam em `stories/` (gitignored). O nome do arquivo usa timestamp no formato
`YYYYMMDDHHmm_<descricao>.md` — igual às migrations de banco — garantindo ordenação
cronológica natural ao listar o diretório.

Ao iniciar uma nova história:
- Criar o arquivo `stories/YYYYMMDDHHmm_<descricao>.md` com contexto, critérios de aceitação e notas técnicas.
- Ao concluir a implementação, adicionar uma seção `## Revisão` no arquivo com checklist do que foi feito.
- **Nenhum commit pode ser feito sem aprovação explícita do navigator. Só proceder com commit ou PR após o navigator marcar `[x] Aprovado` na seção Revisão.**

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

### Slash commands

O fluxo acima é automatizado pelos slash commands em `.claude/commands/`:

| Comando | O que faz |
|---|---|
| `/story <descrição livre>` | Cria story file + branch a partir de develop (passo 1 do fluxo). Passe **só a descrição**; Claude decide tipo/escopo/slug. |
| `/release-pr [vX.Y.Z]` | Valida release file e abre PR develop → master (após todas as histórias `[✓]`). |
| `/release-tag` | Roda `./scripts/release.sh` em master após o PR de release ser mergeado. |

Use os commands em vez de executar os passos manualmente — eles validam pré-condições e evitam erros (branch errada, working tree suja, status incompleto).

### Hooks de pre-commit (`.claude/settings.json`)

Hooks `PreToolUse` (matcher `Bash`, versionados no repo) impõem o fluxo automaticamente. O `settings.json` só **chama scripts** versionados em `scripts/hooks/` (a lógica vive lá, não inline):

| Gate | Script | Bloqueia quando |
|---|---|---|
| Aprovação da story | `scripts/hooks/story-approved.sh` | `git commit` em branch de história cuja story não tem `[x] Aprovado`. |
| Target do PR | `scripts/hooks/pr-target.sh` | `gh pr create --base master` a partir de branch que não seja `develop`/`release/*`. |
| Testes backend | `scripts/hooks/precommit-tests.sh` | `git commit` quando `go build ./...` ou `go test -count=1 ./...` falham. |

O gate de testes roda no host (Go instalado), **sem cache** (`-count=1`) — só execução limpa pega testes dependentes de `time.Now()` (o cache do Go não rastreia o relógio). Escopo é backend; o frontend segue coberto pelo CI. Hooks só recarregam no início da sessão do Claude Code: alterações em `settings.json` (ou nos scripts de hook) valem a partir da próxima sessão.

### Scripts de workflow (`scripts/`)

Encadeiam o fluxo por história. **Checkboxes usam `[]` para não-marcado** (e `[x]` para marcado).

| Script | Quem roda | O que faz |
|---|---|---|
| `check.sh` | Claude | "CI local": `go build`+`go test` sempre; `frontend-check.sh` se `frontend/` mudou (vs develop). Se tudo verde, marca o **1º Critério de Aceitação** da story `[x]`. |
| `story-approval.sh` | **Navigator** (`! scripts/story-approval.sh`) | Interativo: percorre os Critérios de Aceitação, pergunta sobre cada não-marcado e marca conforme a resposta; no fim, oferece marcar `[x] Aprovado`. |
| `commit.sh` | Claude | Commita o que está staged usando o heading `#` da story como mensagem (já é `tipo(escopo): desc`); exige `[x] Aprovado`; adiciona `Co-Authored-By`. |
| `push-pr.sh` | Claude | **Orquestra o ciclo pós-aprovação:** push + `gh pr create --base develop` + aguarda o CI e mergeia (chama `merge-when-green.sh`). Só com tree limpa + story aprovada; idempotente (não recria PR; re-roda após fix). `--no-merge` só abre o PR sem mergear. CI vermelho: propaga erro sem mergear. |

### Merge pós-PR

`scripts/merge-when-green.sh <PR#>` colapsa o ciclo pós-PR numa única invocação (economia de tokens): aguarda o CI em silêncio, mergeia em `develop`, sincroniza o branch local, deleta a branch da história e marca `[~]→[✓]` na linha do PR no release file mais recente. Imprime só o resumo. **Recusa** PRs com base `master` (releases são aprovadas à mão) e é idempotente em PR já mergeado. É a primitiva chamada pelo `push-pr.sh` ao final, mas também pode ser invocada avulsa (ex.: retomar após um fix de CI num PR já aberto).

`scripts/release-tag.sh [--dry-run]` colapsa o **corte de release** (após o PR develop→master já mergeado): cria/envia a tag via `release.sh` (confirmação automática), aguarda o workflow Release publicar **em silêncio** (poll de `gh release view`), mergeia `master→develop` (passo pós-tag) e imprime uma linha (`RELEASED <versão> | assets: N | develop sincronizado`). `--dry-run` só mostra a versão que sairia. Substitui o ciclo manual com `gh run watch`.

### Release

```bash
./scripts/release.sh             # calcula bump, gera changelog, cria tag anotada e faz push
./scripts/release.sh --dry-run   # prévia sem criar nada
```

O script lê os commits convencionais desde a última tag, determina o bump (`feat` → minor, breaking → major, resto → patch), gera o changelog agrupado por tipo e cria uma tag no formato `vX.Y.Z-dev`. O push da tag dispara o GitHub Actions que publica a release. O sufixo `-dev` indica projeto em desenvolvimento ativo; quando atingir estabilidade, as tags passarão a usar `vX.Y.Z` sem sufixo.

#### Planejamento de release (releases/)

`releases/` (gitignored) agrupa histórias em uma release antes de mergeá-las.

**Fluxo:**
1. Criar `releases/YYYYMMDDHHmm_vX.Y.Z.md` com as histórias planejadas.
2. Ao concluir cada história, preencher branch e PR na tabela e marcar `[~]` (aguardando aprovação no GitHub — PR targeta `develop`).
3. Após aprovação no GitHub, marcar `[x]`.
4. Quando todas estiverem `[x]`, o navigator diz **"pode mergear a release"** — Claude itera a lista, mergeia cada PR em `develop` em sequência, deleta a branch local (`git branch -d <branch>`) e marca `[✓]`. O GitHub deleta a branch remota automaticamente após o merge (setting "Automatically delete head branches" ativo).
5. Após todos os merges em `develop`, Claude abre um PR `develop → master` com título `release: vX.Y.Z`.
6. Após aprovação e merge do PR de release, Claude roda `./scripts/release.sh` para gerar a tag.
7. **Após a tag ser criada**, mergear `master` de volta em `develop` para que `git describe` retorne a versão correta no modo dev:
   ```bash
   git checkout develop && git fetch origin master && git merge origin/master --no-edit && git push origin develop
   ```
   Sem este passo, `git describe` não encontra a tag (que vive no merge commit de master) e retorna a versão anterior.

**Estrutura do arquivo de release:**

```markdown
# Release vX.Y.Z — YYYYMMDD

## Histórias

| Status | Descrição | Branch | PR |
|--------|-----------|--------|----|
| [ ]    | descrição | `branch-name` | — |
| [~]    | descrição | `branch-name` | #123 |
| [x]    | descrição | `branch-name` | #123 |
| [✓]    | descrição | `branch-name` | #123 |
```

**Legenda de status:** `[ ]` planejada · `[~]` aguardando aprovação no GitHub · `[x]` aprovada · `[✓]` mergeada.

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

**Design tokens e tema (`src/index.css`).** Base estrutural organizada como **tema → modo → valores**. Os tokens vivem num bloco `@theme` do Tailwind v4 (geram utilitários):
- **Tipografia**: escala `text-display/h1/h2/h3/h4/body/caption` (size + line-height) → use os utilitários (`text-h2`, `text-h4`…) em vez de tamanhos soltos (`text-lg`/`text-xs`).
- **Cor semântica**: papéis `bg-background`, `bg-surface`, `bg-surface-2`, `text-foreground`, `text-muted`, `text-faint`, `border-border`, `bg-primary`/`text-on-primary`, `bg-danger/success/warning`. Os valores no `@theme` são o **modo dark** (padrão); `[data-mode="light"]` sobrescreve **só** os tokens semânticos que mudam (accents seguem vívidos). A migração das cores cruas (`bg-gray-900`/`text-white`) para esses papéis é **incremental** — boa parte do app ainda usa as classes Tailwind diretas, com o bloco legado `[data-mode="light"]` remapeando a rampa de cinzas.

**Color mode ≠ tema.** `dark`/`light`/`system` são **modos de cor**; o **tema** é a identidade (paleta + tipografia). `ThemeContext` (`contexts/ThemeContext.tsx`) expõe `mode`/`setMode` (modo) e `theme` (hoje só `'default'`); aplica `data-mode` (resolvido dark/light) no `<html>`. A preferência persistida (`users.theme`) guarda o **modo**; quando houver um 2º tema, entra um campo de tema. **Adicionar um tema** no futuro = novo conjunto de valores de tokens (sem refatorar componentes).

Rotas de settings por câmera: `/settings/cameras/:id` (detalhes) → `/settings/cameras/:id/motion` (detecção de movimento) → `/settings/cameras/:id/motion/zones` (zonas de exclusão — sub-página de motion).

**`SidebarContext`** (`contexts/SidebarContext.tsx`) — contexto dividido em dois para evitar loop de re-render: `SidebarItemsContext` (leitura, usado apenas em `Sidebar`) e `SetSidebarItemsContext` (setter estável, nunca causa re-render em quem chama). O `SidebarItemsProvider` fica na raiz do `App.tsx`, fora de qualquer layout, para que `CameraPage` consiga registrar itens antes de montar o layout. `useSidebarItems()` lê os itens; `useSetSidebarItems()` retorna o setter. Nunca usar `useSidebarItems()` em componentes que também chamam `setItems()` — isso cria ciclo de re-render.

Componentes reutilizáveis: `AppLayout` (layout base; exibe footer com versão, data do build, uptime e versão do Go via `GET /api/about`), `SettingsLayout` (sidebar + conteúdo para `/settings/*`; prop `wide` usa `max-w-7xl` em vez do padrão `max-w-4xl` — usado na página de zonas para canvas maior), `SettingsSection` (card com lista de campos label/valor), `HLSPlayer` (player HLS com alerta de movimento ao vivo, seek ao evento e botões de mudo/fullscreen), `ListPanel`, `MotionScoreChart` (gráfico SVG em tempo real dos scores brutos via SSE, escala logarítmica, janela de 30 s), `ConfirmDialog` (modal fixo de confirmação para ações destrutivas; prop `danger` alterna botão vermelho/azul).

`NotificationContext` — contexto global que abre um `EventSource` por câmera (via `GET /api/cameras/{id}/motion/live`) e acumula notificações de movimento em `localStorage` (máx. 100). Expõe: `notifications`, `unreadCount`, `markRead`, `markAllRead`, `markSelectedRead`, `markAllUnread`, `remove`, `removeAll`, `removeSelected`. O `Header` exibe o sino com badge, painel de notificações com seleção múltipla, kebab de ações (marcar lidas/não lidas, excluir) e `ConfirmDialog` para confirmações.

`CameraPage` — além das gravações e eventos, exibe abaixo do player uma barra de controles com: botão mudo (sincronizado com os controles nativos via `onVolumeChange`), seletor de velocidade 1×–32× (detecta limite do browser progressivamente e exibe aviso), toggle "Reprodução contínua" (avança automaticamente para a próxima gravação ou próximo evento ao terminar o clip atual). Usa refs (`continuousPlayRef`, `activeEventIdxRef`, `visibleEventsRef`, `recordingsRef`) para evitar closures stale nos handlers de vídeo. Cada evento de movimento com campo `frame` exibe um thumbnail 64×40px do snapshot JPEG; ao clicar abre modal com a imagem em tamanho completo. Ao receber um novo evento via SSE, a lista de eventos é re-fetched do servidor (em vez de adicionar o evento parcial), garantindo que o campo `frame` esteja presente. Layout da página: player → painel lateral condicional (gravações/eventos/calendário) → `VerticalTimeline` sempre visível à direita. Clicar em Gravações ou Eventos enquanto ao vivo auto-sai do modo live e inicia reprodução; clicar em Ao vivo fecha o painel lateral ativo. Quando `recording_enabled=false`: botão "Gravações" some da sidebar (condição: `cam?.recording_enabled !== false && recordings.length > 0`); painel de gravações exibe mensagem "Gravação desabilitada". Poll de gravações e eventos: intervalo de 5 s para o dia atual, 30 s para datas passadas; ambos (`loadRecordingsData` + `loadMotionEvents`) são atualizados juntos a cada tick para manter os painéis sincronizados após exclusão pelo cleaner.

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
- `POST /finetune` / `GET /finetune/status/{id}` / `DELETE /finetune/{id}` — treino assíncrono

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
| `internal/motion` | Detecta movimento via ffmpeg pipe raw. O pipe entrega frames **RGB full-res** (`format=rgb24`); cada frame é reduzido a cinza na resolução de diff (`downscaleRGBToGray`, default 1/4) em memória para o cálculo do diff/bbox, enquanto o frame full-res original é mantido para o snapshot. Expõe dois canais: `Events()` para eventos acima do limiar e `RawScores()` para o score bruto de cada frame diff. A cada evento persiste no banco (`motion_events`) e salva um JPEG anotado (`_motion.jpg`) com bounding box e score: o snapshot é o **próprio frame que disparou** (anotado in-place por `annotateRGBFrame`, full-res colorido), então o box sempre coincide com o sujeito — **não há mais grab RTSP assíncrono** (que capturava segundos depois e desalinhava o box). **`motion.ndjson` não é mais escrito** — todos os eventos vão direto para o banco; o arquivo é lido apenas como fallback legado. |
| `internal/storage` | `Cleaner` com retenção diferenciada por categoria e suporte a drives S3. Quando o banco está disponível: `syncRecordings()` varre o filesystem e sincroniza MP4s para a tabela `recordings` (`INSERT OR IGNORE`), atualizando `has_motion` via join com `motion_events`; `cleanFromDB()` consulta `recordings` para decisões de exclusão — inclui `AND ended_at IS NOT NULL` para nunca processar o arquivo em gravação. Ação configurável por categoria (`delete` ou `send_to_drive`): `uploadAndPurge()` faz upload S3, apaga o MP4 e chama `purgeMotionAssets()`. `purgeMotionAssets()` busca `frame_path` no banco, apaga os JPEGs do disco e deleta as linhas de `motion_events`. `slugify()` converte o nome da câmera para prefixo do objeto S3, transliterando acentos (`ã→a`, `ç→c`, etc.) via mapa estático. Sem banco: consulta `motion.ndjson` diretamente. |
| `internal/db` | Acesso ao SQLite (`modernc.org/sqlite`). Executa migrations em `internal/db/migrations/` na inicialização. Tabelas: `cameras`, `camera_recording_config`, `camera_motion_config`, `camera_motion_zones`, `users` (com `must_change_password`), `settings`, `recordings`, `motion_events`, `drives`, `retention_config`, `user_notifications` (notificações persistidas por usuário; FK `users` ON DELETE CASCADE), `camera_device_info` (metadados de hardware capturados pelo `internal/deviceinfo`, **EAV**: `(camera_id, key, value, collected_at)`, PK `(camera_id, key)`; cada captura é um snapshot completo — `SaveDeviceInfo` faz delete+insert numa tx, removendo chaves que sumiram; FK `cameras` ON DELETE CASCADE). Coluna `users.theme` (preferência de tema da UI, default `dark`). |
| `internal/ffprobe` | Executa e parseia saída JSON do ffprobe para detectar codec, áudio e dimensões do stream. |
| `internal/deviceinfo` | Captura metadados de hardware/manutenção da câmera logo após o cadastro. Saída é um **`map[string]string` plano de chaves namespaced** (`model`, `serial`, `firmware`, `mac`, `ntp.enabled`, `timezone`, `stream.main.gop`, `stream.sub.*`, e no futuro `capability.zoom`, `url.config`, `ptz.*`) + o dump bruto completo sob `raw.*` — EAV, então modelos diferentes guardam o que expõem sem mudar schema. Extensível por família via interface `Collector` (`Name`/`Detect`/`Collect`) — o "tipo". Hoje só o collector `dahua` (CGI Intelbras/Dahua, cliente HTTP digest em `cgi.go`); a função `Collect` escolhe o primeiro collector que dá `Detect`, marca `collector=<nome>` e mescla as chaves `stream.main.*` do `ffprobe` por cima (preenche só as ausentes; fallback `collector=generic` quando nenhum casa). |
| `internal/server` | HTTP server com JWT HS256 (segredo gerado a cada boot, expira em 24h). O claim `must_change_password=true` bloqueia todos os endpoints exceto `POST /api/auth/change-password`. Serve API REST, arquivos de gravação (incluindo snapshots `_motion.jpg`), segmentos HLS e a SPA React. Endpoints SSE de movimento: `/api/cameras/{id}/motion/live`, `/api/cameras/{id}/motion/scores` e `/api/cameras/{id}/motion/region-score` (score por zona/região, usado pelo canvas de zonas). `GET /api/stats` usa `SUM(size_bytes)` da tabela `recordings` quando banco disponível. `PUT /api/settings/cameras/reorder` (admin) — reordena câmeras em lote via `{"ids":[...]}`. A rota estática `/reorder` é registrada antes de `PUT /api/settings/cameras/{id}` para ter precedência no mux do Go 1.22+. `display_order` não é aceito nos endpoints de create/update; o update preserva a ordem existente da câmera. Notificações por usuário (escopadas ao usuário do JWT, não admin-only): `GET /api/notifications` (lista + `unread_count`), `POST /api/notifications/{id}/read`, `POST /api/notifications/read-all`, `DELETE /api/notifications/{id}`, `DELETE /api/notifications`. Preferências do usuário: `GET /api/me/preferences` (`{theme}`) e `PUT /api/me/preferences` (`{theme: dark|moderno}`, valida o valor). Device info: `GET /api/cameras/{id}/device-info` (responde `{collected_at, values:{...}}`; 404 se nunca capturado) e `POST /api/cameras/{id}/device-info/refresh` (admin; recoleta sob demanda). A coleta dispara automaticamente em background no cadastro da câmera (`captureDeviceInfoAsync` em `handleCreateCamera`). |
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
