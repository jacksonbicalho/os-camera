#!/bin/sh
set -e

REPO="jacksonbicalho/camera"
BINARY_NAME="camera"

# Defaults — sobrescritos por flags ou por install.conf (na desinstalação)
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/camera"
DATA_DIR="/data/recordings"
SERVICE_NAME="camera"

# Caminhos derivados (recalculados após parse de flags)
STATE_DIR="/var/lib/camera"
STATE_FILE="${STATE_DIR}/install.conf"

# --- helpers ---

info()  { printf '\033[1;34m==> \033[0m%s\n' "$*"; }
ok()    { printf '\033[1;32m ok \033[0m%s\n' "$*"; }
err()   { printf '\033[1;31mERR \033[0m%s\n' "$*" >&2; exit 1; }
warn()  { printf '\033[1;33mWRN \033[0m%s\n' "$*" >&2; }

require_root() {
    if [ "$(id -u)" -ne 0 ]; then
        err "Este script precisa ser executado como root. Use: sudo $0 $*"
    fi
}

require_cmd() {
    command -v "$1" >/dev/null 2>&1 || err "Comando não encontrado: $1. Instale-o e tente novamente."
}

detect_arch() {
    arch="$(uname -m)"
    case "$arch" in
        x86_64)          echo "linux-amd64" ;;
        aarch64|arm64)   echo "linux-arm64" ;;
        armv7l|armv6l)   echo "linux-arm"   ;;
        *) err "Arquitetura não suportada: $arch" ;;
    esac
}

latest_version() {
    curl -fsSL "https://api.github.com/repos/${REPO}/releases" \
        | grep '"tag_name"' \
        | head -1 \
        | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/'
}

derived_paths() {
    SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
    CONFIG_FILE="${CONFIG_DIR}/${BINARY_NAME}.yaml"
    SHARE_DIR="${INSTALL_DIR%/bin}/share/${BINARY_NAME}"
    UNINSTALL_BIN="${INSTALL_DIR}/${BINARY_NAME}-uninstall"
}

# --- install ---

