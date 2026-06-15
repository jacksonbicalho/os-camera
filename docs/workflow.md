# Fluxo de trabalho — desenvolvimento e publicação

> Fluxo completo do projeto (XP/TDD, branches, CI, ciclo por história com gates, scripts, release).
> Referenciado pelo `CLAUDE.md` e lido no início da sessão (hook `session-start`).

O desenvolvimento segue **XP (Extreme Programming)** com **TDD red → green → refactor**:

- O **navigator** (usuário) define a história, revisa o código e aprova cada etapa.
- O **driver** (Claude) implementa, sempre guiado pelos testes.

> ⚠️ **`master` e `develop` são protegidos.** Push direto é bloqueado em ambos. Features entram via PR para `develop`; o PR `develop → master` acontece apenas no momento de release. Nunca commite ou force-push diretamente em nenhum dos dois.

### Estratégia de branches

```
master   ← PRs de release (develop → master)
  ↑
develop  ← PRs de feature/fix/chore
  ↑
feat/xyz · fix/abc · chore/def  ← branches de história
```

- Branches de história partem **sempre de `develop`**: `git checkout -b <tipo>/<desc> develop`
- PRs de história têm **`develop` como base**: `gh pr create --base develop`
- PRs de release têm **`master` como base**: `gh pr create --base master --head develop`
- **Ciclo de vida da branch:** deletada imediatamente após o merge em `develop` — remoto automaticamente pelo GitHub, local com `git branch -d <branch>` no passo de merge em lote.

### CI e branch protection

Todo PR para `master` ou `develop` dispara `.github/workflows/ci.yml` com dois jobs paralelos:
- **Backend**: `go test ./...` + `go build ./...`
- **Frontend**: `yarn lint` + `yarn test --run` + `yarn build`

Ambos os branches estão protegidos: push direto bloqueado, PR obrigatório, checks `Backend` e `Frontend` obrigatórios. `master` exige 1 aprovação humana; `develop` não exige aprovação (projeto solo — CI já é a barreira de qualidade). Para reaplicar as regras (ex: em novo repositório):

```bash
# master — só aceita PRs vindos de develop (releases); exige 1 aprovação humana
gh api repos/{owner}/{repo}/branches/master/protection \
  --method PUT \
  --header "Accept: application/vnd.github+json" \
  --input - <<'EOF'
{
  "required_status_checks": { "strict": true, "contexts": ["Backend", "Frontend"] },
  "enforce_admins": true,
  "required_pull_request_reviews": { "dismiss_stale_reviews": true, "required_approving_review_count": 1 },
  "restrictions": null
}
EOF

# develop — aceita PRs de feature branches; CI obrigatório, sem aprovação humana
gh api repos/{owner}/{repo}/branches/develop/protection \
  --method PUT \
  --header "Accept: application/vnd.github+json" \
  --input - <<'EOF'
{
  "required_status_checks": { "strict": true, "contexts": ["Backend", "Frontend"] },
  "enforce_admins": false,
  "required_pull_request_reviews": { "dismiss_stale_reviews": true, "required_approving_review_count": 0 },
  "restrictions": null
}
EOF
```

### Fluxo por história

> ⚠️ **OBRIGATÓRIO:** Antes de escrever qualquer linha de código ou teste, o driver DEVE criar o arquivo de história E abrir a branch (use `/story`). Sem exceção — nem para bugs simples, nem para "pequenas correções".

1. Criar `stories/YYYYMMDDHHmm_<descricao>.md` e abrir uma branch a partir de `develop`: `git checkout -b <tipo>/<descricao-curta> develop`. Se a história cobrir mais de um assunto independente, questionar o navigator antes de continuar — histórias devem ser pequenas e focadas. O 1º Critério de Aceitação é **sempre** `- [] Backend e frontend verdes (auto: scripts/check.sh)` (sem "se aplicável").
2. **Revisão da história (antes de implementar).** A história inclui o campo `- [] História revisada`. O driver apresenta o plano e **aguarda o navigator marcar `[x] História revisada`** — monitorando o arquivo da story em background (grep case-insensitive). **Nenhuma linha de código ou teste antes disso.**

   > ⚠️ **O driver NÃO inicia o red phase enquanto `História revisada` não estiver `[x]`.** O navigator revisa o plano (Contexto/Solução/Critérios) primeiro; só então a implementação começa.

