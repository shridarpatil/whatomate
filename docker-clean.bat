@echo off
color 0C
title Whatomate - Clean Docker Setup
echo ========================================
echo Whatomate Docker - CLEAN EVERYTHING
echo ========================================
echo.
echo WARNING: This will:
echo   - Stop all containers
echo   - Remove all containers
echo   - Remove Docker volumes (DATABASE WILL BE DELETED!)
echo.
echo Press Ctrl+C to cancel, or
pause

echo.
echo [INFO] Stopping and removing containers...
docker-compose -f docker-compose.dev.yml down -v

if %ERRORLEVEL% EQU 0 (
    echo.
    echo [OK] All containers and volumes removed
    echo [INFO] Your database has been deleted
    echo [INFO] Run START_DOCKER_WINDOWS.bat to start fresh
) else (
    echo.
    echo [ERROR] Failed to clean up
)

echo.
pause
