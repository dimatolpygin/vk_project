@echo off
setlocal

cd /d "%~dp0"

if not exist ".env" (
  copy ".env.example" ".env" >nul
)

if not exist "node_modules\playwright-core\package.json" (
  echo Installing kwork-assistant dependencies...
  call npm.cmd install --ignore-scripts
  if errorlevel 1 exit /b 1
)

set KWORK_AUTO_SEND=true
set KWORK_HEADLESS=false

echo Starting Kwork assistant autopilot...
call npm.cmd start
