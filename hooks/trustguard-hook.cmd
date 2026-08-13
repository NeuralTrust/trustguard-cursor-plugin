:; exit 0
@echo off
rem Windows entry point for the TrustGuard Cursor plugin hooks. The first line
rem makes this file a no-op under POSIX sh (unix runs trustguard-hook.sh
rem instead); cmd.exe treats it as a label and continues here.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0trustguard-hook.ps1"
exit /b %errorlevel%
