@echo off
color 0A
title Whatomate - Embedded Signup Setup Wizard
cls

echo ============================================================
echo         WHATOMATE - EMBEDDED SIGNUP SETUP WIZARD
echo ============================================================
echo.
echo This wizard will help you set up Whatomate with the new
echo Embedded Signup feature on Windows.
echo.
echo Prerequisites:
echo   [x] Go 1.24+
echo   [x] Node.js 18+
echo   [x] PostgreSQL running
echo   [x] Redis running (optional)
echo.
pause

cls
echo ============================================================
echo STEP 1/4: Configuration
echo ============================================================
echo.

if not exist "config.toml" (
    echo Creating config.toml from example...
    copy config.example.toml config.toml
    echo [OK] Created config.toml
    echo.
    echo [ACTION REQUIRED]
    echo Please edit config.toml with your database credentials:
    echo   - Open config.toml in a text editor
    echo   - Update [database] section with your PostgreSQL settings
    echo   - Save and close
    echo.
    echo Press any key when ready to continue...
    pause >nul
) else (
    echo [OK] config.toml already exists
    echo.
)

cls
echo ============================================================
echo STEP 2/4: Database Migration
echo ============================================================
echo.
echo Running database migrations to create tables...
echo This creates the embedded_signups and embedded_signup_leads tables.
echo.

go run cmd/whatomate/main.go server -config config.toml -migrate

if %ERRORLEVEL% EQU 0 (
    echo.
    echo [OK] Database migrations completed successfully!
    echo.
) else (
    echo.
    echo [ERROR] Migration failed!
    echo.
    echo Possible issues:
    echo   - PostgreSQL is not running
    echo   - Database credentials in config.toml are incorrect
    echo   - Database "whatomate" does not exist
    echo.
    echo Fix the issue and run this script again.
    pause
    exit /b 1
)

pause

cls
echo ============================================================
echo STEP 3/4: Starting Backend Server
echo ============================================================
echo.
echo The backend will start on http://localhost:8080
echo.
echo [INFO] Opening new window for backend...
start "Whatomate Backend" cmd /k "start-backend.bat"
timeout /t 3 /nobreak >nul
echo [OK] Backend server started in new window
echo.

cls
echo ============================================================
echo STEP 4/4: Starting Frontend Dev Server
echo ============================================================
echo.
echo The frontend will start on http://localhost:5173
echo.
echo [INFO] Opening new window for frontend...
start "Whatomate Frontend" cmd /k "start-frontend.bat"
timeout /t 3 /nobreak >nul
echo [OK] Frontend dev server started in new window
echo.

cls
echo ============================================================
echo             SETUP COMPLETE!
echo ============================================================
echo.
echo Your Whatomate instance is now running with Embedded Signup!
echo.
echo Access URLs:
echo   Frontend:  http://localhost:5173
echo   Backend:   http://localhost:8080
echo.
echo Next Steps:
echo   1. Open http://localhost:5173 in your browser
echo   2. Login with your credentials (default: admin@admin.com / admin)
echo   3. Go to Settings -^> Embedded Signup
echo   4. Click "+ Create Signup" to configure your first signup
echo.
echo Two new windows are open:
echo   - Whatomate Backend (port 8080)
echo   - Whatomate Frontend (port 5173)
echo.
echo Keep both windows open while using Whatomate.
echo Press Ctrl+C in each window to stop the servers.
echo.
echo ============================================================
echo.
echo Press any key to open Whatomate in your default browser...
pause >nul

start http://localhost:5173/settings/embedded-signup

echo.
echo This window can now be closed.
echo.
pause
