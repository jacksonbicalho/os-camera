---
description: Abre PR de release develop → master via scripts/release-pr.sh (valida release file e pré-condições)
argument-hint: [arquivo de release ou versão, ex: v0.30.1]
---

Abre o PR de release de `develop` para `master`. Toda a lógica vive em
`scripts/release-pr.sh` (versionado, idempotente) — este comando só o invoca e reporta.

## Passos

1. **Rode o script:**
   ```bash
   bash scripts/release-pr.sh $ARGUMENTS
   ```
   Ele: localiza o `releases/*_next.md` (ou o arquivo/versão do argumento), valida que
   **todas** as histórias estão `[✓]`, checa as pré-condições git (develop sincronizado,
   tree limpa, à frente de master), calcula a versão estimada (bump convencional) e abre o
   PR `develop → master` com o corpo listando os PRs. Se já houver PR aberto, imprime a URL
   e sai (idempotente). **Não mergeia.**

2. **Reporte** a URL do PR ao navigator e diga que está pronto para ele aprovar/mergear no
   GitHub; depois é `/release-tag`.

## Restrições

- NÃO mergeie o PR — apenas abra (o script já não mergeia). O navigator aprova manualmente.
- Se o script abortar (histórias pendentes, tree suja, develop atrás), relate o motivo —
  não contorne com `gh pr create` manual.
