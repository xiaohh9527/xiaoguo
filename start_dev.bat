@echo off
chcp 65001 >nul
title 小果短剧 Web 开发模式

echo 正在启动 Go 后端 (Port 8080) 与 Vite 前端 (Port 5173)...

start "Xiaoguo Backend" cmd /k "cd server && go run main.go -port 8080"
timeout /t 2 >nul
start "Xiaoguo Frontend" cmd /k "cd web && pnpm run dev || npm run dev"

timeout /t 3 >nul
start http://localhost:5173
