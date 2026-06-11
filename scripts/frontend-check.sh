#!/usr/bin/env bash
# Roda os checks do frontend (lint + test + tsc + build) via Docker com a
# invocação CORRETA, encapsulando as armadilhas que costumam dar errado:
#   - usa caminho ABSOLUTO do repo (derivado do próprio script) — nunca $(pwd),
#     que deriva quando a cwd muda;
#   - usa os binários locais (node_modules/.bin/tsc|vite), nunca `npx`;
#   - builda para /tmp (evita EACCES no ./dist root-owned do rebuild do dev);
#   - roda com --user só se o node_modules for do usuário atual (senão, root).
#
# Como a invocação é estática (`scripts/frontend-check.sh ...`), o allowlist do
# Claude Code funciona e não pede confirmação (o $(id -u) fica AQUI dentro, fora
# do comando invocado).
#
# Uso:
#   scripts/frontend-check.sh                 # check completo (lint+test+tsc+build)
#   scripts/frontend-check.sh <arquivo_teste> # roda só esse teste (yarn test --run <arquivo>)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FE="$ROOT/frontend"

userflag=()
if [ -d "$FE/node_modules" ] && [ "$(stat -c %u "$FE/node_modules" 2>/dev/null)" = "$(id -u)" ]; then
  userflag=(--user "$(id -u):$(id -g)")
fi

dock() {
  docker run --rm "${userflag[@]}" \
    -v "$FE":/app -v camera-yarn-cache:/yarn-cache \
    -w /app -e YARN_CACHE_FOLDER=/yarn-cache -e HOME=/tmp \
    node:20-alpine sh -c "$1"
}

if [ "$#" -gt 0 ]; then
  dock "yarn install --frozen-lockfile >/dev/null 2>&1 && yarn test --run $*"
else
  dock "yarn install --frozen-lockfile >/dev/null 2>&1 && yarn lint && yarn test --run && node_modules/.bin/tsc -b && node_modules/.bin/vite build --outDir /tmp/distc --emptyOutDir"
fi
