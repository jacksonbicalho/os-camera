---
description: Cria story file + branch a partir de develop seguindo o fluxo XP/TDD do CLAUDE.md
argument-hint: <tipo>(<escopo>): <descrição curta>
---

Crie uma story file e branch para iniciar uma nova história, seguindo estritamente o fluxo XP/TDD descrito em CLAUDE.md.

**Input do navigator:** `$ARGUMENTS`

## Passos

1. **Parse o input.**
   - Esperado: `<tipo>(<escopo>): <descrição>` (ex: `fix(motion): bbox desalinhado`) OU apenas `<tipo>: <descrição>` OU apenas `<descrição>`.
   - Se o tipo não estiver claro, use AskUserQuestion com as opções: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`.
   - Se o escopo não estiver claro mas o tipo sim, prossiga sem escopo.

2. **Gere o slug.** A partir da descrição, crie um slug kebab-case curto (max ~40 chars, sem stopwords). Ex: "bbox desalinhado no high-res" → `bbox-desalinhado-hires`.

3. **Verifique pré-condições.**
   - `git branch --show-current` deve ser `develop` (ou aceite trocar — pergunte).
   - Working tree limpa (sem arquivos modificados rastreados).
   - Se algo falhar, aborte e explique.

4. **Sincronize develop.** `git fetch origin develop && git pull origin develop --ff-only`.

5. **Crie a branch.** `git checkout -b <tipo>/<slug>` (sem escopo no nome da branch — só `<tipo>/<slug>`).

6. **Crie a story file.** Em `stories/YYYYMMDDHHmm_<tipo>_<slug_underscore>.md` (timestamp via `date +%Y%m%d%H%M`, slug com `_` em vez de `-`):

```markdown
# <tipo>(<escopo>): <descrição>

## Contexto

<deixar em branco pra eu preencher após investigar>

## Solução

<deixar em branco>

## Critérios de Aceitação

- [ ] <critério 1>
- [ ] <critério 2>

## Revisão

- [ ] Aprovado
```

7. **Reporte.** Confirme branch ativa, caminho do story file, e indique que está pronto pra eu começar a investigar/escrever testes (red phase).

## Restrições

- NÃO comece a implementar — só prepare o ambiente.
- NÃO commite a story file ainda (será junto com a implementação).
- Se o navigator pedir uma branch a partir de master ou de outra branch que não develop, confirme com AskUserQuestion antes.
