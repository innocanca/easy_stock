# 一键启动 API :4000 + 前端 Vite :5173（PowerShell）
$ErrorActionPreference = "Stop"
$Root = $PSScriptRoot

$env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" +
  [System.Environment]::GetEnvironmentVariable("Path", "User")
if (-not $env:GOPROXY) { $env:GOPROXY = "https://goproxy.cn,direct" }

$go = Get-Command go -ErrorAction SilentlyContinue
if (-not $go) {
  Write-Error "go not found. Install Go or restart the terminal after installation."
}

Write-Host "Starting easystock-api on :4000 ..."
$apiDir = Join-Path $Root "easystock-api"
# API 日志输出到当前控制台（若看不到，可单独开终端跑 easystock-api）
$api = Start-Process -FilePath "go" -ArgumentList "run", "./cmd/server" `
  -WorkingDirectory $apiDir -PassThru -NoNewWindow

Start-Sleep -Seconds 1
if ($api.HasExited) {
  Write-Error "API exited early. Check easystock-api configuration (e.g. .env)."
}

try {
  Write-Host "Starting easyStock on :5173 ..."
  Set-Location (Join-Path $Root "easyStock")
  & npm.cmd run dev
}
finally {
  if ($api -and -not $api.HasExited) {
    Write-Host "`nStopping API (pid $($api.Id))..."
    Stop-Process -Id $api.Id -Force -ErrorAction SilentlyContinue
  }
}
