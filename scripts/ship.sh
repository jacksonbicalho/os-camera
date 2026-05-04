#!/usr/bin/env bash
set -euo pipefail

branch=$(git rev-parse --abbrev-ref HEAD)

if [ "$branch" = "master" ]; then
  echo "erro: já está na master — faça checkout de uma branch de feature primeiro" >&2
  exit 1
fi

# Extrai tipo e descrição do nome da branch (ex: feat/minha-feature)
if ! [[ "$branch" =~ ^([a-z]+)/(.+)$ ]]; then
  echo "erro: branch deve seguir o formato <tipo>/<descricao-em-kebab-case>" >&2
  exit 1
fi

type="${BASH_REMATCH[1]}"
desc="${BASH_REMATCH[2]//-/ }"

# Infere escopo a partir dos arquivos modificados
files=$(git diff --name-only HEAD; git ls-files --others --exclude-standard)

scope=""
if   echo "$files" | grep -q "^frontend/";           then scope="ui"
elif echo "$files" | grep -q "^internal/server/";    then scope="server"
elif echo "$files" | grep -q "^internal/config/";    then scope="config"
elif echo "$files" | grep -q "^internal/motion/";    then scope="motion"
elif echo "$files" | grep -q "^internal/recorder/";  then scope="recorder"
elif echo "$files" | grep -q "^internal/streaming/"; then scope="streaming"
elif echo "$files" | grep -q "^internal/storage/";   then scope="storage"
elif echo "$files" | grep -q "^internal/ffprobe/";   then scope="ffprobe"
elif echo "$files" | grep -q "^cmd/";                then scope="cmd"
fi

if [ -n "$scope" ]; then
  header="$type($scope): $desc"
else
  header="$type: $desc"
fi

echo "→ $header"
echo ""

git add -A

git commit -m "$header

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"

git checkout master
git merge --no-ff "$branch" -m "Merge $branch into master"

echo ""
echo "✓ mergado em master"
