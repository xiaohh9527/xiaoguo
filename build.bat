@echo off
chcp 65001 >nul
title 构建小果短剧 Web 应用

echo ========================================================
echo       开始构建全栈 Web 应用 (Go + Vue 3)
echo ========================================================
echo.

cd /d "%~dp0"

echo [1/2] 正在编译前端 Vue 应用...
cd web
call pnpm run build || call npm run build
if %errorlevel% neq 0 (
    echo [错误] 前端构建失败！
    pause
    exit /b %errorlevel%
)
cd ..

echo.
echo [2/2] 正在编译后端 Go 二进制服务...
cd server
go build -ldflags="-s -w" -o xiaoguo-server.exe .
if %errorlevel% neq 0 (
    echo [错误] 后端构建失败！
    pause
    exit /b %errorlevel%
)
cd ..

echo.
echo ========================================================
echo  构建完成！可以直接运行 start.bat 或 server\xiaoguo-server.exe
echo ========================================================
pause
