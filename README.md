# camera

[![CI](https://github.com/jacksonbicalho/camera/actions/workflows/ci.yml/badge.svg)](https://github.com/jacksonbicalho/camera/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/release/jacksonbicalho/camera?include_prereleases&label=release)](https://github.com/jacksonbicalho/camera/releases/latest)
[![Go 1.25](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)

Sistema de monitoramento residencial via RTSP. Um único binário estático grava câmeras em chunks MP4, serve live view em HLS, detecta movimento por análise de frames e expõe uma interface web embarcada — sem banco de dados, sem agentes externos.

---

## Funcionalidades

- **Gravação contínua** — chunks MP4 organizados por câmera e data (`{câmera}/{YYYY}/{MM}/{DD}/{HHmmss}.mp4`)
- **Live view HLS** — stream ao vivo com latência baixa; modo DVR opcional para seek por timestamp
- **Detecção de movimento** — diff de frames via ffmpeg, limiar configurável, cooldown e score em tempo real
- **Retenção inteligente** — gravações com movimento ficam mais tempo; sem movimento são apagadas mais cedo
- **Interface web embarcada** — React/Tailwind embutido no binário, zero dependências de assets externos
- **Multi-câmera** — número ilimitado de câmeras, cada uma com overrides individuais de codec, resolução e detecção
- **Binário único** — nenhuma dependência além do `ffmpeg` no sistema

---

## Instalação rápida (Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/jacksonbicalho/camera/master/scripts/install.sh | sudo bash
```

O script detecta a arquitetura (`amd64`, `arm64`, `arm`), baixa o binário da última release e registra um serviço systemd.

- **Instalação interativa** (terminal direto): o assistente de configuração (`camera init`) é executado automaticamente para configurar câmeras, senha e demais opções; o serviço sobe ao final.
- **Instalação via pipe** (`curl | bash`): um arquivo de configuração mínimo é criado em `/etc/camera/camera.yaml`; o serviço é habilitado mas **não iniciado** até que a configuração seja concluída.

**Configurar câmeras após instalação via pipe:**

```bash
sudo camera init --output /etc/camera/camera.yaml
sudo systemctl start camera
```

**Caminhos customizáveis:**

```bash
curl -fsSL https://raw.githubusercontent.com/jacksonbicalho/camera/master/scripts/install.sh \
  | sudo bash -s -- \
      --install-dir /usr/local/bin \
      --config-dir  /etc/camera \
      --data-dir    /data/recordings \
      --service-name camera
```

**Outros comandos úteis:**

```bash
camera --version                   # versão instalada
sudo systemctl restart camera      # reiniciar após editar config
sudo journalctl -u camera -f       # acompanhar logs
sudo systemctl status camera       # status do serviço
```

### Desinstalar

```bash
# Remove binário e serviço (mantém config e gravações)
sudo camera-uninstall

# Remove também a configuração
sudo camera-uninstall --remove-config

# Remove também as gravações
sudo camera-uninstall --remove-data

# Remove tudo
sudo camera-uninstall --remove-config --remove-data
```

---

## Docker

```bash
# Copiar e editar a configuração
cp camera.yaml.example camera.yaml
nano camera.yaml

# Subir em produção
docker compose --profile production up -d
```

```yaml
# docker-compose.yml (trecho relevante)
services:
  camera:
    profiles: [production]
    volumes:
      - ./camera.yaml:/app/camera.yaml:ro
      - ./storage:/data/recordings
    restart: unless-stopped
```

---

## Download manual

Baixe o binário da [última release](https://github.com/jacksonbicalho/camera/releases/latest) para sua plataforma:

| Plataforma | Arquivo |
|---|---|
| Linux x86-64 | `camera-linux-amd64` |
| Linux ARM64 (Raspberry Pi 3/4/5 64-bit) | `camera-linux-arm64` |
| Linux ARMv7 (Raspberry Pi 2/3 32-bit) | `camera-linux-arm` |
| Windows x86-64 | `camera-windows-amd64.exe` |

```bash
chmod +x camera-linux-amd64
./camera-linux-amd64 --config camera.yaml
```

---

## Configuração

Copie o exemplo e edite:

```bash
cp camera.yaml.example camera.yaml
```

```yaml
timezone: America/Sao_Paulo

server:
  port: 8080
  username: admin
  password: sua-senha-aqui

storage:
  path: /data/recordings
  retention:
    with_motion_minutes: 10080   # 7 dias — gravações com movimento
    without_motion_minutes: 1440 # 1 dia  — cenas paradas
  max_size_gb: 20

motion:
  enabled: true
  threshold: 0.02   # fração de pixels alterados (0.0–1.0)
  fps: 2
  cooldown_seconds: 30

cameras:
  - id: entrada
    rtsp_url: rtsp://192.168.1.10:554/stream

  - id: quintal
    rtsp_url: rtsp://192.168.1.11:554/stream
    chunk_duration: 10m
    has_audio: false
    motion:
      enabled: false
```

### Referência de campos

| Campo | Padrão | Descrição |
|---|---|---|
| `timezone` | `UTC` | Fuso horário para logs e nomes de arquivo |
| `server.port` | — | Porta HTTP da interface web |
| `server.hls_dvr_seconds` | `0` | Janela DVR do HLS em segundos (0 = desabilitado) |
| `storage.path` | — | Diretório raiz das gravações |
| `storage.retention.with_motion_minutes` | `0` | Retenção de gravações COM movimento (0 = nunca apaga) |
| `storage.retention.without_motion_minutes` | `0` | Retenção de gravações SEM movimento (0 = desabilitado) |
| `storage.max_size_gb` | `0` | Limite de disco em GB (0 = desabilitado) |
| `motion.threshold` | `0.02` | Fração mínima de pixels alterados para registrar evento |
| `motion.cooldown_seconds` | `0` | Intervalo mínimo entre eventos consecutivos |
| `camera.chunk_duration` | `5m` | Duração de cada arquivo MP4 gravado |

Variáveis de ambiente sobrescrevem campos específicos:

| Variável | Campo |
|---|---|
| `CAMERA_STORAGE_PATH` | `storage.path` |
| `CAMERA_TIMEZONE` | `timezone` |

---

## Compilar a partir do código

**Requisitos:** Go 1.25+, Node 20+, Yarn, ffmpeg

```bash
git clone https://github.com/jacksonbicalho/camera.git
cd camera

# Binário local
make build
./dist/camera --config camera.yaml

# Cross-compilação (todos os alvos)
make all

# Alvo específico
make linux-arm64   # Raspberry Pi
make windows-amd64
```

---

## Desenvolvimento

```bash
# Backend
go test ./...

# Frontend
cd frontend
yarn install
yarn dev      # Vite dev server em :5173 com proxy para :8080
yarn test
yarn lint
```

Docker com live reload:

```bash
make run   # docker compose --profile development
```

### Wizard de configuração

O subcomando `init` gera um `camera.yaml` interativamente:

```bash
./camera init
```

---

## Arquitetura

```
                ┌─────────────────────────────────────────┐
                │              cmd/camera                  │
                │  Para cada câmera:                       │
                │  ├─ recorder   → chunks MP4 em disco     │
                │  ├─ streaming  → segmentos HLS ao vivo   │
                │  └─ motion     → detecção por diff frame  │
                │                                          │
                │  server → API REST + SPA React (embed)   │
                └─────────────────────────────────────────┘

Armazenamento:
  {storage}/
  └── {camera_id}/
      └── {YYYY}/{MM}/{DD}/
          ├── {HHmmss}.mp4     ← chunk gravado
          └── motion.ndjson    ← eventos de movimento (JSON Lines)
```

O servidor emite dois endpoints SSE por câmera:
- `/api/cameras/{id}/motion/live` — eventos acima do limiar
- `/api/cameras/{id}/motion/scores` — score bruto por frame (tempo real)

### Pacotes internos

| Pacote | Responsabilidade |
|---|---|
| `internal/recorder` | Grava RTSP → MP4 em chunks |
| `internal/streaming` | Gera playlist HLS ao vivo |
| `internal/motion` | Detecta movimento via ffmpeg pipe raw |
| `internal/storage` | Apaga MP4s com retenção diferenciada por categoria |
| `internal/server` | HTTP + JWT + API REST + SPA embarcada |
| `internal/config` | Lê `camera.yaml` e aplica overrides de env vars |
| `internal/ffprobe` | Detecta codec, áudio e resolução via ffprobe |
| `cmd/mcp-ffprobe` | Servidor MCP (stdio) para inspeção de câmeras via ferramentas de IA |

---

## Publicar release

```bash
# Prévia do que será gerado
./scripts/release.sh --dry-run

# Criar tag e publicar
./scripts/release.sh
```

O script lê os commits convencionais desde a última tag, calcula o bump semântico e cria uma tag `vX.Y.Z-beta.N`. O push da tag dispara o GitHub Actions que compila e publica os binários automaticamente.
