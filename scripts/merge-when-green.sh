#!/usr/bin/env bash
# Aguarda o CI de um PR ficar verde e, então, mergeia em develop, sincroniza o
# branch local, deleta a branch da história e marca [✓] no release file.
#
# Projetado para colapsar o ciclo pós-PR numa única invocação (economia de tokens):
# o watch do CI roda em silêncio e só o resumo final é impresso.
#
# Uso: scripts/merge-when-green.sh <PR#>
#
# Segurança: recusa mergear PRs cuja base seja master (releases são aprovadas à mão).
# Idempotente: pode rodar sobre um PR já mergeado sem efeito colateral.

set -euo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RESET='\033[0m'

PR="${1:-}"
if [[ -z "$PR" ]]; then
    echo -e "${RED}Uso: $0 <PR#>${RESET}" >&2
    exit 2
fi

for cmd in git gh; do
    command -v "$cmd" &>/dev/null || { echo -e "${RED}Erro: $cmd não encontrado${RESET}" >&2; exit 2; }
done

# ── metadados do PR ─────────────────────────────────────────────────────────
read -r BASE HEAD STATE < <(
    gh pr view "$PR" --json baseRefName,headRefName,state \
        -q '[.baseRefName, .headRefName, .state] | @tsv' 2>/dev/null
) || { echo -e "${RED}PR #$PR não encontrado.${RESET}" >&2; exit 2; }

if [[ "$BASE" != "develop" ]]; then
    echo -e "${RED}Recusado: base do PR #$PR é '${BASE}', não 'develop'. Mergeie manualmente.${RESET}" >&2
    exit 1
fi

# ── espera o CI (silencioso) — só mergeia se ainda não estiver mergeado ──────
if [[ "$STATE" != "MERGED" ]]; then
    echo -e "${YELLOW}Aguardando CI do PR #$PR...${RESET}"
    # Logo após `gh pr create` os checks ainda não foram registrados e
    # `gh pr checks --watch` sai com erro ("no checks reported"). Espera os
    # checks surgirem antes de observar, para não confundir corrida com falha.
    for _ in $(seq 1 30); do
        [[ -n "$(gh pr checks "$PR" 2>/dev/null)" ]] && break
        sleep 4
    done
    if ! gh pr checks "$PR" --watch --interval 20 >/dev/null 2>&1; then
        echo -e "${RED}CI FALHOU no PR #$PR:${RESET}" >&2
        gh pr checks "$PR" 2>/dev/null | grep -iE 'fail|pending' >&2 || true
        exit 1
    fi
    gh pr merge "$PR" --merge >/dev/null
fi

# ── sincroniza develop e remove a branch local ──────────────────────────────
git checkout develop --quiet
git fetch origin develop --quiet
git pull origin develop --ff-only --quiet
MERGE_SHA="$(git rev-parse --short HEAD)"

BRANCH_NOTE="branch local ausente"
if git show-ref --verify --quiet "refs/heads/${HEAD}"; then
    git branch -D "$HEAD" >/dev/null 2>&1 && BRANCH_NOTE="branch ${HEAD} deletada"
fi

# ── marca [✓] no release file mais recente ──────────────────────────────────
REL_NOTE="release file não atualizado"
RF="$(ls -t releases/2*_v*.md 2>/dev/null | head -1 || true)"
if [[ -n "$RF" ]] && grep -q "#${PR} " "$RF"; then
    sed -i "/#${PR} /s/\[~\]/[✓]/" "$RF"
    REL_NOTE="$(basename "$RF"): #${PR} → [✓]"
fi

echo -e "${GREEN}MERGED #${PR} → develop @ ${MERGE_SHA} | ${BRANCH_NOTE} | ${REL_NOTE}${RESET}"
