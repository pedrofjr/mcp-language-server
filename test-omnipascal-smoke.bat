@echo off
setlocal

set "OMNIPASCAL_SERVER=C:\Users\pedro.ailton\Downloads\wosi.omnipascal-0.19.0\extension\bin\win\OmniPascalServer.exe"
set "OMNIPASCAL_WORKSPACE=C:\Users\pedro.ailton\Downloads\mcp-language-server\SelecaoTimCFOPLote"
set "OMNIPASCAL_PROJECT_FILE=%OMNIPASCAL_WORKSPACE%\Project1.dpr"
set "OMNIPASCAL_TARGET_FILE=%OMNIPASCAL_WORKSPACE%\Unit1.pas"
set "OMNIPASCAL_DELPHI_PATH=C:\Programas\Borland\Delphi6"
set "OMNIPASCAL_SEARCH_PATH=C:\GIT\*"
set "OMNIPASCAL_MCP_BINARY=%~dp0mcp-language-server.exe"

go test ./integrationtests/tests/omnipascal -run "TestOmniPascalSmokeTools|TestOmniPascalMCPSurfaceTools" -v -count=1
exit /b %ERRORLEVEL%
