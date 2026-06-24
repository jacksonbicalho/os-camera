#!/bin/sh
set -e

REPO="jacksonbicalho/os-camera"
BINARY_NAME="camera"

# Caminhos: vazio = resolvido por modo (sistema vs usuário) após o parse das flags.
INSTALL_DIR=""
CONFIG_DIR=""
DATA_DIR=""
SEGMENTS_DIR=""
STATE_DIR=""
SERVICE_NAME="camera"

# Modos (flags)
USER_MODE=0       # --user (ou auto quando não-root e sem paths de sistema explícitos)
NO_SERVICE=0      # --no-service (não cria serviço)
SKIP_DEPS=0       # --skip-deps (não instala ffmpeg)
LOCAL_BINARY=""   # --binary=PATH (instala de arquivo local, sem download)
SERVICE_MODE=""   # resolvido: "systemd" | "none"

# --- helpers ---

info()  { printf '\033[1;34m==> \033[0m%s\n' "$*"; }
ok()    { printf '\033[1;32m ok \033[0m%s\n' "$*"; }
err()   { printf '\033[1;31mERR \033[0m%s\n' "$*" >&2; exit 1; }
warn()  { printf '\033[1;33mWRN \033[0m%s\n' "$*" >&2; }

is_root()      { [ "$(id -u)" -eq 0 ]; }
have_systemd() { command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; }

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

# Primeiro gerenciador de pacotes encontrado (vazio se nenhum).
detect_pm() {
    for pm in apt-get dnf yum pacman zypper apk pkg; do
        if command -v "$pm" >/dev/null 2>&1; then echo "$pm"; return; fi
    done
}

# Comando (texto) para instalar ffmpeg num PM — usado nas instruções quando não podemos
# instalar sozinhos. `pkg` (Termux) não usa sudo; os demais sim.
ffmpeg_hint() {
    case "$1" in
        apt-get) echo "sudo apt install ffmpeg" ;;
        dnf)     echo "sudo dnf install ffmpeg" ;;
        yum)     echo "sudo yum install ffmpeg" ;;
        pacman)  echo "sudo pacman -S ffmpeg" ;;
        zypper)  echo "sudo zypper install ffmpeg" ;;
        apk)     echo "sudo apk add ffmpeg" ;;
        pkg)     echo "pkg install ffmpeg" ;;
        *)       echo "instale o pacote 'ffmpeg' pelo gerenciador do seu sistema" ;;
    esac
}

install_ffmpeg() {
    case "$1" in
        apt-get) apt-get update && apt-get install -y ffmpeg ;;
        dnf)     dnf install -y ffmpeg ;;
        yum)     yum install -y ffmpeg ;;
        pacman)  pacman -Sy --noconfirm ffmpeg ;;
        zypper)  zypper install -y ffmpeg ;;
        apk)     apk add --no-cache ffmpeg ;;
        pkg)     pkg install -y ffmpeg ;;
        *)       return 1 ;;
    esac
}

# Garante ffmpeg/ffprobe: instala quando possível, senão instrui e aborta.
ensure_ffmpeg() {
    if command -v ffmpeg >/dev/null 2>&1 && command -v ffprobe >/dev/null 2>&1; then
        return
    fi
    if [ "$SKIP_DEPS" = "1" ]; then
        warn "ffmpeg/ffprobe ausentes — --skip-deps: pulando instalação (a app não roda sem ffmpeg)."
        return
    fi
    pm="$(detect_pm)"
    # Pode instalar: Termux (pkg, sem root) ou PM de sistema sendo root.
    if [ "$pm" = "pkg" ] || { [ -n "$pm" ] && is_root; }; then
        info "ffmpeg ausente — instalando via ${pm} ..."
        install_ffmpeg "$pm" || err "Falha ao instalar ffmpeg via ${pm}. Instale manualmente: $(ffmpeg_hint "$pm")"
        command -v ffmpeg >/dev/null 2>&1 && command -v ffprobe >/dev/null 2>&1 \
            || err "ffmpeg instalado mas ffprobe não foi encontrado. Verifique o pacote."
        ok "ffmpeg instalado"
        return
    fi
    if [ -n "$pm" ]; then
        err "ffmpeg/ffprobe ausentes. Instale e rode de novo: $(ffmpeg_hint "$pm")"
    fi
    err "ffmpeg/ffprobe ausentes e nenhum gerenciador de pacotes detectado. Instale 'ffmpeg' manualmente."
}

