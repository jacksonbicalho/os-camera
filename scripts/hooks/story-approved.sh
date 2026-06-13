#!/bin/sh
# PreToolUse(Bash) gate: bloqueia `git commit` numa branch de história cuja
# story não tenha `[x] Aprovado`. Lê o JSON do hook via stdin.
input=$(cat)
cmd=$(echo "$input" | jq -r '.tool_input.command')
echo "$cmd" | grep -q 'git commit' || exit 0

cd "$(git rev-parse --show-toplevel 2>/dev/null)" || exit 0
branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)
[ "$branch" = "master" ] || [ "$branch" = "develop" ] && exit 0

desc=$(echo "$branch" | sed 's|^[^/]*/||' | tr '-' '_')
story=$(ls stories/*.md 2>/dev/null | grep -i "$desc" | tail -1)
[ -z "$story" ] && exit 0

grep -qE '^-? *\[x\] Aprovado' "$story" && exit 0
echo "❌ Story nao aprovada: $story"
echo "   Navigator precisa marcar [x] Aprovado antes de commitar."
exit 1
