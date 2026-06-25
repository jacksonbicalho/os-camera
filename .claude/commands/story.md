---
description: Cria story file + branch a partir de develop seguindo o fluxo XP/TDD do CLAUDE.md
argument-hint: <descrição livre da história>
---

Crie uma story file e branch para iniciar uma nova história, seguindo estritamente o fluxo XP/TDD descrito em CLAUDE.md.

**Input do navigator:** `$ARGUMENTS` — apenas uma **descrição livre**. Você (Claude) decide tipo, escopo e descrição curta.

## Passos

1. **Decida tipo, escopo e descrição.** A partir da descrição livre, infira:
   - o **tipo**: `feat` (nova funcionalidade), `fix` (bug), `refactor`, `test`, `docs`, `chore` (build/config/tooling);
   - o **escopo** (opcional): o pacote/área afetada, se óbvio (ex: `motion`, `deviceinfo`, `timeline`);
   - uma **descrição curta** em pt-BR para o título.
   - **Não pergunte ao navigator** — decida. Só use AskUserQuestion se a descrição for genuinamente ambígua a ponto de mudar o tipo/escopo.

2. **Gere o slug.** Kebab-case curto a partir da descrição (max ~40 chars, sem stopwords). Ex: "bbox desalinhado no high-res" → `bbox-desalinhado-hires`.

3. **Verifique pré-condições.**
   - `git branch --show-current` deve ser `develop` (ou aceite trocar — pergunte).
   - Working tree limpa (sem arquivos modificados rastreados).
   - Se algo falhar, aborte e explique.

4. **Sincronize develop.** `git fetch origin develop && git pull origin develop --ff-only`.

5. **Crie a branch.** `git checkout -b <tipo>/<slug>` (sem escopo no nome da branch — só `<tipo>/<slug>`).

6. **Crie a story file** com o **plano COMPLETO**. Em `stories/YYYYMMDDHHmm_<tipo>_<slug_underscore>.md` (timestamp via `date +%Y%m%d%H%M`, slug com `_` em vez de `-`). **Investigue antes e preencha Contexto e Solução — NUNCA deixe em branco**; o navigator revisa o plano, então o plano precisa existir, com arquivos/abordagem/decisões de escopo. O **primeiro critério é sempre o "verdes"** (auto-marcado por `scripts/check.sh`):

```markdown
# <tipo>(<escopo>): <descrição>

## Contexto

<o problema e o estado atual, INVESTIGADO — arquivos/funções relevantes, por que mexer>

## Solução

<o plano COMPLETO: arquivos a tocar, abordagem, decisões de escopo a revisar — nunca em branco>

## Critérios de Aceitação

- [] Backend e frontend verdes (auto: `scripts/check.sh`)
- [] <critério 2>
- [] <critério 3>

## Revisão da história (antes de implementar)

- [] História revisada

## Revisão

- [] Aprovado
```

7. **Reporte e aguarde a revisão da história.** Confirme branch ativa e caminho do story file. **NÃO inicie a implementação (nem o red phase).** A história precisa ser revisada pelo navigator antes: peça para ele revisar e marcar `[x] História revisada` na story. Rode **`scripts/await-review.sh` em background** (bloqueia até `[x] História revisada`) e só prossiga para o passo 8 quando ele retornar.

8. **Red phase (somente após `[x] História revisada`).** Aí sim escreva o teste que falha e siga o ciclo TDD red → green → refactor do CLAUDE.md.

## Restrições

- NÃO comece a implementar — só prepare o ambiente.
- NÃO commite a story file ainda (será junto com a implementação).
- **NÃO inicie a implementação antes de o navigator marcar `[x] História revisada`.** Monitore o arquivo em background.
- **Contexto e Solução SEMPRE preenchidos antes da revisão** — escopo/ambiguidade se resolve no plano (use AskUserQuestion *antes* da revisão, se preciso), nunca depois.
- **Após `[x] História revisada`, a ÚNICA interação com o navigator é o pedido de aprovação.** O driver vai até o fim sem perguntar nada e **sem confirmar nenhum comando/execução** (check.sh, scripts, builds, git rodam direto).
- Ao final, ao pedir `./scripts/story-approval.sh`, rode **`scripts/await-approval.sh` em background** e **só siga** (commit/push/PR/merge) quando ele retornar (todos os critérios E `[x] Aprovado` marcados).
- Se o navigator pedir uma branch a partir de master ou de outra branch que não develop, confirme com AskUserQuestion antes.
