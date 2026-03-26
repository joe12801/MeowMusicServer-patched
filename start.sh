#!/bin/bash
set -e
cd "$(dirname "$0")"
export PATH=/usr/local/go/bin:$PATH
export HOME=/root
export GOPATH=/root/go
export GOMODCACHE=/root/go/pkg/mod
export GOCACHE=/root/.cache/go-build

if ! command -v go >/dev/null 2>&1; then
  echo "[错误] 未找到Go语言环境！"
  exit 1
fi

mkdir -p "$GOMODCACHE"
mkdir -p "$GOCACHE"

if [ ! -f ".env" ]; then
  echo "[警告] 未找到 .env 文件，将使用默认配置"
else
  echo "[信息] 已加载 .env 文件"
fi

go mod tidy
/usr/local/go/bin/go build -o meowmusicserver .
exec ./meowmusicserver
