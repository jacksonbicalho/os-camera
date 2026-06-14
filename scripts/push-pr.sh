#!/usr/bin/env bash
# Orquestra o ciclo pós-aprovação: push da branch + abre um PR em develop +
# (por padrão) aguarda o CI ficar verde e mergeia via merge-when-green.sh.
#
# Só roda se: branch de história (≠ develop/master), working tree LIMPA (tudo
# commitado) e story `[x] Aprovado`. Idempotente: se já existe PR aberto para a
# branch, não recria — só (re)espera o CI e mergeia. Em CI vermelho o
# merge-when-green.sh sai com erro sem mergear (PR fica aberto); conserte,
# commite o fix e rode de novo.
#
# Uso: scripts/push-pr.sh [--no-merge]
#   --no-merge  apenas empurra e abre o PR, sem aguardar o CI nem mergear.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

no_merge=0
for arg in "$@"; do
    case "$arg" in
        --no-merge) no_merge=1 ;;
        *) echo "❌ Argumento desconhecido: $arg (uso: push-pr.sh [--no-merge])"; exit 2 ;;
    esac
done

branch=$(git rev-parse --abbrev-ref HEAD)
case "$branch" in
    develop|master) echo "❌ Não rode push-pr em '$branch'."; exit 1 ;;
esac

if [ -n "$(git status --porcelain)" ]; then
    echo "❌ Working tree suja — commite tudo antes de abrir o PR:"
    git status --short
    exit 1
fi

desc=$(echo "$branch" | sed 's|^[^/]*/||' | tr '-' '_')
story=$(ls stories/*.md 2>/dev/null | grep -i "$desc" | tail -1)
[ -z "$story" ] && { echo "❌ Story não encontrada para a branch: $branch"; exit 1; }
grep -qE '^-? *\[x\] Aprovado' "$story" \
    || { echo "❌ Story não aprovada ([x] Aprovado): $(basename "$story")"; exit 1; }

git push -u origin "$branch"

existing=$(gh pr list --head "$branch" --base develop --state open --json number -q '.[0].number' 2>/dev/null || true)
if [ -n "$existing" ]; then
    echo "ℹ️ PR já existe para $branch: #$existing"
    prnum="$existing"
else
    title=$(grep -m1 '^# ' "$story" | sed 's/^# *//')

    # Corpo do PR montado a partir das seções da story (Contexto + Solução),
    # para o PR já nascer com uma descrição rica em vez de só o título.
    section() {
        awk -v sec="$1" '
            $0 ~ "^## " sec { f=1; next }
            /^## / { f=0 }
            f { print }
        ' "$story"
    }
    nl=$'\n'
    contexto=$(section 'Contexto')
    solucao=$(section 'Solu')
    body=""
    [ -n "$contexto" ] && body+="## Contexto${nl}${contexto}${nl}${nl}"
    [ -n "$solucao" ] && body+="## Solução${nl}${solucao}${nl}${nl}"
    [ -z "$body" ] && body="${title}${nl}${nl}"
    body+="🤖 Generated with [Claude Code](https://claude.com/claude-code)"

    gh pr create --base develop --head "$branch" --title "$title" --body "$body"
    prnum=$(gh pr list --head "$branch" --base develop --state open --json number -q '.[0].number')
fi

if [ "$no_merge" -eq 1 ]; then
    echo "ℹ️ --no-merge: PR #$prnum aberto, sem mergear."
    exit 0
fi

# Aguarda o CI e mergeia (propaga o exit code: CI vermelho → erro, sem merge).
exec "$ROOT/scripts/merge-when-green.sh" "$prnum"
