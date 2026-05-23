#!/bin/sh
set -e

REPO="jacksonbicalho/camera"
BINARY_NAME="camera"

# Defaults — sobrescritos por flags ou por install.conf (na desinstalação)
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/camera"
DATA_DIR="/var/camera/data/recordings"
SEGMENTS_DIR="/var/camera/data/hls"
SERVICE_NAME="camera"

# Caminhos derivados (recalculados após parse de flags)
STATE_DIR="/var/camera"
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

    config_ready=0
    if [ -f "$CONFIG_FILE" ]; then
        warn "Config já existe — não foi sobrescrito: ${CONFIG_FILE}"
        config_ready=1
    else
        info "Iniciando assistente de configuração..."
        printf '\n'
        # Tenta rodar o wizard interativamente.
        # curl | bash não tem stdin TTY, mas /dev/tty permite leitura do terminal mesmo assim.
        if [ -t 0 ]; then
            "${INSTALL_DIR}/${BINARY_NAME}" init --output "${CONFIG_FILE}" && config_ready=1 || true
        elif [ -e /dev/tty ]; then
            "${INSTALL_DIR}/${BINARY_NAME}" init --output "${CONFIG_FILE}" </dev/tty && config_ready=1 || true
        fi
        [ "$config_ready" = "0" ] && warn "Wizard cancelado ou indisponível. Gerando config mínimo em ${CONFIG_FILE}."
    fi

    if [ "$config_ready" = "0" ]; then
        # Fallback: gerar placeholder com esquema atual
        cat > "$CONFIG_FILE" <<YAML
# Configuração gerada pelo instalador. Edite conforme necessário.
# Execute: camera init --output <este arquivo>
# Documentação: https://github.com/jacksonbicalho/camera

debug: false
timezone: UTC   # ex: America/Sao_Paulo

db_path: ${STATE_DIR}/data/camera.db  # banco SQLite (criado automaticamente)

log:
  output: stdout   # stdout | file
  path:            # diretório quando output: file

server:
  port: 8080
  segments_path: ${SEGMENTS_DIR}
  hls_dvr_seconds: 0   # 0 = desabilitado
  jwt_secret: ""       # vazio = gerado aleatoriamente a cada boot

storage:
  path: ${DATA_DIR}
  retention:
    with_motion_minutes: 10080    # 7 dias  (0 = nunca apaga)
    without_motion_minutes: 1440  # 1 dia   (0 = desabilitado)
  interval_minutes: 60
  max_size_gb: 20
  warn_percent: 90

admin:
  username: admin
  password: changeme   # OBRIGATÓRIO trocar no primeiro login
YAML
        ok "Config mínimo criado em ${CONFIG_FILE}"
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
    systemctl enable "$SERVICE_NAME"

    if [ "$config_ready" = "1" ]; then
        systemctl start "$SERVICE_NAME"
        ok "Serviço iniciado"
    else
        warn "Serviço habilitado mas NÃO iniciado — configure ${CONFIG_FILE} e execute: systemctl start ${SERVICE_NAME}"
    fi

    # --- salvar estado e instalar desinstalador ---

    info "Salvando estado da instalação em ${STATE_FILE} ..."
    mkdir -p "$STATE_DIR"
    cat > "$STATE_FILE" <<CONF
INSTALL_DIR=${INSTALL_DIR}
CONFIG_DIR=${CONFIG_DIR}
DATA_DIR=${DATA_DIR}
SEGMENTS_DIR=${SEGMENTS_DIR}
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
    if [ "$config_ready" = "0" ]; then
        printf '  Configurar:     %s init --output %s\n' "$BINARY_NAME" "$CONFIG_FILE"
        printf '  Iniciar:        systemctl start %s\n'    "$SERVICE_NAME"
    fi
    printf '  Editar config:  %s\n'                    "$CONFIG_FILE"
    printf '  Reiniciar:      systemctl restart %s\n'  "$SERVICE_NAME"
    printf '  Ver logs:       journalctl -u %s -f\n'   "$SERVICE_NAME"
    printf '  Status:         systemctl status %s\n'   "$SERVICE_NAME"
    printf '  Desinstalar:    %s-uninstall\n'          "$BINARY_NAME"
    printf '\n'
    if [ "$config_ready" = "0" ]; then
        warn "Execute '${BINARY_NAME} init --output ${CONFIG_FILE}' para configurar suas câmeras antes de iniciar o serviço."
    fi
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
        --install-dir=*)   INSTALL_DIR="${arg#--install-dir=}"   ;;
        --config-dir=*)    CONFIG_DIR="${arg#--config-dir=}"     ;;
        --data-dir=*)      DATA_DIR="${arg#--data-dir=}"         ;;
        --segments-dir=*)  SEGMENTS_DIR="${arg#--segments-dir=}" ;;
        --service-name=*)  SERVICE_NAME="${arg#--service-name=}" ;;
        --install-dir)     REST="install-dir"   ;;
        --config-dir)      REST="config-dir"    ;;
        --data-dir)        REST="data-dir"      ;;
        --segments-dir)    REST="segments-dir"  ;;
        --service-name)    REST="service-name"  ;;
        --help|-h)
            printf 'Uso:\n'
            printf '  instalar:   curl -fsSL <url>/install.sh -o /tmp/install.sh && sudo bash /tmp/install.sh\n'
            printf '  alternativa: git clone --depth 1 <repo> /tmp/cam && sudo bash /tmp/cam/scripts/install.sh\n'
            printf '  opções:     --install-dir=DIR  --config-dir=DIR  --data-dir=DIR  --segments-dir=DIR  --service-name=NAME\n'
            printf '  desinstalar (local, sem internet):\n'
            printf '              camera-uninstall [--remove-config] [--remove-data]\n'
            exit 0
            ;;
        *)
            if [ -n "$REST" ]; then
                case "$REST" in
                    install-dir)   INSTALL_DIR="$arg"   ;;
                    config-dir)    CONFIG_DIR="$arg"    ;;
                    data-dir)      DATA_DIR="$arg"      ;;
                    segments-dir)  SEGMENTS_DIR="$arg"  ;;
                    service-name)  SERVICE_NAME="$arg"  ;;
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
