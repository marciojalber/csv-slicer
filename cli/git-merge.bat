@echo off
setlocal enabledelayedexpansion
chcp 65001 >nul

echo.
set user_text=Regular update
if not "%~1"=="" set user_text=%~1
:: echo %date% %time% - %user_text%> last-delivery.txt

git add .
git commit -m "%user_text%"
git push origin main

echo.
echo -----
time /t
echo Merge request finished
echo -----
echo.
