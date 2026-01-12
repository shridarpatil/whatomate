@echo off
title Whatomate - Stop Docker Services
echo ========================================
echo Stopping Whatomate Docker Services
echo ========================================
echo.

docker-compose -f docker-compose.dev.yml stop

if %ERRORLEVEL% EQU 0 (
    echo.
    echo [OK] All services stopped
    echo [INFO] Data is preserved. Use START_DOCKER_WINDOWS.bat to start again.
) else (
    echo.
    echo [ERROR] Failed to stop services
)

echo.
pause
