@echo off
title Whatomate - View Docker Logs
echo ========================================
echo Whatomate Docker Logs
echo ========================================
echo.
echo Press Ctrl+C to exit log view
echo.
timeout /t 2 /nobreak >nul

docker-compose -f docker-compose.dev.yml logs -f --tail=100
