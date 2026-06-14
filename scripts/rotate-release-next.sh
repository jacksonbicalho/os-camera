#!/usr/bin/env bash
# Rotaciona o arquivo de planejamento de release no corte de uma versão.
#
# Opera apenas sobre a pasta releases/ (sem git/gh) — testável em diretório
# temporário via RELEASES_DIR. Chamado pelo release-tag.sh após a release
# publicar, com a versão recém-criada.
#
#   (a) carimba o `*_next.md` atual com a versão publicada e o renomeia para
#       `<timestamp>_<version>.md` (cada arquivo = uma release publicada);
#   (b) cria um novo `<agora>_next.md` com a versão recém-publicada carimbada no
#       topo como base do próximo ciclo.
#
# Uso: scripts/rotate-release-next.sh <version>   (ex.: v0.12.0-dev)
set -euo pipefail

version="${1:-}"
[ -z "$version" ] && { echo "Uso: rotate-release-next.sh <version>" >&2; exit 2; }

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELEASES_DIR="${RELEASES_DIR:-$ROOT/releases}"
mkdir -p "$RELEASES_DIR"

today="$(date +%Y%m%d)"
date_iso="$(date +%Y-%m-%d)"

# (a) carimbar + renomear o _next.md atual (o mais recente, se houver mais de um)
current="$(ls -t "$RELEASES_DIR"/*_next.md 2>/dev/null | head -1 || true)"
if [ -n "$current" ]; then
    ts="$(basename "$current" | sed 's/_next\.md$//')"
    published="$RELEASES_DIR/${ts}_${version}.md"
    {
        echo "# Release ${version} — ${today}"
        echo
        echo "> Publicada: ${version} em ${date_iso}"
        echo
        # corpo original, sem o H1 antigo (linha 1, se for título)
        sed '1{/^# /d;}' "$current"
    } > "$published"
    rm -f "$current"
    echo "carimbado: $(basename "$published")"
else
    echo "aviso: nenhum *_next.md encontrado para carimbar" >&2
fi

# (b) criar o novo _next.md, com a versão recém-publicada como base
new_next="$RELEASES_DIR/$(date +%Y%m%d%H%M)_next.md"
cat > "$new_next" <<EOF
# Release next — ${today}

> Base: ${version} (publicada em ${date_iso})

## Histórias

| Status | Descrição | Branch | PR |
|--------|-----------|--------|----|
EOF
echo "criado: $(basename "$new_next")"
