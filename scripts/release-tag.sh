#!/usr/bin/env bash
# Corta a release numa única invocação (economia de tokens), após o PR
# develop→master já estar mergeado:
#   - cria e envia a tag via release.sh (confirmação automática)
#   - aguarda o workflow Release publicar (em silêncio)
#   - mergeia master de volta em develop (passo pós-tag)
#   - imprime só o resumo final
#
# Uso:
#   scripts/release-tag.sh            # corta a release
#   scripts/release-tag.sh --dry-run  # só mostra a versão que sairia
#
# Pré-requisito: o PR de release (develop→master) já mergeado em master.

set -euo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RESET='\033[0m'

DRY_RUN=false
[[ "${1:-}" == "--dry-run" ]] && DRY_RUN=true

for cmd in git gh; do
    command -v "$cmd" &>/dev/null || { echo -e "${RED}Erro: $cmd não encontrado${RESET}" >&2; exit 2; }
done

strip_ansi() { sed 's/\x1b\[[0-9;]*m//g'; }

ORIG_BRANCH="$(git branch --show-current)"

git checkout master --quiet
git fetch origin master --quiet
git pull origin master --ff-only --quiet

# ── versão que sairia ───────────────────────────────────────────────────────
DRY_OUT="$(./scripts/release.sh --dry-run 2>&1 | strip_ansi)"
if grep -q "Nada para versionar" <<< "$DRY_OUT"; then
    echo -e "${YELLOW}Nada a versionar desde a última tag.${RESET}"
    [[ "$ORIG_BRANCH" != "master" ]] && git checkout "$ORIG_BRANCH" --quiet
    exit 0
fi
VERSION="$(grep 'Nova versão' <<< "$DRY_OUT" | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+[-.A-Za-z0-9]*' | tail -1)"
if [[ -z "$VERSION" ]]; then
    echo -e "${RED}Não foi possível determinar a versão a partir do dry-run.${RESET}" >&2
    [[ "$ORIG_BRANCH" != "master" ]] && git checkout "$ORIG_BRANCH" --quiet
    exit 1
fi

if [[ "$DRY_RUN" == true ]]; then
    echo -e "${GREEN}[dry-run] sairia: ${VERSION}${RESET}"
    [[ "$ORIG_BRANCH" != "master" ]] && git checkout "$ORIG_BRANCH" --quiet
    exit 0
fi

# ── cria e envia a tag ──────────────────────────────────────────────────────
echo -e "${YELLOW}Criando tag ${VERSION}...${RESET}"
echo s | ./scripts/release.sh >/dev/null 2>&1

# ── aguarda a release publicar (silencioso, timeout ~6min) ──────────────────
echo -e "${YELLOW}Aguardando o workflow Release publicar ${VERSION}...${RESET}"
ASSETS=0
for _ in $(seq 1 72); do
    ASSETS="$(gh release view "$VERSION" --json assets -q '.assets | length' 2>/dev/null || echo 0)"
    [[ "$ASSETS" -gt 0 ]] && break
    sleep 5
done
if [[ "$ASSETS" -eq 0 ]]; then
    echo -e "${RED}Tag ${VERSION} enviada, mas a release não publicou em tempo. Verifique o GitHub Actions.${RESET}" >&2
    exit 1
fi

# ── merge pós-tag master → develop ──────────────────────────────────────────
git checkout develop --quiet
git fetch origin master --quiet
git merge origin/master --no-edit --quiet
git push origin develop --quiet

echo -e "${GREEN}RELEASED ${VERSION} | assets: ${ASSETS} | develop sincronizado (describe: $(git describe --tags))${RESET}"
