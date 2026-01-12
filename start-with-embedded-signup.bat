@echo off
echo ========================================
echo Starting Whatomate with Embedded Signup
echo ========================================
echo.

REM Check if config exists
if not exist "config.toml" (
    echo [WARNING] config.toml not found. Copying from example...
    copy config.example.toml config.toml
    echo [OK] Created config.toml
    echo [WARNING] Please edit config.toml with your database credentials
    echo.
)

REM Run migrations
echo [Step 1] Running database migrations...
go run cmd/whatomate/main.go server -config config.toml -migrate

if %ERRORLEVEL% EQU 0 (
    echo [OK] Migrations completed successfully
) else (
    echo [ERROR] Migration failed. Please check your database connection.
    pause
    exit /b 1
)

echo.
echo [OK] Setup complete!
echo.
echo Next steps:
echo   1. Start backend:  go run cmd/whatomate/main.go -config config.toml
echo   2. Start frontend: cd frontend ^&^& npm run dev
echo   3. Open: http://localhost:5173
echo   4. Navigate to: Settings -^> Embedded Signup
echo.
pause
