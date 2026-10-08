@echo off
cd /d "%~dp0"

echo ==========================================
echo   CardServer ONE-CLICK DEPLOY
echo ==========================================
echo.

where dotnet >nul 2>nul
if errorlevel 1 (
    echo [ERROR] dotnet not found. Install .NET 8 SDK first.
    pause
    exit /b 1
)

REM 1. Build
echo [1/5] Building (win-x64 self-contained)...
dotnet publish -c Release -r win-x64 --self-contained true -p:PublishSingleFile=true -o publish
if errorlevel 1 (
    echo [ERROR] Build failed. See output above.
    pause
    exit /b 1
)

REM 2. Prepare deploy folder
set "DEPLOY=%~dp0deploy"
if exist "%DEPLOY%" rmdir /s /q "%DEPLOY%"
mkdir "%DEPLOY%"

echo [2/5] Copying build output...
REM Clean junk inside publish first (prevent nested deploy dir)
if exist "%~dp0publish\deploy" rmdir /s /q "%~dp0publish\deploy"
xcopy "%~dp0publish\*" "%DEPLOY%\" /e /y /q >nul
REM Safety: remove nested deploy if any
if exist "%DEPLOY%\deploy" rmdir /s /q "%DEPLOY%\deploy"

REM 3. Config files
echo [3/5] Copying appsettings.json + private.pem...
if exist "%~dp0appsettings.json" copy /y "%~dp0appsettings.json" "%DEPLOY%\" >nul
if not exist "%DEPLOY%\appsettings.json" (
    echo [ERROR] appsettings.json not found!
)
if exist "%~dp0private.pem" copy /y "%~dp0private.pem" "%DEPLOY%\" >nul
if not exist "%DEPLOY%\private.pem" (
    echo [WARN] private.pem not found - generate one or copy it in!
)

REM 4. Hotupdate folder
echo [4/5] Creating hotupdate folder...
mkdir "%DEPLOY%\hotupdate" 2>nul
if not exist "%DEPLOY%\hotupdate\version.txt" echo 1.0> "%DEPLOY%\hotupdate\version.txt"

REM 5. Start script
copy /y "%~dp0start_server.bat" "%DEPLOY%\" >nul

echo [5/5] Done!
echo.
echo ==========================================
echo   DEPLOY FOLDER READY:
echo   %DEPLOY%
echo ==========================================
echo.
echo   Files:
echo     CardServer.exe        - main server
echo     wwwroot\              - admin pages (NEEDED)
echo     e_sqlite3.dll         - SQLite native lib (NEEDED)
echo     appsettings.json      - EDIT password/port before run!
echo     private.pem           - RSA private key
echo     hotupdate\version.txt - hot update version (edit)
echo     start_server.bat      - start server
echo.
echo   NEXT STEPS:
echo     1. Copy whole "deploy" folder to server, e.g. C:\cardserver
echo     2. Edit appsettings.json: AdminPassword / Urls port
echo     3. Put Go build (app) into hotupdate\ and bump version.txt
echo     4. Run start_server.bat
echo.
pause
