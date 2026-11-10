@echo off
setlocal
set "SCRIPT=%~dp0Scripts\Setup.ps1"

where pwsh.exe >nul 2>nul
if not errorlevel 1 (
    pwsh.exe -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT%" %*
    set "RESULT=%errorlevel%"
    goto finished
)

where powershell.exe >nul 2>nul
if not errorlevel 1 (
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT%" %*
    set "RESULT=%errorlevel%"
    goto finished
)

echo PowerShell was not found. Install PowerShell 7 or Windows PowerShell.
set "RESULT=1"

:finished
echo.
pause
exit /b %RESULT%
