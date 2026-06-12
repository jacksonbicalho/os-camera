#!/usr/bin/env bash
# Push da branch da história + abre um PR em develop.
#
# Só roda se: branch de história (≠ develop/master), working tree LIMPA (tudo
# commitado) e story `[x] Aprovado`. Idempotente: se já existe PR aberto para a
# branch, não recria. NÃO mergeia — o merge é do `merge-when-green.sh` (este
# abre o PR, aquele aguarda o CI e fecha).
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

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
    exit 0
fi

title=$(grep -m1 '^# ' "$story" | sed 's/^# *//')
gh pr create --base develop --head "$branch" --title "$title" \
    --body "$(printf '%s\n\n🤖 Generated with [Claude Code](https://claude.com/claude-code)' "$title")"
