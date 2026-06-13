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

6. **Crie a story file.** Em `stories/YYYYMMDDHHmm_<tipo>_<slug_underscore>.md` (timestamp via `date +%Y%m%d%H%M`, slug com `_` em vez de `-`). O **primeiro critério é sempre o "verdes"** (auto-marcado por `scripts/check.sh`):

```markdown
# <tipo>(<escopo>): <descrição>

## Contexto

<deixar em branco pra eu preencher após investigar>

## Solução

<deixar em branco>

## Critérios de Aceitação

- [] Backend e (se aplicável) frontend verdes (auto: `scripts/check.sh`)
- [] <critério 2>
- [] <critério 3>

## Revisão

- [] Aprovado
```

7. **Reporte.** Confirme branch ativa, caminho do story file, e indique que está pronto pra começar a investigar/escrever testes (red phase).

## Restrições

- NÃO comece a implementar — só prepare o ambiente.
- NÃO commite a story file ainda (será junto com a implementação).
- Se o navigator pedir uma branch a partir de master ou de outra branch que não develop, confirme com AskUserQuestion antes.
