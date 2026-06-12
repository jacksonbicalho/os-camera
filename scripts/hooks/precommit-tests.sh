#!/bin/sh
# PreToolUse(Bash) gate: bloqueia `git commit` quando `go build ./...` ou
# `go test -count=1 ./...` falham. Escopo backend (frontend é coberto pelo CI).
# Lê o JSON do hook via stdin.
input=$(cat)
cmd=$(echo "$input" | jq -r '.tool_input.command')
echo "$cmd" | grep -q 'git commit' || exit 0

cd "$(git rev-parse --show-toplevel 2>/dev/null)" || exit 0
out=$(go build ./... 2>&1 && go test -count=1 ./... 2>&1)
[ $? -eq 0 ] && exit 0

echo "❌ Build/testes falharam — commit bloqueado. Corrija antes de commitar:"
echo "$out" | grep -E 'FAIL|error|cannot' | head -20
exit 1