do_install() {
    require_cmd curl
    require_cmd systemctl

    derived_paths

    info "Detectando sistema..."
    os="$(uname -s)"
    [ "$os" = "Linux" ] || err "Sistema operacional não suportado: $os (somente Linux)"
    target="$(detect_arch)"
    ok "Alvo: $target"

    info "Obtendo versão mais recente..."
    version="$(latest_version)"
    [ -n "$version" ] || err "Não foi possível obter a versão mais recente do GitHub."
    ok "Versão: $version"

    download_url="https://github.com/${REPO}/releases/download/${version}/${BINARY_NAME}-${target}"
    info "Baixando $download_url ..."
    tmp="$(mktemp)"
    curl -fsSL --progress-bar "$download_url" -o "$tmp"
    chmod +x "$tmp"

    info "Instalando em ${INSTALL_DIR}/${BINARY_NAME} ..."
    mkdir -p "$INSTALL_DIR"
    mv "$tmp" "${INSTALL_DIR}/${BINARY_NAME}"
    ok "Binário instalado"

    info "Criando diretório de configuração ${CONFIG_DIR} ..."
    mkdir -p "$CONFIG_DIR"

    if [ ! -f "$CONFIG_FILE" ]; then
        info "Gerando configuração mínima em ${CONFIG_FILE} ..."
        cat > "$CONFIG_FILE" <<'YAML'
# Configuração gerada pelo instalador. Edite conforme necessário.
# Documentação: https://github.com/jacksonbicalho/camera

timezone: UTC   # ex: America/Sao_Paulo

server:
  port: 8080
  segments_path: /tmp/hls
  username: admin
  password: troque-esta-senha   # ALTERE antes de expor na rede

storage:
  path: /data/recordings
  retention:
    with_motion_minutes: 10080    # 7 dias
    without_motion_minutes: 1440  # 1 dia
  max_size_gb: 20

motion:
  enabled: false   # altere para true para ativar detecção de movimento
  threshold: 0.02
  fps: 2
  cooldown_seconds: 30

cameras: []   # adicione suas câmeras aqui, ex:
#   - id: entrada
#     rtsp_url: rtsp://192.168.1.10:554/stream
YAML
        ok "Config criado"
    else
        warn "Config já existe — não foi sobrescrito: ${CONFIG_FILE}"
    fi

    info "Criando serviço systemd ${SERVICE_FILE} ..."
    cat > "$SERVICE_FILE" <<UNIT
[Unit]
Description=Camera monitoring service
After=network.target
StartLimitIntervalSec=0

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/${BINARY_NAME} --config ${CONFIG_FILE}
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
UNIT

    systemctl daemon-reload
    systemctl enable --now "$SERVICE_NAME"
    ok "Serviço iniciado"

    # --- salvar estado e instalar desinstalador ---

    info "Salvando estado da instalação em ${STATE_FILE} ..."
    mkdir -p "$STATE_DIR"
    cat > "$STATE_FILE" <<CONF
INSTALL_DIR=${INSTALL_DIR}
CONFIG_DIR=${CONFIG_DIR}
DATA_DIR=${DATA_DIR}
SERVICE_NAME=${SERVICE_NAME}
SERVICE_FILE=${SERVICE_FILE}
CONFIG_FILE=${CONFIG_FILE}
SHARE_DIR=${SHARE_DIR}
UNINSTALL_BIN=${UNINSTALL_BIN}
CONF
    ok "Estado salvo"

    info "Instalando desinstalador em ${SHARE_DIR} ..."
    mkdir -p "$SHARE_DIR"

    # Se o script foi executado via "curl | bash", $0 é /dev/stdin — baixar uma cópia
    if [ -f "$0" ] && [ "$0" != "/dev/stdin" ]; then
        cp "$0" "${SHARE_DIR}/install.sh"
    else
        script_url="https://raw.githubusercontent.com/${REPO}/master/scripts/install.sh"
        info "Baixando cópia do instalador de ${script_url} ..."
        curl -fsSL "$script_url" -o "${SHARE_DIR}/install.sh"
    fi
    chmod +x "${SHARE_DIR}/install.sh"

    cat > "$UNINSTALL_BIN" <<WRAPPER
#!/bin/sh
exec "${SHARE_DIR}/install.sh" --uninstall "\$@"
WRAPPER
    chmod +x "$UNINSTALL_BIN"
    ok "Desinstalador disponível: ${BINARY_NAME}-uninstall"

    printf '\n'
    info "Instalação concluída!"
    printf '  Editar config:  %s\n'               "$CONFIG_FILE"
    printf '  Reiniciar:      systemctl restart %s\n' "$SERVICE_NAME"
    printf '  Ver logs:       journalctl -u %s -f\n'  "$SERVICE_NAME"
    printf '  Status:         systemctl status %s\n'  "$SERVICE_NAME"
    printf '  Desinstalar:    %s-uninstall\n'         "$BINARY_NAME"
    printf '\n'
    warn "Lembre-se de editar ${CONFIG_FILE} com suas câmeras e senha antes de usar."
}

# --- uninstall ---

