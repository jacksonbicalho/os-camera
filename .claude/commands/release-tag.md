---
description: Gera a tag de release rodando ./scripts/release.sh após o PR develop→master ser mergeado
---

Cria a tag de release executando o script `./scripts/release.sh`, após o PR `release: vX.Y.Z` ter sido mergeado.

## Passos

1. **Verifique pré-condições.**
   - `git branch --show-current` deve ser `master`. Se não estiver, faça `git checkout master`.
   - `git fetch origin master && git pull origin master --ff-only`.
   - Working tree limpa.
   - Último commit deve ser do tipo `release: vX.Y.Z (#NNN)`. Se não for, alerte o navigator (pode ser que o release PR não foi mergeado ainda).

2. **Rode dry-run primeiro.** `./scripts/release.sh --dry-run` — mostre o resumo.

3. **Pergunte confirmação.** Use AskUserQuestion: "Criar tag vX.Y.Z-rc.N?" com opções `Sim` / `Cancelar`.

4. **Crie a tag.** Se confirmado: `echo "s" | ./scripts/release.sh`.

5. **Atualize o release file.** Edite `releases/*_vX.Y.Z.md` adicionando ao final:

```markdown

## Tag

- vX.Y.Z-rc.N publicada em YYYY-MM-DDTHH:MM:SSZ
- GitHub Actions: https://github.com/jacksonbicalho/camera/actions
```

6. **Reporte.** Confirme tag criada, link do release no GitHub e que o workflow vai gerar o binário automaticamente.

## Restrições

- NÃO crie a tag manualmente via `git tag` — use sempre o script (mantém o changelog consistente).
- NÃO force-push tags (`git push --force --tags`) — se algo deu errado, abra discussão com o navigator.
- Se o script reportar "Nenhum commit desde X. Nada para versionar.", significa que master não tem commits novos pra release; aborte.
- Conhecido: bug do `git describe` no script às vezes pega tag errada como "última". Se a versão proposta parecer muito alta, alerte e pergunte antes de criar.