3. Escrever o teste que falha (**red**) — nunca escrever código de produção sem um teste falhando antes.
4. Implementar o mínimo para o teste passar (**green**).
5. Refatorar se necessário, mantendo os testes verdes (**refactor**).
6. Executar a suíte de testes (obrigatório antes de apresentar ao navigator):
   - Backend: `go test ./...` + `go build ./...`
   - Frontend: `yarn lint` + `yarn test --run` + `yarn build` (em `frontend/`)
   - Nunca prosseguir se qualquer um desses falhar.
7. Adicionar seção `## Revisão` na história e **aguardar aprovação explícita do navigator**.

   > ⚠️ **Nenhum commit pode ser feito antes de o navigator marcar `[x] Aprovado`.** O driver apresenta o resultado, o navigator revisa o código e aprova — só então o commit acontece.

   > ⚠️ **O driver NÃO marca os Critérios de Aceitação.** Apenas o **1º critério** (verdes) é marcado automaticamente pelo `scripts/check.sh`. **Todos os demais critérios** e o `[x] Aprovado` são marcados pelo **navigator** via `scripts/story-approval.sh`. O driver pode preencher a seção `## Revisão`, mas **sem tocar nos checkboxes dos critérios**.

8. Com a aprovação do navigator, o ciclo roda **direto, sem novas perguntas** — mas o driver **só segue quando TODOS os Critérios de Aceitação E `[x] Aprovado` estiverem marcados** (apenas monitorar o arquivo após pedir `scripts/story-approval.sh`): commitar (mensagem semântica) → `scripts/push-pr.sh`, que **orquestra o ciclo inteiro** — push + abre o PR (**sempre `--base develop`**, nunca `master`) + aguarda o CI + mergeia quando verde (chama o `merge-when-green.sh` internamente). A aprovação da story autoriza, de uma vez, commit + push + PR + merge-quando-verde. **CI vermelho:** o `push-pr.sh` propaga o erro sem mergear (PR fica aberto) — **Política A**: um fix trivial pós-aprovação (deixar o CI verde) não exige nova aprovação; o driver corrige, avisa o que mudou, commita e roda `push-pr.sh` de novo (idempotente: não recria o PR, só reespera o CI). Se o fix mexer em lógica de produção, volta pro navigator. Use `scripts/push-pr.sh --no-merge` para só abrir o PR sem mergear. **Apenas o corte de release** (`develop → master`) depende de autorização explícita do navigator.
9. Atualizar o arquivo de release correspondente em `releases/`: preencher a branch e o número do PR na tabela, marcar `[~]`; o `merge-when-green.sh` marca `[~]→[✓]` ao mergear em `develop`. O **corte de release** (PR `develop → master`) é o passo que aguarda o navigator liberar.

### Histórias

Histórias ficam em `stories/` (gitignored). O nome do arquivo usa timestamp no formato
`YYYYMMDDHHmm_<descricao>.md` — igual às migrations de banco — garantindo ordenação
cronológica natural ao listar o diretório.

Ao iniciar uma nova história:
- Criar o arquivo `stories/YYYYMMDDHHmm_<descricao>.md` com contexto, critérios de aceitação e notas técnicas.
- Ao concluir a implementação, adicionar uma seção `## Revisão` no arquivo com checklist do que foi feito.
- **Nenhum commit pode ser feito sem aprovação explícita do navigator. Só proceder com commit ou PR após o navigator marcar `[x] Aprovado` na seção Revisão.**

### Commits semânticos

Formato: `<tipo>(<escopo opcional>): <descrição curta em inglês>`

| Tipo | Quando usar |
|---|---|
| `feat` | nova funcionalidade |
| `fix` | correção de bug |
| `refactor` | refatoração sem mudança de comportamento |
| `test` | adição ou correção de testes |
| `docs` | documentação |
| `chore` | configuração, build, dependências |

### Slash commands

O fluxo acima é automatizado pelos slash commands em `.claude/commands/`:

| Comando | O que faz |
|---|---|
| `/story <descrição livre>` | Cria story file + branch a partir de develop (passo 1 do fluxo). Passe **só a descrição**; Claude decide tipo/escopo/slug. |
| `/release-pr [vX.Y.Z]` | Valida release file e abre PR develop → master (após todas as histórias `[✓]`). |
| `/release-tag` | Roda `./scripts/release.sh` em master após o PR de release ser mergeado. |

