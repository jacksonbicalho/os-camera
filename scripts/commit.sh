#!/usr/bin/env bash
# Commita as mudanças JÁ STAGED usando o heading da story como mensagem.
# O heading já está no formato `tipo(escopo): descrição` — usado verbatim
# (sem prefixar `feat:`, que duplicaria o tipo). Exige `[x] Aprovado` na story
# e adiciona o trailer Co-Authored-By.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

branch=$(git rev-parse --abbrev-ref HEAD)
desc=$(echo "$branch" | sed 's|^[^/]*/||' | tr '-' '_')
story=$(ls stories/*.md 2>/dev/null | grep -i "$desc" | tail -1)
[ -z "$story" ] && { echo "❌ Story não encontrada para a branch: $branch"; exit 1; }

grep -qE '^-? *\[x\] Aprovado' "$story" \
    || { echo "❌ Story não aprovada ([x] Aprovado): $(basename "$story")"; exit 1; }

title=$(grep -m1 '^# ' "$story" | sed 's/^# *//')
[ -z "$title" ] && { echo "❌ Não foi possível extrair o título (heading '# ') da story."; exit 1; }

git diff --cached --quiet && { echo "❌ Nada staged — faça 'git add' antes."; exit 1; }

echo "Commit: $title"
git commit -m "$title" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
