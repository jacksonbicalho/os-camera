#!/bin/sh
# PreToolUse(Bash) gate: `gh pr create --base master` só é permitido a partir de
# `develop` ou `release/*`. Lê o JSON do hook via stdin.
input=$(cat)
cmd=$(echo "$input" | jq -r '.tool_input.command')
echo "$cmd" | grep -q 'gh pr create' || exit 0
echo "$cmd" | grep -q -- '--base master' || exit 0

cd "$(git rev-parse --show-toplevel 2>/dev/null)" || exit 0
branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)
[ "$branch" = "develop" ] && exit 0
case "$branch" in release/*) exit 0 ;; esac

echo "❌ PRs de feature devem usar --base develop, nao --base master."
echo "   Excecao: branches release/* e develop podem abrir PR para master."
exit 1
