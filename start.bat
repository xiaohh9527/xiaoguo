@echo off
@cd /d "%~dp0"
chcp 65001 >nul
title 小果短剧 Web 应用

echo ========================================================
echo       小果短剧 (Xiaoguo) 独立 Web 应用 (Go + Vue 3)
echo ========================================================
echo.

if exist "%~dp0server\xiaoguo-server.exe" goto check_frontend
echo [1/2] 正在编译后端服务...
cd /d "%~dp0server"
go build -o xiaoguo-server.exe .
cd /d "%~dp0"

:check_frontend
if exist "%~dp0web\dist\index.html" goto run_app
echo [2/2] 正在构建前端资源...
cd /d "%~dp0web"
call pnpm run build || call npm run build
cd /d "%~dp0"

:run_app
echo.
echo 启动后端服务并提供前端访问...
echo 浏览器访问: http://localhost:8080
echo.

start "" http://localhost:8080
cd /d "%~dp0server"
xiaoguo-server.exe -port 8080 -web ../web/dist
pause
