#!/bin/sh
# SessionStart hook: injeta os guardrails do fluxo XP/TDD no contexto do Claude a
# cada início de sessão (versionado no repo, lido toda vez). Saída em JSON com
# hookSpecificOutput.additionalContext; se jq faltar, cai para texto puro (que o
# SessionStart também adiciona ao contexto).

guardrails=$(cat <<'EOF'
GUARDRAILS DO FLUXO (os-camera) — leia antes de agir:

1. Toda história começa por story file + branch a partir de develop (use /story). Nada de código/teste sem isso.
2. TDD red → green → refactor: nunca código de produção sem um teste falhando antes.
3. Rode testes SEMPRE via scripts versionados: `bash scripts/check.sh` (nunca `docker run ... node` nem `go test` crus).
4. CRITÉRIOS DE ACEITAÇÃO: o driver NÃO os marca. Só o `scripts/check.sh` marca o 1º (verdes). Os demais critérios E o `[x] Aprovado` são marcados pelo NAVIGATOR via `scripts/story-approval.sh`. Preencha a seção `## Revisão` SEM tocar nos checkboxes dos critérios.
5. Nenhum commit antes de `[x] Aprovado`. Commit via `scripts/commit.sh`.
6. Após aprovação + push, abra o PR (`scripts/push-pr.sh`, base develop) E mergeie quando verde (`scripts/merge-when-green.sh <PR#>`) — direto, SEM perguntar em nenhum dos dois. Só o corte de release (`develop → master`) depende do navigator.
7. `master` e `develop` são protegidos: nunca commit/push direto.
EOF
)

if command -v jq >/dev/null 2>&1; then
  jq -n --arg ctx "$guardrails" \
    '{hookSpecificOutput:{hookEventName:"SessionStart",additionalContext:$ctx}}'
else
  printf '%s\n' "$guardrails"
fi
