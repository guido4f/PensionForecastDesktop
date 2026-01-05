@echo off
REM Pension Forecast Simulator - Windows Run Script
REM This script runs the application

cd /d "%~dp0"

echo Pension Forecast Simulator
echo ==========================
echo.

set WEB_BIN=goPensionForecast-windows-amd64-web.exe
set CONSOLE_BIN=goPensionForecast-windows-amd64-console.exe
set UI_BIN=goPensionForecast-windows-amd64-ui.exe

echo Available modes:
echo.
if exist "%WEB_BIN%" echo   1) Web Server (recommended) - Opens in browser
if exist "%UI_BIN%" echo   2) Desktop UI - Native Windows window
if exist "%CONSOLE_BIN%" echo   3) Console - Command line interface
echo   Q) Quit
echo.
set /p choice="Select mode [1]: "

if "%choice%"=="" set choice=1

if "%choice%"=="1" (
    if exist "%WEB_BIN%" (
        echo Starting web server...
        start "" "%WEB_BIN%" -web
    ) else (
        echo Web binary not found
        pause
    )
    goto :eof
)

if "%choice%"=="2" (
    if exist "%UI_BIN%" (
        echo Starting desktop UI...
        start "" "%UI_BIN%" -ui
    ) else (
        echo UI binary not found
        pause
    )
    goto :eof
)

if "%choice%"=="3" (
    if exist "%CONSOLE_BIN%" (
        echo Starting console mode...
        "%CONSOLE_BIN%" -console
    ) else (
        echo Console binary not found
        pause
    )
    goto :eof
)

if /i "%choice%"=="q" (
    echo Exiting.
    goto :eof
)

echo Invalid choice
pause
