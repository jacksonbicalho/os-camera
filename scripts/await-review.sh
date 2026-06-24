#!/usr/bin/env bash
# Bloqueia (poll silencioso) até a story ter `[x] História revisada` — o gate de
# revisão do plano, antes de o driver iniciar a implementação.
#
# Resolve o story file a partir da branch atual (como check.sh/story-approval.sh)
# ou via argumento. Padrão ancorado (`^- \[[xX]\] ...`), case-insensitive,
# imune a menções do termo na prosa da story.
#
# Uso: scripts/await-review.sh [story.md]   (rodar em background)
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

echo "⏳ Aguardando revisão da história: $(basename "$story")"
until grep -qiE '^- \[[xX]\] História revisada' "$story"; do sleep 15; done
echo "✅ História revisada: $(basename "$story")"
