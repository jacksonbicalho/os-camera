#!/bin/sh
set -e

# Abre o PR de release develop → master (encapsula o fluxo do skill /release-pr).
# Idempotente: se já houver PR aberto, mostra a URL e sai. Não mergeia (master exige
# aprovação humana). Uso: scripts/release-pr.sh [arquivo de release | vX.Y.Z]

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

err() { printf '\033[1;31mERR \033[0m%s\n' "$*" >&2; exit 1; }
info() { printf '\033[1;34m==> \033[0m%s\n' "$*"; }

# --- 1. localizar o release file ---
arg="${1:-}"
if [ -z "$arg" ]; then
    RF="$(ls -1 releases/*_next.md 2>/dev/null | tail -1)"
    [ -n "$RF" ] || err "Nenhum releases/*_next.md encontrado."
elif [ -f "$arg" ]; then
    RF="$arg"
else
    RF="$(ls -1 releases/*_"${arg}".md 2>/dev/null | tail -1)"
    [ -n "$RF" ] || err "Release file para '$arg' não encontrado em releases/."
fi
info "Release file: $RF"

# --- 2. validar que todas as histórias estão [✓] ---
pending="$(grep -E '^\| \[' "$RF" | grep -v '\[✓\]' || true)"
if [ -n "$pending" ]; then
    printf '\033[1;31mHistórias pendentes (não [✓]):\033[0m\n%s\n' "$pending" >&2
    err "Conclua/mergeie as histórias acima antes de cortar a release."
fi

# --- 3. pré-condições git ---
git checkout develop >/dev/null 2>&1 || err "Não consegui ir para develop."
git fetch origin develop -q
git pull origin develop --ff-only >/dev/null 2>&1 || err "develop não fast-forwarda (resolva antes)."
[ -z "$(git status --porcelain)" ] || err "Working tree suja — commite/descarte antes."
git fetch origin master -q
ahead="$(git rev-list --count origin/master..develop)"
[ "$ahead" -gt 0 ] || err "develop não está à frente de master (nada para liberar)."

# --- 4. idempotência: PR já aberto? ---
existing="$(gh pr list --base master --head develop --state open --json url -q '.[0].url' 2>/dev/null || true)"
if [ -n "$existing" ]; then
    info "PR de release já aberto: $existing"
    exit 0
fi

# --- 5. versão (bump convencional desde a última tag) ---
last_tag="$(git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0-dev)"
base="${last_tag#v}"; base="${base%-dev}"
MA="${base%%.*}"; rest="${base#*.}"; MI="${rest%%.*}"; PA="${rest#*.}"
msgs="$(git log origin/master..develop --pretty=%s)"
bump=patch
printf '%s\n' "$msgs" | grep -qE '^[a-z]+(\([^)]+\))?!:' && bump=major
printf '%s\n' "$msgs" | grep -qi 'BREAKING CHANGE' && bump=major
if [ "$bump" = patch ] && printf '%s\n' "$msgs" | grep -qE '^feat(\([^)]+\))?:'; then bump=minor; fi
case "$bump" in
    major) MA=$((MA + 1)); MI=0; PA=0 ;;
    minor) MI=$((MI + 1)); PA=0 ;;
    patch) PA=$((PA + 1)) ;;
esac
version="v${MA}.${MI}.${PA}-dev"
info "Versão estimada: $version (bump $bump desde $last_tag) — a tag definitiva sai no release-tag."

# --- 6. abrir o PR ---
body_list="$(awk -F'|' '/^\| \[✓\]/ { d=$3; p=$5; gsub(/^ +| +$/,"",d); gsub(/^ +| +$/,"",p); print "- " p " — " d }' "$RF")"
url="$(gh pr create --base master --head develop --title "release: ${version}" --body "$(cat <<EOF
## Release ${version}

Corte de \`develop\` → \`master\`.

## Histórias incluídas

${body_list}

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)")"
info "PR de release aberto: $url"
info "Aprove e mergeie no GitHub; depois rode /release-tag."
