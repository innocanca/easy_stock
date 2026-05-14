#!/usr/bin/env bash
# 一键启动 easystock-api（:4000）+ easyStock Vite（:5173）
# Windows：请用 Git Bash 或 WSL 执行：bash start.sh  （PowerShell 请用 .\start.ps1）
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

for cmd in go npm; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "error: '$cmd' not found in PATH" >&2
    exit 1
  fi
done

# 首次拉依赖在国内网络可按需使用（可被环境变量覆盖）
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"

API_PID=""
cleanup() {
  if [[ -n "$API_PID" ]] && kill -0 "$API_PID" 2>/dev/null; then
    echo ""
    echo "Stopping API (pid $API_PID)..."
    kill "$API_PID" 2>/dev/null || true
    wait "$API_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

echo "Starting easystock-api on :4000 ..."
cd "$ROOT/easystock-api"
go run ./cmd/server &
API_PID=$!
cd "$ROOT"

# 给 API 一点时间监听端口（可选）
sleep 1
if ! kill -0 "$API_PID" 2>/dev/null; then
  echo "error: API process exited early. Check easystock-api logs above." >&2
  exit 1
fi

echo "Starting easyStock (Vite) on :5173 ..."
cd "$ROOT/easyStock"
npm run dev
