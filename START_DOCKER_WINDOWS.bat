@echo off
color 0A
title Whatomate - Docker Setup for Windows
cls

echo ============================================================
echo      WHATOMATE - DOCKER SETUP FOR WINDOWS
echo ============================================================
echo.
echo This script will start Whatomate using Docker Desktop.
echo All services (PostgreSQL, Redis, Backend, Frontend) will
echo run in Docker containers.
echo.
echo Prerequisites:
echo   [x] Docker Desktop for Windows (must be running)
echo.
pause

REM Check if Docker is running
docker info >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo.
    echo [ERROR] Docker is not running!
    echo.
    echo Please start Docker Desktop and try again.
    echo.
    pause
    exit /b 1
)

cls
echo ============================================================
echo STEP 1/3: Preparing Configuration
echo ============================================================
echo.

REM Ensure config.docker.toml exists
if not exist "config.docker.toml" (
    echo [WARNING] config.docker.toml not found!
    echo This file should have been created. Please check your installation.
    pause
    exit /b 1
) else (
    echo [OK] config.docker.toml found
)

REM Create uploads directory if not exists
if not exist "uploads" (
    echo [INFO] Creating uploads directory...
    mkdir uploads
    echo [OK] Created uploads directory
) else (
    echo [OK] uploads directory exists
)

echo.
pause

cls
echo ============================================================
echo STEP 2/3: Building Docker Images
echo ============================================================
echo.
echo This will build Whatomate from source with the latest code
echo including the embedded signup feature.
echo.
echo [INFO] Building backend and frontend images...
echo This may take a few minutes on first run...
echo.

docker-compose -f docker-compose.dev.yml build

if %ERRORLEVEL% NEQ 0 (
    echo.
    echo [ERROR] Docker build failed!
    echo.
    echo Check the error messages above.
    pause
    exit /b 1
)

echo.
echo [OK] Docker images built successfully!
echo.
pause

cls
echo ============================================================
echo STEP 3/3: Starting All Services
echo ============================================================
echo.
echo Starting:
echo   - PostgreSQL (port 5432)
echo   - Redis (port 6379)
echo   - Whatomate Backend (port 8080)
echo   - Whatomate Frontend (port 5173)
echo.

docker-compose -f docker-compose.dev.yml up -d

if %ERRORLEVEL% NEQ 0 (
    echo.
    echo [ERROR] Failed to start services!
    echo.
    pause
    exit /b 1
)

echo.
echo [OK] All services started!
echo.
echo [INFO] Waiting 10 seconds for services to initialize...
timeout /t 10 /nobreak >nul

cls
echo ============================================================
echo             SETUP COMPLETE!
echo ============================================================
echo.
echo Whatomate is now running in Docker containers!
echo.
echo Access URLs:
echo   Frontend:  http://localhost:5173
echo   Backend:   http://localhost:8080
echo.
echo Next Steps:
echo   1. Open http://localhost:5173 in your browser
echo   2. Login with default credentials:
echo      Email: admin@admin.com
echo      Password: admin
echo   3. Go to Settings -^> Embedded Signup
echo   4. Create your first embedded signup configuration
echo.
echo ============================================================
echo.
echo Useful Commands:
echo.
echo   View logs:
echo     docker-compose -f docker-compose.dev.yml logs -f
echo.
echo   Stop services:
echo     docker-compose -f docker-compose.dev.yml stop
echo.
echo   Restart services:
echo     docker-compose -f docker-compose.dev.yml restart
echo.
echo   Stop and remove everything:
echo     docker-compose -f docker-compose.dev.yml down
echo.
echo   Check service status:
echo     docker-compose -f docker-compose.dev.yml ps
echo.
echo ============================================================
echo.
echo Press any key to open Whatomate in your browser...
pause >nul

start http://localhost:5173

echo.
echo [INFO] Whatomate is running in the background.
echo [INFO] Use the commands above to manage the services.
echo.
pause
