# camera

[![CI](https://github.com/jacksonbicalho/camera/actions/workflows/ci.yml/badge.svg)](https://github.com/jacksonbicalho/camera/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/release/jacksonbicalho/camera?include_prereleases&label=release)](https://github.com/jacksonbicalho/camera/releases/latest)
[![Go 1.25](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)

Sistema de monitoramento residencial via RTSP. Um único binário estático grava câmeras em chunks MP4, serve live view em HLS, detecta movimento por análise de frames e expõe uma interface web embarcada. Toda a configuração é gerenciada via interface web e persistida em SQLite — sem edição manual de arquivos de câmera.

---

## Funcionalidades

- **Gravação contínua** — chunks MP4 organizados por câmera e data (`{câmera}/{YYYY}/{MM}/{DD}/{HHmmss}.mp4`)
- **Live view HLS** — stream ao vivo com latência baixa; modo DVR opcional para seek por timestamp
- **Detecção de movimento** — diff de frames via ffmpeg, limiar configurável, cooldown e score em tempo real; cada evento gera um snapshot JPEG anotado com bounding box e score sobrepostos
- **Retenção inteligente** — gravações com movimento ficam mais tempo; sem movimento são apagadas mais cedo; decisões via queries SQL
- **Configuração via UI** — câmeras, motion e zonas de exclusão gerenciados pela interface web, sem editar arquivos
- **Interface web embarcada** — React/Tailwind embutido no binário, zero dependências de assets externos
- **Multi-câmera** — número ilimitado de câmeras, cada uma com overrides individuais de codec, resolução e detecção
- **Análise de vídeo (YOLO)** — serviço opcional que analisa cada gravação com modelos YOLO; suporta fine-tuning com seus próprios snapshots anotados; compatível com GPU NVIDIA via docker-compose override
- **Binário único** — dependências: `ffmpeg` no sistema + SQLite embutido no binário

---

## Documentação

| | |
|---|---|
| [Instalação](docs/installation.md) | Script automático, Docker, Raspberry Pi, download manual |
| [Configuração](docs/configuration.md) | Referência completa do `camera.yaml` |
| [Câmeras](docs/cameras.md) | Adicionar, descobrir e configurar câmeras |
| [Detecção de movimento](docs/motion.md) | Threshold, zonas, buffer pré/pós-evento |
| [Armazenamento](docs/storage.md) | Retenção, limpeza e limites de disco |
| [Usuários](docs/users.md) | Papéis, permissões e autenticação |
| [Análise de vídeo](docs/analysis.md) | Serviço YOLO, GPU, fine-tuning e modelos disponíveis |

---

## Instalação rápida (Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/jacksonbicalho/camera/master/scripts/install.sh -o /tmp/camera-install.sh
sudo bash /tmp/camera-install.sh
```

O script detecta a arquitetura (`amd64`, `arm64`, `arm`), baixa o binário da última release, executa o wizard de configuração interativo e registra um serviço systemd.

> **Por que baixar antes de executar?**
> Com `curl | sudo bash` o `stdin` do bash fica ocupado com o pipe do curl — o wizard não consegue ler o teclado. Salvar o script em um arquivo e executar separadamente mantém o `stdin` conectado ao terminal real.

**Alternativa via git clone:**

```bash
git clone --depth 1 https://github.com/jacksonbicalho/camera.git /tmp/camera-install
sudo bash /tmp/camera-install/scripts/install.sh
rm -rf /tmp/camera-install
```

**Caminhos customizáveis:**

```bash
curl -fsSL https://raw.githubusercontent.com/jacksonbicalho/camera/master/scripts/install.sh -o /tmp/camera-install.sh
sudo bash /tmp/camera-install.sh \
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
# Remove binário e serviço (mantém config e dados)
sudo camera-uninstall

# Remove também a configuração (/etc/camera/)
sudo camera-uninstall --remove-config

# Remove também gravações, banco de dados e segmentos HLS
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

O arquivo `camera.yaml` é um **bootstrap mínimo** — contém apenas o necessário para o sistema iniciar. Toda configuração de câmeras, motion e zonas é feita via interface web e persistida no banco.

Use o wizard interativo para gerar o arquivo:

```bash
camera init
# ou para um caminho específico:
camera init --output /etc/camera/camera.yaml
```

Ou copie e edite o exemplo manualmente:

```bash
cp camera.yaml.example camera.yaml
```

```yaml
debug: false
timezone: America/Sao_Paulo

db_path: /var/camera/data/camera.db

server:
  port: 8080
  segments_path: /var/camera/data/hls

storage:
  path: /var/camera/data/recordings

admin:
  username: admin
  password: changeme   # obrigatório trocar no primeiro login
```

### Referência de campos

| Campo | Padrão | Descrição |
|---|---|---|
| `db_path` | — | Caminho do banco SQLite |
| `timezone` | `UTC` | Fuso horário para logs e nomes de arquivo |
| `server.port` | — | Porta HTTP da interface web |
| `storage.path` | — | Diretório raiz das gravações |
| `admin.username` | `admin` | Usuário administrador criado na primeira execução |
| `admin.password` | — | Senha inicial; obrigatório trocar no primeiro login |

Variáveis de ambiente sobrescrevem campos específicos:

| Variável | Campo |
|---|---|
| `CAMERA_TIMEZONE` | `timezone` |
| `CAMERA_SERVER_JWT_SECRET` | segredo JWT fixo (vazio = gerado aleatoriamente a cada boot) |

---

## Compilar a partir do código

**Requisitos:** Go 1.25+, Docker, ffmpeg

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

> **Branch `master` protegido.** Todo código entra via Pull Request. Crie uma branch descritiva (`feat/…`, `fix/…`), abra o PR e aguarde os checks de CI (`go test ./...`, `yarn lint`, `yarn test`, `yarn build`) passarem antes do merge.

### Wizard de configuração

O subcomando `init` gera o arquivo de bootstrap (`camera.yaml`) interativamente. Câmeras são adicionadas depois via interface web.

```bash
./camera init
./camera init --output /etc/camera/camera.yaml
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
          ├── {HHmmss}.mp4                ← chunk gravado
          └── {YYYYMMDDHHmmss}_motion.jpg ← snapshot anotado do evento

Eventos de movimento são persistidos no banco SQLite (tabela `motion_events`).
```

O servidor emite dois endpoints SSE por câmera:
- `/api/cameras/{id}/motion/live` — eventos acima do limiar
- `/api/cameras/{id}/motion/scores` — score bruto por frame (tempo real)

### Pacotes internos

| Pacote | Responsabilidade |
|---|---|
| `internal/recorder` | Grava RTSP → MP4 em chunks |
| `internal/streaming` | Gera playlist HLS ao vivo |
| `internal/motion` | Detecta movimento via ffmpeg pipe raw; persiste eventos no banco e em NDJSON; salva snapshot JPEG anotado por evento |
| `internal/storage` | Sincroniza MP4s do disco para o banco (`recordings`) e apaga chunks com retenção diferenciada por categoria via SQL |
| `internal/db` | Acesso ao SQLite; executa migrations na inicialização; tabelas: cameras, users, recordings, motion_events, settings |
| `internal/server` | HTTP + JWT + API REST + SPA embarcada; bloqueia endpoints se `must_change_password=true` |
| `internal/config` | Lê o bootstrap `camera.yaml` e aplica overrides de env vars |
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

O script lê os commits convencionais desde a última tag, calcula o bump semântico e cria uma tag `vX.Y.Z-dev`. O push da tag dispara o GitHub Actions que compila e publica os binários automaticamente.
