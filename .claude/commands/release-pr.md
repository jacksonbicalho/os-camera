---
description: Abre PR de release develop → master após validar que todas as histórias do release file estão mergeadas
argument-hint: [arquivo de release ou versão, ex: v0.30.1]
---

Abre o PR de release de `develop` para `master`, seguindo o fluxo de release em lote do CLAUDE.md.

**Input opcional:** `$ARGUMENTS` (nome do arquivo de release ou versão; se vazio, usa o mais recente).

## Passos

1. **Localize o arquivo de release.**
   - Sem argumento: pegue o mais recente em `releases/` (ordenação por timestamp do nome).
   - Com argumento `vX.Y.Z`: procure `releases/*_vX.Y.Z.md`. Se não achar, aborte com mensagem.
   - Com argumento de caminho: use direto.

2. **Extraia a versão** do nome do arquivo (`releases/YYYYMMDDHHmm_vX.Y.Z.md` → `vX.Y.Z`).

3. **Valide o release file.**
   - Leia o arquivo.
   - Confirme que TODAS as linhas da tabela `| Status | ... |` estão `[✓]` (mergeado).
   - Se alguma estiver `[ ]`, `[~]`, ou `[x]`, aborte imprimindo a lista das pendentes.

4. **Verifique pré-condições git.**
   - Working tree limpa (sem mods em arquivos rastreados).
   - `git checkout develop && git fetch origin develop && git pull origin develop --ff-only`.
   - Confirme que develop está à frente de master: `git rev-list --count master..develop` > 0.
   - Se develop estiver atrás de master (raro), aborte.

5. **Determine a versão real** rodando o dry-run do script de release:
   ```bash
   ./scripts/release.sh --dry-run 2>&1 | grep "Nova versão:" | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+-rc\.[0-9]+'
   ```
   Use esta versão no título do PR — **não** a versão do nome do release file, que é apenas uma estimativa.

6. **Abra o PR.** Use HEREDOC pro body:

```bash
gh pr create --title "release: vX.Y.Z" --base master --head develop --body "$(cat <<'EOF'
## Histórias incluídas

<lista de PRs do release file, formato: - #NNN <descrição>>

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

7. **Reporte.** Mostre URL do PR e diga que está pronto pro navigator aprovar no GitHub.

## Restrições

- NÃO mergeie o PR — apenas abra. O navigator aprova manualmente.
- NÃO use `--base master` em PR de feature; este é o único caso autorizado pelo hook (develop → master).
- Se o arquivo de release tiver linhas em branco no meio da tabela, ignore-as.
- Se o PR já existir (`gh pr list --base master --head develop --state open`), mostre a URL existente em vez de criar duplicado.
