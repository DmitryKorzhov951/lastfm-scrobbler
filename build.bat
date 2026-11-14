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
set "LDFLAGS=-s -w -H=windowsgui -X github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/version.Version=%VERSION%"
set CGO_ENABLED=0

if not exist bin mkdir bin

echo Сборка lastfm-scrobbler.exe ...
go build -trimpath -ldflags "%LDFLAGS%" -o bin\lastfm-scrobbler.exe .\cmd\lastfm-scrobbler
if errorlevel 1 goto failed

copy /y third_party\webview2\WebView2Loader.dll bin\ >nul
if errorlevel 1 goto failed
copy /y third_party\webview2\LICENSE.txt bin\WebView2Loader.LICENSE.txt >nul

echo.
echo Готово, версия %VERSION%. Файлы лежат в папке bin:
echo   bin\lastfm-scrobbler.exe   - программа: окно и значок в трее
echo   bin\WebView2Loader.dll     - загрузчик окна от Microsoft, держите рядом с exe
echo.
echo Дальше: запустите bin\lastfm-scrobbler.exe и войдите в Last.fm в окне
pause
exit /b 0

:failed
echo.
echo Сборка не удалась, подробности выше.
pause
exit /b 1