# Resolve os caminhos conforme o modo (sistema vs usuário). Flags explícitas têm
# precedência; o resto recebe o default do modo.
resolve_mode() {
    if [ "$USER_MODE" = "0" ] && ! is_root; then
        # Sem root e sem nenhum path de sistema explícito → modo usuário automático.
        if [ -z "$INSTALL_DIR" ] && [ -z "$CONFIG_DIR" ] && [ -z "$DATA_DIR" ] \
            && [ -z "$SEGMENTS_DIR" ] && [ -z "$STATE_DIR" ]; then
            USER_MODE=1
            info "Sem root detectado → instalando em modo usuário (~/.local)."
        fi
    fi

    if [ "$USER_MODE" = "1" ]; then
        : "${INSTALL_DIR:=$HOME/.local/bin}"
        : "${CONFIG_DIR:=$HOME/.config/camera}"
        : "${STATE_DIR:=$HOME/.local/share/camera}"
        : "${DATA_DIR:=$STATE_DIR/data/recordings}"
        : "${SEGMENTS_DIR:=$STATE_DIR/data/hls}"
    else
        : "${INSTALL_DIR:=/usr/local/bin}"
        : "${CONFIG_DIR:=/etc/camera}"
        : "${STATE_DIR:=/var/camera}"
        : "${DATA_DIR:=/var/camera/data/recordings}"
        : "${SEGMENTS_DIR:=/var/camera/data/hls}"
    fi
}

resolve_service() {
    if [ "$NO_SERVICE" = "1" ] || [ "$USER_MODE" = "1" ] || ! have_systemd; then
        SERVICE_MODE="none"
    else
        SERVICE_MODE="systemd"
    fi
}

# Sem root: exige escrita nos diretórios-alvo e proíbe serviço systemd.
ensure_perms() {
    if is_root; then return; fi
    for d in "$INSTALL_DIR" "$CONFIG_DIR" "$STATE_DIR"; do
        parent="$d"
        while [ ! -d "$parent" ]; do parent="$(dirname "$parent")"; done
        [ -w "$parent" ] || err "Sem permissão de escrita em ${d}. Rode com sudo, ou use --user."
    done
    if [ "$SERVICE_MODE" = "systemd" ]; then
        err "Serviço systemd exige root. Rode com sudo, ou use --no-service / --user."
    fi
}

derived_paths() {
    SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
    CONFIG_FILE="${CONFIG_DIR}/${BINARY_NAME}.yaml"
    SHARE_DIR="${INSTALL_DIR%/bin}/share/${BINARY_NAME}"
    UNINSTALL_BIN="${INSTALL_DIR}/${BINARY_NAME}-uninstall"
    DB_PATH="${STATE_DIR}/data/camera.db"
    STATE_FILE="${STATE_DIR}/install.conf"
}

# Coloca o binário a instalar em $TMP_BIN (download da release ou cópia local).
acquire_binary() {
    if [ -n "$LOCAL_BINARY" ]; then
        [ -f "$LOCAL_BINARY" ] || err "Binário local não encontrado: ${LOCAL_BINARY}"
        info "Usando binário local ${LOCAL_BINARY} ..."
        TMP_BIN="$(mktemp)"
        cp "$LOCAL_BINARY" "$TMP_BIN"
    else
        require_cmd curl
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
        TMP_BIN="$(mktemp)"
        curl -fsSL --progress-bar "$download_url" -o "$TMP_BIN"
    fi
    chmod +x "$TMP_BIN"
}

# --- install ---

