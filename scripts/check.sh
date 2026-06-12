#!/usr/bin/env bash
# Roda os checks "verde" e, se tudo passar, marca o 1º Critério de Aceitação da
# story da branch atual como [x].
#
# - Backend SEMPRE: `go build ./...` + `go test -count=1 ./...`
# - Frontend SE `frontend/` mudou (vs develop ou na working tree): frontend-check.sh
#
# Claude roda isto manualmente (não é hook). Equivale ao "CI local" da história.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

echo "→ go build ./..."
if ! go build ./...; then echo "❌ go build falhou"; exit 1; fi

echo "→ go test -count=1 ./..."
if ! go test -count=1 ./... >/tmp/check-go.log 2>&1; then
    echo "❌ go test falhou:"
    grep -E '--- FAIL|FAIL|panic|cannot' /tmp/check-go.log | head -20
    exit 1
fi
echo "✓ backend verde"

fe_changed=0
git diff --name-only develop...HEAD 2>/dev/null | grep -q '^frontend/' && fe_changed=1
git status --porcelain | grep -qE '^.. frontend/' && fe_changed=1

if [ "$fe_changed" -eq 1 ]; then
    echo "→ frontend mudou: scripts/frontend-check.sh (Docker)"
    if ! bash "$ROOT/scripts/frontend-check.sh" >/tmp/check-fe.log 2>&1; then
        echo "❌ frontend-check falhou:"
        tail -25 /tmp/check-fe.log
        exit 1
    fi
    echo "✓ frontend verde"
else
    echo "· frontend não mudou — pulando"
fi

# ── marca o 1º Critério de Aceitação da story ───────────────────────────────
branch=$(git rev-parse --abbrev-ref HEAD)
desc=$(echo "$branch" | sed 's|^[^/]*/||' | tr '-' '_')
story=$(ls stories/*.md 2>/dev/null | grep -i "$desc" | tail -1)
if [ -z "$story" ]; then
    echo "✅ tudo verde (nenhuma story encontrada para marcar)"
    exit 0
fi

# grep (UTF-8) acha o header; awk acha a 1ª linha de critério; sed marca.
start=$(grep -nE '^## Crit' "$story" | head -1 | cut -d: -f1)
if [ -z "$start" ]; then
    echo "✅ tudo verde (story sem seção de Critérios para marcar)"
    exit 0
fi
firstcrit=$(awk -v s="$start" 'NR>s && /^- \[/{print NR; exit}' "$story")
if [ -n "$firstcrit" ]; then
    sed -i "${firstcrit}s/^- \[[ x]*\]/- [x]/" "$story"
    echo "✅ tudo verde — 1º critério marcado em $(basename "$story")"
else
    echo "✅ tudo verde (nenhum critério encontrado para marcar)"
fi
