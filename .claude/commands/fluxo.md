---
description: Lê o fluxo de trabalho completo (docs/workflow.md + seção de fluxo/gates do CLAUDE.md) e confirma o entendimento
---

Carregue e internalize **todo o fluxo de trabalho combinado** deste projeto antes de agir. Este comando não inicia nenhuma história nem executa nenhuma ação — só lê e resume.

## Passos

1. **Leia integralmente `docs/workflow.md`** com a ferramenta Read (é a fonte canônica: XP/TDD, estratégia de branches, CI/branch protection, ciclo por história com os gates, slash commands, hooks, scripts de workflow e planejamento/corte de release). Leia o arquivo inteiro, não só um trecho.

2. **Releia, no `CLAUDE.md`, a seção "Fluxo de trabalho"** (resumo + **"Regras-gate inegociáveis"**) — é o ponteiro canônico que complementa o `docs/workflow.md`.

3. **Confirme o entendimento** devolvendo um resumo curto (bullets) dos pontos-gate, sem reabrir discussão e sem começar tarefa nenhuma:
   - Toda história começa por `/story` (story file + branch a partir de `develop`); nada de código/teste antes.
   - **Gate de revisão:** não implementar (nem red phase) antes de `[x] História revisada`.
   - TDD red → green → refactor; testes sempre via `bash scripts/check.sh` (nunca `docker run … node` nem `go test` crus).
   - **Critérios de aceitação:** o driver não os marca — só o `check.sh` marca o 1º (verdes); os demais e `[x] Aprovado` são do navigator via `scripts/story-approval.sh`. Preencher `## Revisão` sem tocar nos checkboxes.
   - **Gate de aprovação:** nenhum commit antes de `[x] Aprovado`; commit via `scripts/commit.sh`, depois `scripts/push-pr.sh` (push + PR base `develop` + CI + merge), direto e sem perguntar.
   - Após `[x] História revisada`, a única interação com o navigator é o pedido de aprovação — o driver vai até o fim sem perguntar nada nem confirmar comandos.
   - Monitorar revisão/aprovação com `scripts/await-review.sh` / `scripts/await-approval.sh` rodando em background **rastreado pelo harness** (`run_in_background`), não `nohup`.
   - `master` e `develop` são protegidos — nunca commit/push direto; tudo via PR. O corte de release (`develop → master`, via `scripts/release-pr.sh`) só com ok explícito do navigator.

## Restrição

- **Não** crie story, branch, nem rode scripts de workflow ao executar este comando. O objetivo é só carregar o fluxo no contexto e confirmar que está alinhado.