Use os commands em vez de executar os passos manualmente — eles validam pré-condições e evitam erros (branch errada, working tree suja, status incompleto).

### Hooks de pre-commit (`.claude/settings.json`)

Hooks `PreToolUse` (matcher `Bash`, versionados no repo) impõem o fluxo automaticamente. O `settings.json` só **chama scripts** versionados em `scripts/hooks/` (a lógica vive lá, não inline):

| Gate | Script | Bloqueia quando |
|---|---|---|
| Aprovação da story | `scripts/hooks/story-approved.sh` | `git commit` em branch de história cuja story não tem `[x] Aprovado`. |
| Target do PR | `scripts/hooks/pr-target.sh` | `gh pr create --base master` a partir de branch que não seja `develop`/`release/*`. |
| Testes backend | `scripts/hooks/precommit-tests.sh` | `git commit` quando `go build ./...` ou `go test -count=1 ./...` falham. |

O gate de testes roda no host (Go instalado), **sem cache** (`-count=1`) — só execução limpa pega testes dependentes de `time.Now()` (o cache do Go não rastreia o relógio). Escopo é backend; o frontend segue coberto pelo CI. Hooks só recarregam no início da sessão do Claude Code: alterações em `settings.json` (ou nos scripts de hook) valem a partir da próxima sessão.

### Scripts de workflow (`scripts/`)

Encadeiam o fluxo por história. **Checkboxes usam `[]` para não-marcado** (e `[x]` para marcado).

| Script | Quem roda | O que faz |
|---|---|---|
| `check.sh` | Claude | "CI local": `go build`+`go test` sempre; `frontend-check.sh` se `frontend/` mudou (vs develop). Se tudo verde, marca o **1º Critério de Aceitação** da story `[x]`. |
| `story-approval.sh` | **Navigator** (`! scripts/story-approval.sh`) | Interativo: percorre os Critérios de Aceitação, pergunta sobre cada não-marcado e marca conforme a resposta; no fim, oferece marcar `[x] Aprovado`. |
| `await-review.sh [story]` | Claude (background) | Bloqueia até `[x] História revisada` na story (padrão ancorado, case-insensitive, imune a menções na prosa). Resolve a story pela branch atual. Usado no passo 2 do fluxo (gate de revisão antes de implementar). |
| `await-approval.sh [story]` | Claude (background) | Bloqueia até **nenhum** Critério de Aceitação desmarcado **e** `[x] Aprovado`. Usado no passo 8 (gate final antes de commit/push/PR/merge). |
| `commit.sh` | Claude | Commita o que está staged usando o heading `#` da story como mensagem (já é `tipo(escopo): desc`); exige `[x] Aprovado`; adiciona `Co-Authored-By`. |
| `push-pr.sh` | Claude | **Orquestra o ciclo pós-aprovação:** push + `gh pr create --base develop` + **registra a linha da história no `_next.md` com `[~]`** (idempotente por `#PR`) + aguarda o CI e mergeia (chama `merge-when-green.sh`). Só com tree limpa + story aprovada; idempotente (não recria PR; re-roda após fix). `--no-merge` só abre o PR sem mergear. CI vermelho: propaga erro sem mergear. |

### Merge pós-PR

`scripts/merge-when-green.sh <PR#>` colapsa o ciclo pós-PR numa única invocação (economia de tokens): aguarda o CI em silêncio, mergeia em `develop`, sincroniza o branch local e marca `[~]→[✓]` na linha do PR no release file que a contém (busca por conteúdo — funciona com `_next.md` e `_vX.Y.Z.md`). **Só remove o story file e deleta a branch da história QUANDO a marcação `[✓]` teve sucesso** — se a linha não existir no release file, preserva branch e story (nada se perde). Imprime só o resumo. **Recusa** PRs com base `master` (releases são aprovadas à mão) e é idempotente em PR já mergeado. É a primitiva chamada pelo `push-pr.sh` ao final, mas também pode ser invocada avulsa (ex.: retomar após um fix de CI num PR já aberto).

