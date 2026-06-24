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

for cmd in git gh jq; do
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

# ── espera o CI do commit HEAD atual (silencioso) ───────────────────────────
# Avalia os check-runs do SHA exato da PR (via API), em vez de `gh pr checks
# --watch`. Isso evita falsos negativos tanto na corrida pós-create (checks ainda
# não registrados) quanto na pós-push (checks obsoletos do commit anterior).
if [[ "$STATE" != "MERGED" ]]; then
    echo -e "${YELLOW}Aguardando CI do PR #$PR...${RESET}"
    REPO="$(gh repo view --json nameWithOwner -q .nameWithOwner)"
    SHA="$(gh pr view "$PR" --json headRefOid -q .headRefOid)"
    failed=1  # timeout/sem conclusão = falha (não mergeia)
    for _ in $(seq 1 120); do  # ~20 min (10s cada)
        runs="$(gh api "repos/$REPO/commits/$SHA/check-runs" 2>/dev/null || echo '{}')"
        total="$(echo "$runs" | jq -r '.total_count // 0')"
        if [[ "$total" -gt 0 ]]; then
            pending="$(echo "$runs" | jq -r '[.check_runs[] | select(.status != "completed")] | length')"
            if [[ "$pending" -eq 0 ]]; then
                failed="$(echo "$runs" | jq -r '[.check_runs[] | select(.conclusion != "success" and .conclusion != "neutral" and .conclusion != "skipped")] | length')"
                break
            fi
        fi
        sleep 10
    done
    if [[ "$failed" -ne 0 ]]; then
        echo -e "${RED}CI FALHOU (ou não concluiu) no PR #$PR:${RESET}" >&2
        gh pr checks "$PR" 2>/dev/null | grep -iE 'fail|pending' >&2 || true
        exit 1
    fi
    gh pr merge "$PR" --merge >/dev/null
fi

# ── sincroniza develop ──────────────────────────────────────────────────────
git checkout develop --quiet
git fetch origin develop --quiet
git pull origin develop --ff-only --quiet
MERGE_SHA="$(git rev-parse --short HEAD)"

# ── marca [✓] no release file que contém a linha do PR ──────────────────────
# Seleciona pelo conteúdo (funciona com `_next.md` e com `_vX.Y.Z.md`).
REL_NOTE="release file não atualizado"
MARKED=0
RF="$(grep -lF "#${PR} " releases/*.md 2>/dev/null | head -1 || true)"
if [[ -n "$RF" ]]; then
    sed -i "/#${PR} /s/\[~\]/[✓]/" "$RF"
    REL_NOTE="$(basename "$RF"): #${PR} → [✓]"
    MARKED=1
fi

# ── remove story file + branch SÓ quando a história ficou [✓] no release file ─
# Se a marcação não rolou (linha ausente), preserva tudo para o driver resolver.
BRANCH_NOTE="branch/story mantidos (sem [✓] no release file)"
STORY_NOTE=""
if [[ "$MARKED" -eq 1 ]]; then
    BRANCH_NOTE="branch local ausente"
    if git show-ref --verify --quiet "refs/heads/${HEAD}"; then
        git branch -D "$HEAD" >/dev/null 2>&1 && BRANCH_NOTE="branch ${HEAD} deletada"
    fi
    desc=$(echo "$HEAD" | sed 's|^[^/]*/||' | tr '-' '_')
    story=$(ls stories/*.md 2>/dev/null | grep -i "$desc" | tail -1 || true)
    [[ -n "$story" ]] && rm -f "$story" && STORY_NOTE=" | story removido ($(basename "$story"))"
fi

echo -e "${GREEN}MERGED #${PR} → develop @ ${MERGE_SHA} | ${BRANCH_NOTE} | ${REL_NOTE}${STORY_NOTE}${RESET}"