do_install() {
    resolve_mode
    resolve_service
    ensure_perms
    [ "$SERVICE_MODE" = "systemd" ] && require_cmd systemctl
    ensure_ffmpeg

    derived_paths

    info "Detectando sistema..."
    acquire_binary

    info "Instalando em ${INSTALL_DIR}/${BINARY_NAME} ..."
    mkdir -p "$INSTALL_DIR"
    mv "$TMP_BIN" "${INSTALL_DIR}/${BINARY_NAME}"
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
        # curl | bash não tem stdin TTY, mas /dev/tty permite leitura do terminal mesmo assim.
        if [ -t 0 ]; then
            "${INSTALL_DIR}/${BINARY_NAME}" init --output "${CONFIG_FILE}" && config_ready=1 || true
        elif [ -e /dev/tty ]; then
            "${INSTALL_DIR}/${BINARY_NAME}" init --output "${CONFIG_FILE}" </dev/tty && config_ready=1 || true
        fi
        [ "$config_ready" = "0" ] && warn "Wizard cancelado ou indisponível. Gerando config mínimo em ${CONFIG_FILE}."
    fi

    if [ "$config_ready" = "0" ]; then
        cat > "$CONFIG_FILE" <<YAML
# Configuração gerada pelo instalador. Edite conforme necessário.
# Execute: camera init --output <este arquivo>
# Documentação: https://github.com/jacksonbicalho/os-camera

debug: false
timezone: UTC   # ex: America/Sao_Paulo

db_path: ${DB_PATH}  # banco SQLite (criado automaticamente)

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

    if [ "$SERVICE_MODE" = "systemd" ]; then
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
    else
        info "Sem serviço (modo --no-service/usuário ou sem systemd) — você roda o binário direto."
    fi

    # --- salvar estado e instalar desinstalador ---

    info "Salvando estado da instalação em ${STATE_FILE} ..."
    mkdir -p "$STATE_DIR"
    cat > "$STATE_FILE" <<CONF
INSTALL_DIR=${INSTALL_DIR}
CONFIG_DIR=${CONFIG_DIR}
DATA_DIR=${DATA_DIR}
SEGMENTS_DIR=${SEGMENTS_DIR}
DB_PATH=${DB_PATH}
SERVICE_NAME=${SERVICE_NAME}
SERVICE_FILE=${SERVICE_FILE}
SERVICE_MODE=${SERVICE_MODE}
USER_MODE=${USER_MODE}
CONFIG_FILE=${CONFIG_FILE}
SHARE_DIR=${SHARE_DIR}
UNINSTALL_BIN=${UNINSTALL_BIN}
CONF
    ok "Estado salvo"

    info "Instalando desinstalador em ${SHARE_DIR} ..."
    mkdir -p "$SHARE_DIR"
    if [ -f "$0" ] && [ "$0" != "/dev/stdin" ]; then
        cp "$0" "${SHARE_DIR}/install.sh"
    else
        script_url="https://raw.githubusercontent.com/${REPO}/master/scripts/install.sh"
        info "Baixando cópia do instalador de ${script_url} ..."
        require_cmd curl
        curl -fsSL "$script_url" -o "${SHARE_DIR}/install.sh"
    fi
    chmod +x "${SHARE_DIR}/install.sh"

    cat > "$UNINSTALL_BIN" <<WRAPPER
#!/bin/sh
exec "${SHARE_DIR}/install.sh" --uninstall "\$@"
WRAPPER
    chmod +x "$UNINSTALL_BIN"
    ok "Desinstalador disponível: ${UNINSTALL_BIN}"

    printf '\n'
    info "Instalação concluída!"
    printf '  Editar config:  %s\n' "$CONFIG_FILE"
    if [ "$SERVICE_MODE" = "systemd" ]; then
        [ "$config_ready" = "0" ] && printf '  Configurar:     %s init --output %s\n' "$BINARY_NAME" "$CONFIG_FILE"
        printf '  Iniciar:        systemctl start %s\n'   "$SERVICE_NAME"
        printf '  Reiniciar:      systemctl restart %s\n' "$SERVICE_NAME"
        printf '  Ver logs:       journalctl -u %s -f\n'  "$SERVICE_NAME"
        printf '  Status:         systemctl status %s\n'  "$SERVICE_NAME"
    else
        [ "$config_ready" = "0" ] && printf '  Configurar:     %s init --output %s\n' "${INSTALL_DIR}/${BINARY_NAME}" "$CONFIG_FILE"
        printf '  Rodar:          %s --config %s\n' "${INSTALL_DIR}/${BINARY_NAME}" "$CONFIG_FILE"
        printf '  (Dica: rode em background com nohup/tmux; autostart no Termux é tratado à parte.)\n'
        case ":$PATH:" in
            *":${INSTALL_DIR}:"*) ;;
            *) printf '  Atenção: %s não está no PATH — adicione ao seu shell rc.\n' "$INSTALL_DIR" ;;
        esac
    fi
    printf '  Desinstalar:    %s-uninstall\n' "$BINARY_NAME"
    printf '\n'
}

# --- uninstall ---

do_uninstall() {
    remove_config=0
    remove_data=0
    for arg in "$@"; do
        case "$arg" in
            --remove-config) remove_config=1 ;;
            --remove-data)   remove_data=1   ;;
            --uninstall)     ;;
        esac
    done

    [ -z "$SERVICE_MODE" ] && SERVICE_MODE="none"

    if [ "$SERVICE_MODE" = "systemd" ]; then
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
    fi

    if [ -f "${INSTALL_DIR}/${BINARY_NAME}" ]; then
        info "Removendo ${INSTALL_DIR}/${BINARY_NAME} ..."
        rm -f "${INSTALL_DIR}/${BINARY_NAME}"
        ok "Binário removido"
    fi

    [ -f "$UNINSTALL_BIN" ] && { info "Removendo ${UNINSTALL_BIN} ..."; rm -f "$UNINSTALL_BIN"; }

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

    if [ "$remove_data" = "1" ]; then
        [ -d "$DATA_DIR" ]     && { info "Removendo gravações ${DATA_DIR} ...";    rm -rf "$DATA_DIR";     ok "Gravações removidas"; }
        [ -d "$SEGMENTS_DIR" ] && { info "Removendo segmentos HLS ${SEGMENTS_DIR} ..."; rm -rf "$SEGMENTS_DIR"; ok "Segmentos HLS removidos"; }
        [ -f "$DB_PATH" ]      && { info "Removendo banco ${DB_PATH} ...";          rm -f "$DB_PATH";       ok "Banco de dados removido"; }
    fi

    printf '\n'
    ok "Desinstalação concluída."
    if [ "$remove_config" = "0" ] && [ -d "$CONFIG_DIR" ]; then
        printf '  Config mantido:  %s\n' "$CONFIG_DIR"
        printf '  Para remover:    %s-uninstall --remove-config\n' "$BINARY_NAME"
    fi
    if [ "$remove_data" = "0" ]; then
        data_kept=0
        [ -d "$DATA_DIR" ]     && data_kept=1
        [ -d "$SEGMENTS_DIR" ] && data_kept=1
        [ -f "$DB_PATH" ]      && data_kept=1
        if [ "$data_kept" = "1" ]; then
            printf '  Dados mantidos:  gravações, segmentos HLS e banco de dados\n'
            printf '  Para remover:    %s-uninstall --remove-data\n' "$BINARY_NAME"
        fi
    fi
}

