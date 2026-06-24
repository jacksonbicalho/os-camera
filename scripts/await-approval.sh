#!/usr/bin/env bash
# Bloqueia (poll silencioso) até a story estar pronta para seguir: nenhum
# checkbox desmarcado (todos os Critérios de Aceitação cumpridos) E `[x] Aprovado`.
#
# Resolve o story file a partir da branch atual (como check.sh/story-approval.sh)
# ou via argumento. Padrão ancorado, case-insensitive.
#
# Uso: scripts/await-approval.sh [story.md]   (rodar em background)
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

story="${1:-}"
if [ -z "$story" ]; then
    branch=$(git rev-parse --abbrev-ref HEAD)
    desc=$(echo "$branch" | sed 's|^[^/]*/||' | tr '-' '_')
    story=$(ls stories/*.md 2>/dev/null | grep -i "$desc" | tail -1)
fi
if [ -z "$story" ] || [ ! -f "$story" ]; then
    echo "❌ Story não encontrada${story:+: $story}"; exit 1
fi

echo "⏳ Aguardando aprovação (critérios + Aprovado): $(basename "$story")"
until ! grep -qE '^- \[[ ]*\] ' "$story" && grep -qiE '^- *\[[xX]\] Aprovado' "$story"; do
    sleep 15
done
echo "✅ Aprovada — todos os critérios marcados + [x] Aprovado: $(basename "$story")"
