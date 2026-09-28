@echo off
chcp 65001 >nul
title 小果短剧 Web 应用

echo ========================================================
echo       小果短剧 (Xiaoguo) 独立 Web 应用 (Go + Vue 3)
echo ========================================================
echo.

cd /d "%~dp0"

if not exist "server\xiaoguo-server.exe" (
    echo [1/2] 正在编译后端服务...
    cd server
    go build -o xiaoguo-server.exe .
    cd ..
)

if not exist "web\dist\index.html" (
    echo [2/2] 正在构建前端资源...
    cd web
    call pnpm run build || call npm run build
    cd ..
)

echo.
echo 启动后端服务并提供前端访问...
echo 浏览器访问: http://localhost:8080
echo.

start http://localhost:8080
cd server
xiaoguo-server.exe -port 8080 -web ../web/dist
pause
