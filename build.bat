@echo off
chcp 65001 >nul
setlocal
cd /d "%~dp0"

where go >nul 2>nul
if errorlevel 1 (
    echo Не найден Go. Установите его с https://go.dev/dl/ и запустите build.bat снова.
    pause
    exit /b 1
)

set "VERSION=dev"
for /f "delims=" %%v in ('git describe --tags --always --dirty 2^>nul') do set "VERSION=%%v"
set "LDFLAGS=-s -w -X github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/version.Version=%VERSION%"
set CGO_ENABLED=0

if not exist bin mkdir bin

echo Сборка lastfm-scrobbler.exe (команды) ...
go build -trimpath -ldflags "%LDFLAGS%" -o bin\lastfm-scrobbler.exe .\cmd\lastfm-scrobbler
if errorlevel 1 goto failed

echo Сборка lastfm-scrobblerw.exe (окно и значок в трее) ...
go build -trimpath -ldflags "%LDFLAGS% -H=windowsgui -X main.backgroundBuild=1" -o bin\lastfm-scrobblerw.exe .\cmd\lastfm-scrobbler
if errorlevel 1 goto failed

copy /y third_party\webview2\WebView2Loader.dll bin\ >nul
if errorlevel 1 goto failed
copy /y third_party\webview2\LICENSE.txt bin\WebView2Loader.LICENSE.txt >nul

echo.
echo Готово, версия %VERSION%. Файлы лежат в папке bin:
echo   bin\lastfm-scrobblerw.exe  - программа: окно и значок в трее
echo   bin\lastfm-scrobbler.exe   - команды для консоли: status, queue, run
echo   bin\WebView2Loader.dll     - загрузчик окна от Microsoft, держите рядом с exe
echo.
echo Дальше: запустите bin\lastfm-scrobblerw.exe и войдите в Last.fm в окне
pause
exit /b 0

:failed
echo.
echo Сборка не удалась, подробности выше.
pause
exit /b 1
