#!/bin/bash

set -e

git submodule foreach '
  git fetch

  COUNT=$(git log --oneline HEAD..origin/master | wc -l)
  OLDEST=$(git log --format="%cr" --reverse HEAD..origin/master | head -n 1)

  echo "$COUNT commits found ($OLDEST)"
'
