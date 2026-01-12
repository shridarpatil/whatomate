@echo off
title Whatomate Backend Server
echo ========================================
echo Starting Whatomate Backend
echo ========================================
echo.

REM Check if config exists
if not exist "config.toml" (
    echo [WARNING] config.toml not found. Creating from example...
    copy config.example.toml config.toml
    echo [OK] Created config.toml
    echo.
    echo [INFO] Edit config.toml with your database settings before running.
    pause
    exit /b 1
)

echo [INFO] Starting backend server on port 8080...
echo [INFO] Press Ctrl+C to stop
echo.

go run cmd/whatomate/main.go -config config.toml

if %ERRORLEVEL% NEQ 0 (
    echo.
    echo [ERROR] Backend failed to start
    echo [INFO] Check the error messages above
    pause
)
