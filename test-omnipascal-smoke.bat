@echo off
setlocal

set "OMNIPASCAL_SERVER=C:\Users\pedro.ailton\Downloads\wosi.omnipascal-0.19.0\extension\bin\win\OmniPascalServer.exe"
set "OMNIPASCAL_WORKSPACE=C:\Users\pedro.ailton\Downloads\mcp-language-server\SelecaoTimCFOPLote"
set "OMNIPASCAL_PROJECT_FILE=%OMNIPASCAL_WORKSPACE%\Project1.dpr"
set "OMNIPASCAL_TARGET_FILE=%OMNIPASCAL_WORKSPACE%\Unit1.pas"
set "OMNIPASCAL_DELPHI_PATH=C:\Programas\Borland\Delphi6"
set "OMNIPASCAL_SEARCH_PATH=C:\GIT\*"
set "OMNIPASCAL_MCP_BINARY=%~dp0mcp-language-server.exe"
set "OMNIPASCAL_TEST_BINARY=%~dp0omnipascal.test.exe"
set "OMNIPASCAL_ALLOW_GO_RUN_FALLBACK=0"

if not exist "%OMNIPASCAL_MCP_BINARY%" (
	echo Building MCP binary: %OMNIPASCAL_MCP_BINARY%
	go build -o "%OMNIPASCAL_MCP_BINARY%" .
	if errorlevel 1 exit /b %ERRORLEVEL%
)

if not exist "%OMNIPASCAL_TEST_BINARY%" (
	echo Building test binary: %OMNIPASCAL_TEST_BINARY%
	go test -c ./integrationtests/tests/omnipascal -o "%OMNIPASCAL_TEST_BINARY%"
	if errorlevel 1 exit /b %ERRORLEVEL%
)

"%OMNIPASCAL_TEST_BINARY%" -test.v -test.run "TestOmniPascalSmokeTools|TestOmniPascalMCPSurfaceTools"
exit /b %ERRORLEVEL%
