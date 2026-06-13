#!/usr/bin/env bash
# Aprovação da story da branch atual.
#
# - Se há Critérios de Aceitação ainda não-marcados:
#     • com terminal (navigator roda `! scripts/story-approval.sh`): pergunta
#       sobre cada um e marca [x] conforme a resposta;
#     • sem terminal (Claude roda): sai != 0 pedindo para rodar interativo.
# - Quando TODOS os critérios estão cumpridos: marca `[x] Aprovado`
#   automaticamente e sai 0 ("passou limpo").
#
# Assim Claude pode rodar `./scripts/story-approval.sh` como gate: exit 0 =
# aprovada → segue o ciclo commit→push→PR.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

branch=$(git rev-parse --abbrev-ref HEAD)
desc=$(echo "$branch" | sed 's|^[^/]*/||' | tr '-' '_')
story=$(ls stories/*.md 2>/dev/null | grep -i "$desc" | tail -1)
[ -z "$story" ] && { echo "⚠️ Nenhuma story encontrada para a branch: $branch"; exit 2; }

start=$(grep -nE '^## Crit' "$story" | head -1 | cut -d: -f1)
[ -z "$start" ] && { echo "⚠️ Story sem seção de Critérios de Aceitação."; exit 2; }
end=$(awk -v s="$start" 'NR>s && /^## /{print NR; exit}' "$story")
[ -z "$end" ] && end=$(($(wc -l < "$story") + 1))

pending_lines() { awk -v s="$start" -v e="$end" 'NR>s && NR<e && /^- \[[ ]?\]/{print NR}' "$story"; }
has_tty() { { true >/dev/tty; } 2>/dev/null; }

mapfile -t pending < <(pending_lines)

if [ "${#pending[@]}" -gt 0 ]; then
    if ! has_tty; then
        echo "⏳ $(basename "$story"): ${#pending[@]} critério(s) ainda não marcado(s)."
        echo "   Rode interativo para revisar e aprovar: ! scripts/story-approval.sh"
        exit 1
    fi
    echo "📋 Story: $(basename "$story")"
    ask() {
        local ans
        printf '%s [s/N] ' "$1" > /dev/tty
        read -r ans < /dev/tty || ans=""
        case "$ans" in [sSyY]*) return 0 ;; *) return 1 ;; esac
    }
    for ln in "${pending[@]}"; do
        text=$(sed -n "${ln}p" "$story" | sed 's/^- \[[ ]*\] *//')
        echo
        echo "• $text"
        if ask "  Critério cumprido?"; then
            sed -i "${ln}s/^- \[[ ]*\]/- [x]/" "$story"
        fi
    done
    echo
fi

still=$(pending_lines | grep -c . || true)
if [ "$still" -gt 0 ]; then
    echo "⏳ $still critério(s) ainda pendente(s) — story NÃO aprovada."
    exit 1
fi

if grep -qE '^-? *\[x\] Aprovado' "$story"; then
    echo "✅ $(basename "$story"): critérios cumpridos e já [x] Aprovado."
else
    sed -i 's/^- *\[[ ]*\] Aprovado/- [x] Aprovado/' "$story"
    echo "✅ $(basename "$story"): todos os critérios cumpridos → APROVADA automaticamente."
fi
exit 0