> ⚠️ **Limpeza gated em `[✓]`:** o story file (`stories/`) e a branch da história só são apagados quando a história está `[✓]` no release file. Como o `push-pr.sh` registra a linha com `[~]` ao abrir o PR e o `merge-when-green.sh` a marca `[✓]` ao mergear, isso roda sozinho — o driver **não edita mais o `_next.md` à mão**.

`scripts/release-tag.sh [--dry-run]` colapsa o **corte de release** (após o PR develop→master já mergeado): cria/envia a tag via `release.sh` (confirmação automática), aguarda o workflow Release publicar **em silêncio** (poll de `gh release view`), mergeia `master→develop` (passo pós-tag), **rotaciona o release file** (chama `rotate-release-next.sh`) e imprime uma linha (`RELEASED <versão> | assets: N | develop sincronizado | release file rotacionado`). `--dry-run` só mostra a versão que sairia. Substitui o ciclo manual com `gh run watch`.

`scripts/rotate-release-next.sh <version>` opera só sobre `releases/` (sem git/gh; testável via `RELEASES_DIR`): no corte, **(a)** carimba o `*_next.md` atual com `Publicada: <version>` e o renomeia para `<timestamp>_<version>.md` (cada arquivo = uma release publicada) e **(b)** cria um novo `<agora>_next.md` com `Base: <version>` (a recém-publicada) no topo. Chamado pelo `release-tag.sh`.

### Release

```bash
./scripts/release.sh             # calcula bump, gera changelog, cria tag anotada e faz push
./scripts/release.sh --dry-run   # prévia sem criar nada
```

O script lê os commits convencionais desde a última tag, determina o bump (`feat` → minor, breaking → major, resto → patch), gera o changelog agrupado por tipo e cria uma tag no formato `vX.Y.Z-dev`. O push da tag dispara o GitHub Actions que publica a release. O sufixo `-dev` indica projeto em desenvolvimento ativo; quando atingir estabilidade, as tags passarão a usar `vX.Y.Z` sem sufixo.

#### Planejamento de release (releases/)

`releases/` (gitignored) agrupa histórias em uma release antes de mergeá-las.

**Fluxo:**
1. O arquivo de planejamento se chama **`releases/YYYYMMDDHHmm_next.md`** (sem versão — o bump só é conhecido no corte). As histórias planejadas entram nele. No corte, o `rotate-release-next.sh` (via `release-tag.sh`) carimba esse `_next.md` com a versão publicada, renomeia para `<timestamp>_<version>.md` e abre um `_next.md` novo já com `Base: <version>` no topo. **Nunca nomear o arquivo de planejamento com a versão na frente.**
2. Ao concluir cada história, preencher branch e PR na tabela e marcar `[~]` (aguardando aprovação no GitHub — PR targeta `develop`).
3. Após aprovação no GitHub, marcar `[x]`.
4. Quando todas estiverem `[x]`, o navigator diz **"pode mergear a release"** — Claude itera a lista, mergeia cada PR em `develop` em sequência, deleta a branch local (`git branch -d <branch>`) e marca `[✓]`. O GitHub deleta a branch remota automaticamente após o merge (setting "Automatically delete head branches" ativo).
5. Após todos os merges em `develop`, Claude abre um PR `develop → master` com título `release: vX.Y.Z`.
6. Após aprovação e merge do PR de release, Claude roda `./scripts/release.sh` para gerar a tag.
7. **Após a tag ser criada**, mergear `master` de volta em `develop` para que `git describe` retorne a versão correta no modo dev:
   ```bash
   git checkout develop && git fetch origin master && git merge origin/master --no-edit && git push origin develop
   ```
   Sem este passo, `git describe` não encontra a tag (que vive no merge commit de master) e retorna a versão anterior.

**Estrutura do arquivo de release:**

```markdown
# Release vX.Y.Z — YYYYMMDD

## Histórias

| Status | Descrição | Branch | PR |
|--------|-----------|--------|----|
| [ ]    | descrição | `branch-name` | — |
| [~]    | descrição | `branch-name` | #123 |
| [x]    | descrição | `branch-name` | #123 |
| [✓]    | descrição | `branch-name` | #123 |
```

**Legenda de status:** `[ ]` planejada · `[~]` aguardando aprovação no GitHub · `[x]` aprovada · `[✓]` mergeada.
