@echo off
setlocal EnableExtensions

rem Resolve the project root from this script so it works from any directory.
set "SCRIPT_DIR=%~dp0"
for %%I in ("%SCRIPT_DIR%..") do set "PROJECT_DIR=%%~fI"

pushd "%PROJECT_DIR%" || (
    echo Failed to move to project directory: "%PROJECT_DIR%"
    exit /b 1
)

echo Stopping running teamgg.exe processes...
taskkill /F /IM teamgg.exe >nul 2>&1
if errorlevel 1 (
    echo No running teamgg.exe process found.
) else (
    echo Running teamgg.exe processes stopped.
)

echo Building teamgg.exe...
go build -o teamgg.exe .\main.go
if errorlevel 1 (
    echo Build failed.
    popd
    exit /b 1
)
echo Successfully built to "%PROJECT_DIR%\teamgg.exe".

echo Starting teamgg.exe in the foreground...
"%PROJECT_DIR%\teamgg.exe"
set "TEAMGG_EXIT_CODE=%ERRORLEVEL%"

popd
endlocal & exit /b %TEAMGG_EXIT_CODE%
