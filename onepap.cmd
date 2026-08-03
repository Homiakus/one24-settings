@echo off
where pwsh >nul 2>nul
if errorlevel 1 (
  echo PowerShell 7 ^(pwsh^) not found.
  echo Install PowerShell 7 or run onepap.ps1 from an existing PS7 terminal.
  exit /b 1
)
pwsh -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0onepap.ps1" %*
