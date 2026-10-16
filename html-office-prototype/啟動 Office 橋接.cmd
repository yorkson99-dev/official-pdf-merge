@echo off
chcp 65001 >nul
title Official PDF Merge - Microsoft Office Bridge
cd /d "%~dp0"
echo ==================================================
echo  Office COM local conversion bridge (HTML prototype)
echo  No network server / no custom EXE
echo  Keep this window open while converting documents.
echo  Close it or press Ctrl+C to stop.
echo ==================================================
echo.
if not exist "OfficeBridgeData\jobs" mkdir "OfficeBridgeData\jobs"
%SystemRoot%\System32\cscript.exe //nologo "OfficeBridge.vbs"
echo.
echo Office bridge stopped.
pause