do_uninstall() {
    remove_config=0
    remove_data=0

    for arg in "$@"; do
        case "$arg" in
            --remove-config) remove_config=1 ;;
            --remove-data)   remove_data=1   ;;
            --uninstall)     ;;  # ignorado aqui, já processado no entrypoint
        esac
    done

    derived_paths

    require_cmd systemctl

    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        info "Parando serviço ${SERVICE_NAME} ..."
        systemctl stop "$SERVICE_NAME"
    fi

    if systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
        info "Desabilitando serviço ${SERVICE_NAME} ..."
        systemctl disable "$SERVICE_NAME"
    fi

    if [ -f "$SERVICE_FILE" ]; then
        info "Removendo ${SERVICE_FILE} ..."
        rm -f "$SERVICE_FILE"
        systemctl daemon-reload
        ok "Serviço removido"
    fi

    if [ -f "${INSTALL_DIR}/${BINARY_NAME}" ]; then
        info "Removendo ${INSTALL_DIR}/${BINARY_NAME} ..."
        rm -f "${INSTALL_DIR}/${BINARY_NAME}"
        ok "Binário removido"
    fi

    if [ -f "$UNINSTALL_BIN" ]; then
        info "Removendo ${UNINSTALL_BIN} ..."
        rm -f "$UNINSTALL_BIN"
    fi

    if [ -d "$SHARE_DIR" ]; then
        info "Removendo ${SHARE_DIR} ..."
        rm -rf "$SHARE_DIR"
        ok "Arquivos do instalador removidos"
    fi

    if [ -f "$STATE_FILE" ]; then
        info "Removendo ${STATE_FILE} ..."
        rm -f "$STATE_FILE"
        rmdir "$STATE_DIR" 2>/dev/null || true
    fi

    if [ "$remove_config" = "1" ] && [ -d "$CONFIG_DIR" ]; then
        info "Removendo ${CONFIG_DIR} ..."
        rm -rf "$CONFIG_DIR"
        ok "Configuração removida"
    fi

    if [ "$remove_data" = "1" ] && [ -d "$DATA_DIR" ]; then
        info "Removendo ${DATA_DIR} ..."
        rm -rf "$DATA_DIR"
        ok "Dados removidos"
    fi

    printf '\n'
    ok "Desinstalação concluída."
    if [ "$remove_config" = "0" ] && [ -d "$CONFIG_DIR" ]; then
        printf '  Config mantido: %s\n' "$CONFIG_DIR"
        printf '  Para remover:   %s-uninstall --remove-config\n' "$BINARY_NAME"
    fi
    if [ "$remove_data" = "0" ] && [ -d "$DATA_DIR" ]; then
        printf '  Dados mantidos: %s\n' "$DATA_DIR"
        printf '  Para remover:   %s-uninstall --remove-data\n' "$BINARY_NAME"
    fi
}

# --- entrypoint ---

UNINSTALL=0

# Parse flags — aceita qualquer ordem
REST=""
for arg in "$@"; do
    case "$arg" in
        --uninstall)     UNINSTALL=1 ;;
        --remove-config) ;;  # repassado para do_uninstall via $@
        --remove-data)   ;;  # idem
        --install-dir=*) INSTALL_DIR="${arg#--install-dir=}" ;;
        --config-dir=*)  CONFIG_DIR="${arg#--config-dir=}"   ;;
        --data-dir=*)    DATA_DIR="${arg#--data-dir=}"       ;;
        --service-name=*)SERVICE_NAME="${arg#--service-name=}";;
        --install-dir)   REST="install-dir" ;;
        --config-dir)    REST="config-dir"  ;;
        --data-dir)      REST="data-dir"    ;;
        --service-name)  REST="service-name";;
        --help|-h)
            printf 'Uso:\n'
            printf '  instalar:   curl -fsSL <url>/install.sh | sudo bash\n'
            printf '  opções:     --install-dir=DIR  --config-dir=DIR  --data-dir=DIR  --service-name=NAME\n'
            printf '  desinstalar (local, sem internet):\n'
            printf '              camera-uninstall [--remove-config] [--remove-data]\n'
            exit 0
            ;;
        *)
            if [ -n "$REST" ]; then
                case "$REST" in
                    install-dir)  INSTALL_DIR="$arg" ;;
                    config-dir)   CONFIG_DIR="$arg"  ;;
                    data-dir)     DATA_DIR="$arg"    ;;
                    service-name) SERVICE_NAME="$arg";;
                esac
                REST=""
            else
                err "Argumento desconhecido: $arg"
            fi
            ;;
    esac
done

if [ "$UNINSTALL" = "1" ]; then
    require_root "$@"
    # Carregar estado salvo para que os caminhos corretos sejam usados
    if [ -f "$STATE_FILE" ]; then
        # shellcheck disable=SC1090
        . "$STATE_FILE"
    else
        warn "Arquivo de estado não encontrado (${STATE_FILE}). Usando caminhos padrão."
    fi
    do_uninstall "$@"
else
    require_root "$@"
    do_install
fi
