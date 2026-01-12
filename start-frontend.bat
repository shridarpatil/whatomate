@echo off
title Whatomate Frontend Dev Server
echo ========================================
echo Starting Whatomate Frontend
echo ========================================
echo.

cd frontend

REM Check if node_modules exists
if not exist "node_modules" (
    echo [INFO] Installing frontend dependencies...
    echo [INFO] This may take a few minutes...
    call npm install
    if %ERRORLEVEL% NEQ 0 (
        echo [ERROR] Failed to install dependencies
        pause
        exit /b 1
    )
    echo.
)

echo [INFO] Starting frontend dev server on port 5173...
echo [INFO] Press Ctrl+C to stop
echo.

call npm run dev

if %ERRORLEVEL% NEQ 0 (
    echo.
    echo [ERROR] Frontend failed to start
    pause
)
