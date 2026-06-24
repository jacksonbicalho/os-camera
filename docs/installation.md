# Instalação

## Docker (recomendado)

A forma mais simples e portável: a **mesma imagem** roda em x86-64, Raspberry Pi
(arm64) e ARMv7 (32-bit). Não precisa de `install.sh` nem systemd — o container roda o
binário direto e o **Docker é o supervisor** (restart, logs).

A imagem é publicada no GHCR: `ghcr.io/jacksonbicalho/os-camera` (tags `:latest` e
`:vX.Y.Z`).

```bash
# 1) Gerar a configuração (a partir do exemplo, ou com o wizard — ver abaixo)
cp camera.yaml.example camera.yaml   # e edite

# 2) Subir
docker run -d --name camera \
  --network host \
  -v "$PWD/camera.yaml:/app/camera.yaml:ro" \
  -v "$PWD/storage:/data" \
  --restart unless-stopped \
  ghcr.io/jacksonbicalho/os-camera:latest
```

> **`--network host`** é necessário para a descoberta de câmeras (ONVIF multicast + scan
> de porta) funcionar na LAN real.

### docker compose

```yaml
services:
  camera:
    image: ghcr.io/jacksonbicalho/os-camera:latest
    network_mode: host
    volumes:
      - ./camera.yaml:/app/camera.yaml:ro
      - ./storage:/data
    restart: unless-stopped
```

```bash
docker compose up -d
```

### Gerar o `camera.yaml` com o wizard (opcional)

```bash
docker run --rm -it -v "$PWD:/cfg" \
  ghcr.io/jacksonbicalho/os-camera:latest ./camera init --output /cfg/camera.yaml
```

> Para **buildar a imagem localmente** em vez de baixar do GHCR:
> `docker compose --profile production up -d --build` (usa o `Dockerfile` do repo).

---

## Linux (script automático — systemd)

Para um host Linux "bare-metal" (com systemd). O script detecta a arquitetura (`amd64`,
`arm64`, `arm`), baixa o binário da última release, executa o wizard de configuração e
registra um serviço systemd.

> Em **container** ou **Termux** não use este caminho — não há systemd. Use a imagem
> Docker (acima) ou o download manual (abaixo).

```bash
curl -fsSL https://raw.githubusercontent.com/jacksonbicalho/os-camera/master/scripts/install.sh -o /tmp/camera-install.sh
sudo bash /tmp/camera-install.sh
```

> **Por que salvar antes de executar?**
> Com `curl | sudo bash` o stdin do bash fica ocupado com o pipe — o wizard não consegue ler o teclado. Salvando em arquivo o stdin continua conectado ao terminal.

### Caminhos personalizados

```bash
sudo bash /tmp/camera-install.sh \
  --install-dir /usr/local/bin \
  --config-dir  /etc/camera \
  --data-dir    /data/recordings \
  --service-name camera
```

### Comandos pós-instalação

```bash
camera --version
sudo systemctl status camera
sudo systemctl restart camera
sudo journalctl -u camera -f
```

### Desinstalar

```bash
sudo camera-uninstall                          # remove binário e serviço
sudo camera-uninstall --remove-config          # + configuração
sudo camera-uninstall --remove-data            # + gravações e banco
sudo camera-uninstall --remove-config --remove-data  # tudo
```

---

## Raspberry Pi

A imagem **Docker** (recomendado) cobre os dois casos — RPi 3/4/5 (64-bit) usa `arm64` e
RPi 2/3 em 32-bit usa `arm/v7`, ambos na mesma tag `:latest`. Como alternativa
bare-metal, o `install.sh` baixa o binário certo (`linux-arm64` ou `linux-arm`).

**Requisito (instalação bare-metal):** ffmpeg no sistema.

```bash
sudo apt update && sudo apt install -y ffmpeg
```

**Dica de desempenho:** ative o hardware decoding para aliviar a CPU. Edite `camera.yaml`
e configure as câmeras com `hls_video_mode: copy` quando o stream já for H.264, evitando
retranscodificação.

---

## Download manual (binário)

Sem Docker e sem systemd (ex.: Termux/Android, ambientes restritos): baixe o binário em
[github.com/jacksonbicalho/os-camera/releases](https://github.com/jacksonbicalho/os-camera/releases).

| Plataforma | Arquivo |
|---|---|
| Linux x86-64 | `camera-linux-amd64` |
| Linux ARM64 (RPi 3/4/5 64-bit) | `camera-linux-arm64` |
| Linux ARMv7 (RPi 2/3 32-bit) | `camera-linux-arm` |
| Windows x86-64 | `camera-windows-amd64.exe` |

```bash
chmod +x camera-linux-amd64
./camera-linux-amd64 init           # wizard de configuração
./camera-linux-amd64 --config camera.yaml
```

O binário é estático (não depende de libc), mas o **ffmpeg precisa estar instalado** no
sistema (ex.: `apt install ffmpeg`, ou `pkg install ffmpeg` no Termux).

O wizard pergunta o destino dos logs (`stdout` ou `file`). Ao escolher `file`, ele
também pergunta o diretório e os parâmetros de **rotação**: tamanho de rotação
(`max_size_mb`), retenção (`max_age_days`), número máximo de arquivos
(`max_backups`) e compressão gzip (`compress`). Em `stdout` a rotação não se aplica
— quem cuida disso é o supervisor de processo (Docker/journald/systemd).

---

## Compilar a partir do código

**Requisitos:** Go 1.25+, Docker, ffmpeg

```bash
git clone https://github.com/jacksonbicalho/os-camera.git
cd camera

make build          # binário local em ./dist/
make linux-arm64    # Raspberry Pi 3/4/5
make all            # todos os alvos (linux-amd64, arm64, arm, windows-amd64)
```

O frontend é compilado automaticamente via Docker (node:20-alpine). Não é necessário ter Node.js instalado localmente.

---

## Primeiro acesso

Após iniciar o servidor, acesse `http://localhost:8080` (ou o IP/porta configurado).

1. Faça login com o usuário `admin` e a senha definida no `camera.yaml`
2. O sistema solicitará troca de senha obrigatória no primeiro login
3. Após a troca, acesse **Configurações → Câmeras** para adicionar a primeira câmera

Ver também: [Usuários](users.md) | [Câmeras](cameras.md)
