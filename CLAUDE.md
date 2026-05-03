# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## O que é este projeto

Sistema de monitoramento residencial via RTSP. Cada câmera configurada tem dois processos ffmpeg rodando em paralelo: um grava chunks MP4 em disco e outro gera segmentos HLS para visualização ao vivo. O frontend React é embutido no binário Go via `go:embed`.

## Comandos principais

### Backend (Go)

```bash
go test ./...                                         # todos os testes
go test ./internal/server/... -run TestLogin          # teste específico
go build ./cmd/camera                                 # binário de produção
go run ./cmd/camera --config camera.yaml              # desenvolvimento local
```

### Frontend

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

### Fluxo de inicialização (`cmd/camera/main.go`)

Para cada câmera configurada, `main` cria e inicia:
1. Um `recorder.Recorder` — grava RTSP → MP4 chunk
2. Um `streaming.HLSStreamer` — grava RTSP → segmentos HLS para live

O `server.Server` é levantado em goroutine separada e serve a SPA + API REST.

### Pacotes internos

| Pacote | Responsabilidade |
|---|---|
| `internal/exec` | Interfaces `Commander` / `Process` e implementação real com `os/exec`. Injetadas nos pacotes abaixo para permitir testes sem ffmpeg. |
| `internal/recorder` | Grava RTSP em chunks MP4. Armazena em `{storage}/{camera_id}/{YYYY/MM/DD}/{YYYYMMDDHHmmss}.mp4`. |
| `internal/streaming` | Gera playlist HLS ao vivo em `{segments_path}/{camera_id}/index.m3u8` com janela de 5 segmentos de 2s. |
| `internal/server` | HTTP server com JWT HS256 (segredo gerado a cada boot, expira em 24h). Serve API REST, arquivos de gravação, segmentos HLS e a SPA React. |
| `internal/config` | Lê `camera.yaml`; variáveis de ambiente sobrescrevem campos específicos (ver abaixo). |
| `internal/logger` | `stdout`: JSON em stdout. `file`: um arquivo por nível (`debug.log`, `info.log`, `warn.log`, `error.log`) no diretório configurado. |
| `internal/rtsp` | Cliente RTSP com interface `Connection` — presente mas não usado no fluxo principal de gravação. |
| `frontend/` | SPA React/Vite/Tailwind embutida via `go:embed all:dist`. |

### Autenticação

O JWT é assinado com um segredo aleatório gerado no boot — tokens não sobrevivem a reinicializações do servidor. O token é aceito via header `Authorization: Bearer <token>` ou query param `?token=<token>` (necessário para `<video src>` e `<HLSPlayer>`).

### Frontend (`frontend/src/`)

Três páginas com React Router: `LoginPage` → `DashboardPage` → `CameraPage`. O token JWT fica em `localStorage` (gerenciado em `auth.ts`). Em desenvolvimento, o Vite faz proxy de `/api` e `/stream` para `localhost:8080`.

## Variáveis de ambiente

| Variável | Campo sobrescrito |
|---|---|
| `STORAGE_PATH` | `storage.path` |
| `TIMEZONE` | `timezone` (fuso da instalação; usado pelo servidor para interpretar datas locais) |
| `LOG_OUTPUT` | `log.output` |
| `LOG_PATH` | `log.path` |

## Forma de trabalho

O desenvolvimento segue **XP (Extreme Programming)** com **TDD red → green → refactor**:

- O **navigator** (usuário) define a história, revisa o código e aprova cada etapa.
- O **driver** (Claude) implementa, sempre guiado pelos testes.

### Fluxo por história

1. Abrir uma branch nova por história: `git checkout -b <tipo>/<descricao-curta>` a partir de `master`.
2. Escrever o teste que falha (**red**) — nunca escrever código de produção sem um teste falhando antes.
3. Implementar o mínimo para o teste passar (**green**).
4. Refatorar se necessário, mantendo os testes verdes (**refactor**).
5. Propor o commit ao navigator antes de executá-lo — aguardar aprovação explícita.
6. Após aprovação, commitar com mensagem semântica e propor o merge em `master`.

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
