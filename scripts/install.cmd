@echo off
setlocal enabledelayedexpansion

REM Heimdal CLI installer for Windows CMD
REM Usage:
REM   curl -fsSL https://raw.githubusercontent.com/coone-ai/heimdal/main/scripts/install.cmd -o install.cmd ^&^& install.cmd ^&^& del install.cmd
REM
REM Optional environment variables:
REM   HEIMDAL_REPO      GitHub repo in owner/name format (default: coone-ai/heimdal)
REM   HEIMDAL_VERSION   Release tag to install, e.g. v0.0.1 (default: latest)
REM   HEIMDAL_INSTALL   Install directory (default: %%LOCALAPPDATA%%\Programs\heimdal)

if not defined HEIMDAL_REPO set "HEIMDAL_REPO=coone-ai/heimdal"
if not defined HEIMDAL_INSTALL set "HEIMDAL_INSTALL=%LOCALAPPDATA%\Programs\heimdal"
set "VERSION=%HEIMDAL_VERSION%"

if not defined VERSION (
  echo Resolving latest release tag...
  for /f "usebackq delims=" %%V in (`powershell -NoProfile -Command "(Invoke-RestMethod 'https://api.github.com/repos/%HEIMDAL_REPO%/releases/latest').tag_name"`) do set "VERSION=%%V"
)

if not defined VERSION (
  echo Failed to resolve release tag from GitHub.
  exit /b 1
)

set "ASSET_VERSION=%VERSION%"
if /i "%ASSET_VERSION:~0,1%"=="v" set "ASSET_VERSION=%ASSET_VERSION:~1%"

set "ARCH_RAW=%PROCESSOR_ARCHITECTURE%"
if /i "%ARCH_RAW%"=="AMD64" (
  set "ARCH=amd64"
) else if /i "%ARCH_RAW%"=="ARM64" (
  set "ARCH=arm64"
) else (
  echo Unsupported architecture: %ARCH_RAW%
  exit /b 1
)

set "ARCHIVE=heimdal_%ASSET_VERSION%_windows_%ARCH%.zip"
set "URL=https://github.com/%HEIMDAL_REPO%/releases/download/%VERSION%/%ARCHIVE%"

set "TMPDIR=%TEMP%\heimdal-install-%RANDOM%%RANDOM%"
mkdir "%TMPDIR%" >nul 2>&1
if errorlevel 1 (
  echo Failed to create temp directory.
  exit /b 1
)

set "ZIP_PATH=%TMPDIR%\%ARCHIVE%"
set "EXTRACT_DIR=%TMPDIR%\extract"

echo Downloading %URL%
powershell -NoProfile -Command "Invoke-WebRequest -Uri '%URL%' -OutFile '%ZIP_PATH%'" || goto :fail
powershell -NoProfile -Command "Expand-Archive -Path '%ZIP_PATH%' -DestinationPath '%EXTRACT_DIR%' -Force" || goto :fail

if not exist "%EXTRACT_DIR%\heimdal.exe" (
  echo Binary not found in archive: %ARCHIVE%
  goto :fail
)

mkdir "%HEIMDAL_INSTALL%" >nul 2>&1
copy /Y "%EXTRACT_DIR%\heimdal.exe" "%HEIMDAL_INSTALL%\heimdal.exe" >nul || goto :fail
copy /Y "%EXTRACT_DIR%\heimdal.exe" "%HEIMDAL_INSTALL%\coval.exe" >nul || goto :fail

echo Installed: %HEIMDAL_INSTALL%\heimdal.exe
echo Alias: %HEIMDAL_INSTALL%\coval.exe

set "USER_PATH="
for /f "usebackq delims=" %%P in (`powershell -NoProfile -Command "[Environment]::GetEnvironmentVariable('Path','User')"`) do set "USER_PATH=%%P"
echo ;%USER_PATH%; | find /I ";%HEIMDAL_INSTALL%;" >nul
if errorlevel 1 (
  setx PATH "%USER_PATH%;%HEIMDAL_INSTALL%" >nul
  echo Added to user PATH: %HEIMDAL_INSTALL%
)

"%HEIMDAL_INSTALL%\heimdal.exe" version
echo.
echo Open a new terminal if 'heimdal' or 'coval' is not found in your current session.

rmdir /s /q "%TMPDIR%" >nul 2>&1
exit /b 0

:fail
echo Installation failed.
rmdir /s /q "%TMPDIR%" >nul 2>&1
exit /b 1
