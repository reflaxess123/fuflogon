@echo off
setlocal EnableExtensions EnableDelayedExpansion

REM ============================================================
REM  pack.bat — упаковывает xray-bundle для переноса на другой комп
REM  Кладёт zip рядом с этим батником: xray-bundle-YYYYMMDD.zip
REM ============================================================

set "DIR=%~dp0"
if "!DIR:~-1!"=="\" set "DIR=!DIR:~0,-1!"

REM --- дата для имени архива ---
for /f "tokens=2 delims==" %%I in ('wmic os get localdatetime /value') do set "DT=%%I"
set "STAMP=!DT:~0,8!"
set "OUT=%DIR%\xray-bundle-!STAMP!.zip"

echo.
echo Упаковываем xray-bundle...
echo Источник: %DIR%
echo Архив:    %OUT%
echo.

REM --- удаляем старый архив если есть ---
if exist "%OUT%" del /F /Q "%OUT%"

REM --- список нужных файлов ---
set FILES=
set FILES=!FILES! "%DIR%\xray.exe"
set FILES=!FILES! "%DIR%\xray-launcher.exe"
set FILES=!FILES! "%DIR%\wintun.dll"
set FILES=!FILES! "%DIR%\geoip.dat"
set FILES=!FILES! "%DIR%\geosite.dat"
set FILES=!FILES! "%DIR%\config.json"

REM --- проверяем что все файлы на месте ---
for %%F in (xray.exe xray-launcher.exe wintun.dll geoip.dat geosite.dat config.json) do (
    if not exist "%DIR%\%%F" (
        echo [ERROR] Не найден: %DIR%\%%F
        pause
        exit /b 1
    )
)

REM --- пакуем через PowerShell Compress-Archive ---
powershell -NoProfile -Command ^
  "Compress-Archive -Path @('%DIR%\xray.exe','%DIR%\xray-launcher.exe','%DIR%\wintun.dll','%DIR%\geoip.dat','%DIR%\geosite.dat','%DIR%\config.json') -DestinationPath '%OUT%' -Force"

if errorlevel 1 (
    echo [ERROR] Не удалось создать архив
    pause
    exit /b 1
)

REM --- размер архива ---
for %%F in ("%OUT%") do set "SIZE=%%~zF"
set /a "SIZE_MB=!SIZE! / 1048576"

echo.
echo ============================================================
echo  ГОТОВО
echo  Файл:   %OUT%
echo  Размер: ~!SIZE_MB! MB
echo.
echo  Содержимое:
echo    xray.exe            - движок
echo    xray-launcher.exe   - лаунчер (двойной клик = трей)
echo    wintun.dll          - TUN-драйвер
echo    geoip.dat           - geo-базы
echo    geosite.dat         - geo-базы
echo    config.json         - конфиг
echo.
echo  Инструкция для кореша:
echo    1. Распаковать всё в одну папку
echo    2. Запустить xray-launcher.exe от администратора
echo    3. Логи в папке Logs\
echo ============================================================
echo.
pause
