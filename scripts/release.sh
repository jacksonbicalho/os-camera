#!/usr/bin/env bash
# Gera uma nova versão a partir dos commits convencionais desde a última tag.
# Por enquanto todas as releases são beta (-beta.N).
#
# Uso:
#   ./scripts/release.sh            # interativo — cria e envia a tag
#   ./scripts/release.sh --dry-run  # apenas exibe o que seria feito

set -euo pipefail

DRY_RUN=false
for arg in "$@"; do
    [[ "$arg" == "--dry-run" ]] && DRY_RUN=true
done

# ── cores ─────────────────────────────────────────────────────────────────────
RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

# ── pré-requisitos ────────────────────────────────────────────────────────────
for cmd in git; do
    command -v "$cmd" &>/dev/null || { echo -e "${RED}Erro: $cmd não encontrado${RESET}" >&2; exit 1; }
done

# Garante que não há alterações em arquivos rastreados
if [[ -n "$(git status --porcelain | grep -v '^??')" ]]; then
    echo -e "${RED}Erro: há alterações não commitadas. Faça commit ou stash antes de criar uma release.${RESET}" >&2
    exit 1
fi

# ── commits desde a última tag ────────────────────────────────────────────────
LAST_TAG="$(git describe --tags --abbrev=0 2>/dev/null || echo "")"
RANGE="${LAST_TAG:+${LAST_TAG}..}HEAD"
COMMITS="$(git log "$RANGE" --format="%H %s" --no-merges)"

if [[ -z "$COMMITS" ]]; then
    echo -e "${YELLOW}Nenhum commit desde ${LAST_TAG:-o início}. Nada para versionar.${RESET}"
    exit 0
fi

# ── determina tipo de bump ────────────────────────────────────────────────────
BUMP="patch"
while read -r _hash subject; do
    if [[ "$subject" =~ ^[a-z]+[^:]*!: ]] || grep -qi "breaking.change" <<< "$subject"; then
        BUMP="major"; break
    fi
    if [[ "$subject" =~ ^feat ]]; then
        BUMP="minor"
    fi
done <<< "$COMMITS"

# ── calcula a próxima versão ──────────────────────────────────────────────────
# Base semântica: remove sufixo -beta.N / -rc.N etc e o 'v'
LAST_BASE="${LAST_TAG:-v0.0.0}"
LAST_BASE="${LAST_BASE%%-*}"
LAST_BASE="${LAST_BASE#v}"

IFS='.' read -r MAJ MIN PAT <<< "$LAST_BASE"
MAJ="${MAJ:-0}"; MIN="${MIN:-0}"; PAT="${PAT:-0}"

case "$BUMP" in
    major) MAJ=$((MAJ + 1)); MIN=0; PAT=0 ;;
    minor) MIN=$((MIN + 1)); PAT=0 ;;
    patch) PAT=$((PAT + 1)) ;;
esac

NEW_BASE="v${MAJ}.${MIN}.${PAT}"

# Incrementa beta dentro do mesmo base se já houver
BETA_N=1
EXISTING="$(git tag --list "${NEW_BASE}-beta.*" 2>/dev/null | sort -V | tail -1 || true)"
if [[ -n "$EXISTING" ]]; then
    LAST_N="${EXISTING##*beta.}"
    BETA_N=$((LAST_N + 1))
fi

NEW_VERSION="${NEW_BASE}-beta.${BETA_N}"

# ── organiza commits por categoria ────────────────────────────────────────────
declare -A SECTIONS=(
    [feat]="Novidades"
    [fix]="Correções"
    [perf]="Performance"
    [refactor]="Refatoração"
    [docs]="Documentação"
    [test]="Testes"
    [chore]="Manutenção"
    [ci]="CI/Build"
    [other]="Outros"
)
ORDER=(feat fix perf refactor docs test chore ci other)

declare -A LINES
for key in "${!SECTIONS[@]}"; do LINES[$key]=""; done

while read -r hash subject; do
    short_hash="${hash:0:7}"
    type=""
    scope=""
    breaking=""
    msg="${subject#*: }"   # tudo após o primeiro ": "

    # Extrai tipo
    if [[ "$subject" =~ ^([a-z]+) ]]; then
        type="${BASH_REMATCH[1]}"
    fi
    # Extrai scope entre parênteses
    _scope_re='^[a-z]+\(([^)]+)\)'
    if [[ "$subject" =~ $_scope_re ]]; then
        scope="${BASH_REMATCH[1]}"
    fi
    # Detecta breaking change
    if [[ "$subject" =~ ^[a-z]+[^:]*!: ]]; then
        breaking=" ⚠ BREAKING"
    fi

    if [[ -n "$scope" ]]; then
        line="- **${scope}**: ${msg}${breaking} (\`${short_hash}\`)"
    else
        line="- ${msg}${breaking} (\`${short_hash}\`)"
    fi

    if [[ -n "$type" ]] && [[ -v "SECTIONS[$type]" ]]; then
        LINES[$type]+="${line}"$'\n'
    else
        LINES[other]+="${line}"$'\n'
    fi
done <<< "$COMMITS"

# Monta corpo do changelog
CHANGELOG=""
for key in "${ORDER[@]}"; do
    [[ -z "${LINES[$key]:-}" ]] && continue
    CHANGELOG+="### ${SECTIONS[$key]}"$'\n'
    CHANGELOG+="${LINES[$key]}"$'\n'
done

# ── exibe resumo ──────────────────────────────────────────────────────────────
echo ""
echo -e "${BOLD}╔═══════════════════════════��══════════════╗${RESET}"
echo -e "${BOLD}║           Nova release: ${CYAN}${NEW_VERSION}${RESET}${BOLD}   ║${RESET}"
echo -e "${BOLD}╚══════════════════════════════════════════╝${RESET}"
echo ""
echo -e "  Última tag : ${YELLOW}${LAST_TAG:-<nenhuma>}${RESET}"
echo -e "  Tipo bump  : ${GREEN}${BUMP}${RESET}"
echo -e "  Nova versão: ${CYAN}${NEW_VERSION}${RESET}"
echo ""
echo -e "${BOLD}── Changelog ─────────────────────────────��──${RESET}"
echo ""
echo "$CHANGELOG"

if [[ "$DRY_RUN" == true ]]; then
    echo -e "${YELLOW}[dry-run] Nenhuma tag criada.${RESET}"
    exit 0
fi

# ── confirmação ───────────────────────────────────────────────────────────────
echo -e "${BOLD}Criar e enviar a tag ${CYAN}${NEW_VERSION}${RESET}${BOLD}? [s/N]${RESET} "
read -r confirm
[[ "$confirm" =~ ^[sS]$ ]] || { echo "Cancelado."; exit 0; }

# ── cria tag anotada ──────────────────────────────────────────────────────────
TAG_BODY="## ${NEW_VERSION}"$'\n\n'"${CHANGELOG}"

git tag -a "$NEW_VERSION" -m "$TAG_BODY"
echo -e "${GREEN}Tag ${NEW_VERSION} criada.${RESET}"

# ── envia ao remote ───────────────────────────────────────────────────────────
echo -e "${YELLOW}Enviando tag ao remote...${RESET}"
git push origin "$NEW_VERSION"
echo -e "${GREEN}Tag enviada. GitHub Actions iniciará a release automaticamente.${RESET}"
echo ""
echo -e "  Acompanhe em: https://github.com/jacksonbicalho/camera/actions"
