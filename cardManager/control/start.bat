@echo off
cd /d "%~dp0"

echo ==========================================
echo   AutoGo Ctrl - BUILD AND RUN
echo ==========================================
echo.

where dotnet >nul 2>nul
if errorlevel 1 (
    echo [ERROR] dotnet not found. Install .NET 8 SDK first.
    echo         https://dotnet.microsoft.com/download/dotnet/8.0
    pause
    exit /b 1
)

echo [1/3] Cleaning old build...
if exist "bin" rmdir /s /q "bin"
if exist "obj" rmdir /s /q "obj"

echo.
echo [2/3] Building...
dotnet build -c Release
if errorlevel 1 (
    echo.
    echo [ERROR] Build failed. See output above.
    pause
    exit /b 1
)

echo.
echo [3/3] Starting AutoGo Ctrl...
dotnet run -c Release --no-build
if errorlevel 1 (
    echo.
    echo [ERROR] Run failed.
    pause
    exit /b 1
)
