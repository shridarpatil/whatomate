@echo off
title Whatomate - Restart Docker Services
echo ========================================
echo Restarting Whatomate Docker Services
echo ========================================
echo.

docker-compose -f docker-compose.dev.yml restart

if %ERRORLEVEL% EQU 0 (
    echo.
    echo [OK] All services restarted
    echo [INFO] Wait a few seconds for services to be ready
) else (
    echo.
    echo [ERROR] Failed to restart services
)

echo.
pause
