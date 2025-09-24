#!/bin/bash
mkdir -p versions
STAMP=$(date +"%Y-%m-%d_%H-%M-%S")
COMMENT=$1
if [ -n "$COMMENT" ]; then
  COMMENT="_$(echo $COMMENT | tr ' ' '_')"
fi
cp main.go "versions/main_${STAMP}${COMMENT}.go"
echo "✅ Сохранено: versions/main_${STAMP}${COMMENT}.go"