# --- entrypoint ---

UNINSTALL=0
REST=""
for arg in "$@"; do
    case "$arg" in
        --uninstall)       UNINSTALL=1 ;;
        --remove-config)   ;;
        --remove-data)     ;;
        --user)            USER_MODE=1 ;;
        --no-service)      NO_SERVICE=1 ;;
        --skip-deps)       SKIP_DEPS=1 ;;
        --binary=*)        LOCAL_BINARY="${arg#--binary=}" ;;
        --install-dir=*)   INSTALL_DIR="${arg#--install-dir=}"   ;;
        --config-dir=*)    CONFIG_DIR="${arg#--config-dir=}"     ;;
        --data-dir=*)      DATA_DIR="${arg#--data-dir=}"         ;;
        --segments-dir=*)  SEGMENTS_DIR="${arg#--segments-dir=}" ;;
        --state-dir=*)     STATE_DIR="${arg#--state-dir=}"       ;;
        --service-name=*)  SERVICE_NAME="${arg#--service-name=}" ;;
        --binary)          REST="binary"       ;;
        --install-dir)     REST="install-dir"  ;;
        --config-dir)      REST="config-dir"   ;;
        --data-dir)        REST="data-dir"     ;;
        --segments-dir)    REST="segments-dir" ;;
        --state-dir)       REST="state-dir"    ;;
        --service-name)    REST="service-name" ;;
        --help|-h)
            cat <<USAGE
Uso: install.sh [opções]

Instala em modo SISTEMA (root, serviço systemd) por padrão. Sem root, cai em modo
USUÁRIO automaticamente (~/.local, sem serviço).

Opções:
  --user                Instala no diretório do usuário (~/.local/bin, ~/.config/camera),
                        sem root e sem serviço.
  --no-service          Não cria serviço (mesmo com systemd) — você roda o binário.
  --skip-deps           Não tenta instalar o ffmpeg.
  --binary=ARQUIVO      Instala a partir de um binário local (sem download / offline).
  --install-dir=DIR     Diretório do binário.
  --config-dir=DIR      Diretório da configuração.
  --data-dir=DIR        Diretório das gravações.
  --segments-dir=DIR    Diretório dos segmentos HLS.
  --state-dir=DIR       Diretório de estado/banco.
  --service-name=NOME   Nome do serviço systemd.

Desinstalar (local, sem internet):
  camera-uninstall [--remove-config] [--remove-data]
USAGE
            exit 0
            ;;
        *)
            if [ -n "$REST" ]; then
                case "$REST" in
                    binary)        LOCAL_BINARY="$arg" ;;
                    install-dir)   INSTALL_DIR="$arg"  ;;
                    config-dir)    CONFIG_DIR="$arg"   ;;
                    data-dir)      DATA_DIR="$arg"     ;;
                    segments-dir)  SEGMENTS_DIR="$arg" ;;
                    state-dir)     STATE_DIR="$arg"    ;;
                    service-name)  SERVICE_NAME="$arg" ;;
                esac
                REST=""
            else
                err "Argumento desconhecido: $arg"
            fi
            ;;
    esac
done

if [ "$UNINSTALL" = "1" ]; then
    # Carrega o estado salvo (paths + SERVICE_MODE) antes de decidir permissões.
    resolve_mode
    derived_paths
    if [ -f "$STATE_FILE" ]; then
        # shellcheck disable=SC1090
        . "$STATE_FILE"
    else
        warn "Arquivo de estado não encontrado (${STATE_FILE}). Usando caminhos padrão."
    fi
    # Sem root só é permitido para uma instalação de usuário.
    if ! is_root && [ "${USER_MODE:-0}" != "1" ]; then
        err "Desinstalação de uma instalação de sistema exige root. Use sudo."
    fi
    do_uninstall "$@"
else
    do_install
fi
