#!/bin/sh
# SessionStart hook: injeta os guardrails do fluxo XP/TDD no contexto do Claude a
# cada início de sessão (versionado no repo, lido toda vez). Saída em JSON com
# hookSpecificOutput.additionalContext; se jq faltar, cai para texto puro (que o
# SessionStart também adiciona ao contexto).

guardrails=$(cat <<'EOF'
GUARDRAILS DO FLUXO (os-camera) — leia antes de agir:

→ LEIA `docs/workflow.md` no início da sessão — é o fluxo completo (branches, CI, ciclo por história, scripts, release). O resumo abaixo é só a rede de segurança.

1. Toda história começa por /story (story file + branch a partir de develop). Nada de código/teste sem isso.
2. GATE DE REVISÃO: NÃO implemente (nem red phase) antes de `[x] História revisada` na story — monitore com `scripts/await-review.sh`.
3. TDD red → green → refactor. Rode testes via `bash scripts/check.sh` (nunca `docker run ... node` nem `go test` crus).
4. CRITÉRIOS DE ACEITAÇÃO: o driver NÃO os marca. Só o `scripts/check.sh` marca o 1º (verdes). Os demais E o `[x] Aprovado` são do NAVIGATOR via `scripts/story-approval.sh`. Preencha `## Revisão` SEM tocar nos checkboxes.
5. GATE DE APROVAÇÃO: nenhum commit antes de `[x] Aprovado`. Commit via `scripts/commit.sh`.
6. Após aprovação: `scripts/push-pr.sh` orquestra push + PR (base develop) + CI + merge — direto, SEM perguntar. Story file e branch só somem quando a história fica `[✓]` no release file. Só o corte de release (`develop → master`) depende do navigator.
7. `master` e `develop` são protegidos: nunca commit/push direto.
EOF
)

if command -v jq >/dev/null 2>&1; then
  jq -n --arg ctx "$guardrails" \
    '{hookSpecificOutput:{hookEventName:"SessionStart",additionalContext:$ctx}}'
else
  printf '%s\n' "$guardrails"
fi
