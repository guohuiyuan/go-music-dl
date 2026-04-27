@echo off
setlocal EnableExtensions

if "%ANDROID_HOME%"=="" set "ANDROID_HOME=C:\Android"
set "ANDROID_SDK_ROOT=%ANDROID_HOME%"

set "SDKMANAGER=%ANDROID_HOME%\cmdline-tools\latest\bin\sdkmanager.bat"
set "ADB_EXE=%ANDROID_HOME%\cmdline-tools\latest\bin\adb.exe"
if not exist "%ADB_EXE%" set "ADB_EXE=%ANDROID_HOME%\platform-tools\adb.exe"

set "PATH=%ANDROID_HOME%\platform-tools;%ANDROID_HOME%\cmdline-tools\latest\bin;%PATH%"

if "%ANDROID_NDK_ROOT%"=="" (
	if exist "%ANDROID_HOME%\ndk-bundle\source.properties" (
		set "ANDROID_NDK_ROOT=%ANDROID_HOME%\ndk-bundle"
	) else (
		for /f "delims=" %%d in ('dir /b /ad /o-n "%ANDROID_HOME%\ndk\*" 2^>nul') do (
			if not defined ANDROID_NDK_ROOT set "ANDROID_NDK_ROOT=%ANDROID_HOME%\ndk\%%d"
		)
	)
)

if not defined ANDROID_NDK_ROOT (
	echo NDK not found, trying to install via sdkmanager...
	if not exist "%SDKMANAGER%" (
		echo ERROR: sdkmanager not found at "%SDKMANAGER%"
		exit /b 1
	)
	call "%SDKMANAGER%" "ndk;27.0.12077973"
	if errorlevel 1 (
		echo ERROR: failed to install NDK with sdkmanager.
		exit /b 1
	)
	for /f "delims=" %%d in ('dir /b /ad /o-n "%ANDROID_HOME%\ndk\*" 2^>nul') do (
		if not defined ANDROID_NDK_ROOT set "ANDROID_NDK_ROOT=%ANDROID_HOME%\ndk\%%d"
	)
)

if not defined ANDROID_NDK_ROOT (
	echo ERROR: ANDROID_NDK_ROOT is still empty after install attempt.
	exit /b 1
)

set "JAVA_EXE=java"
if defined JAVA_HOME if exist "%JAVA_HOME%\bin\java.exe" set "JAVA_EXE=%JAVA_HOME%\bin\java.exe"
set "JAVA_VERSION_LINE="
set "JAVA_VER_FILE=%TEMP%\gogio_java_version.txt"
"%JAVA_EXE%" -version > "%JAVA_VER_FILE%" 2>&1
for /f "usebackq delims=" %%l in ("%JAVA_VER_FILE%") do if not defined JAVA_VERSION_LINE set "JAVA_VERSION_LINE=%%l"
if exist "%JAVA_VER_FILE%" del /q "%JAVA_VER_FILE%" >nul 2>&1
if not defined JAVA_VERSION_LINE (
	echo ERROR: failed to read java version output using "%JAVA_EXE%".
	exit /b 1
)
set "JAVA_VERSION="
for /f "tokens=3" %%v in ("%JAVA_VERSION_LINE%") do set "JAVA_VERSION=%%~v"
if not defined JAVA_VERSION (
	echo ERROR: failed to detect java version using "%JAVA_EXE%".
	exit /b 1
)

echo Using Java version %JAVA_VERSION%

echo JAVA_HOME = %JAVA_HOME%
echo ANDROID_HOME = %ANDROID_HOME%
echo ANDROID_NDK_ROOT = %ANDROID_NDK_ROOT%
echo ADB_EXE = %ADB_EXE%
echo Download gogio

go install github.com/lianhong2758/gio-cmd/gogio@latest
if errorlevel 1 (
	echo ERROR: go install gogio failed.
	exit /b 1
)

cd desktop_app
if errorlevel 1 (
	echo ERROR: cannot enter desktop_app directory.
	exit /b 1
)

echo Building!
gogio -target android ^
 -buildmode exe ^
 -o ../music-dl.apk ^
 -appid com.musicdl.app.util ^
 -name MusicDL ^
 -version 1.0.0.1 ^
 -icon ../winres/icon_256x256.png ^
 -slice ^
 github.com/guohuiyuan/go-music-dl/desktop_app
if errorlevel 1 (
	echo ERROR: gogio build failed.
	exit /b 1
)

if exist "%ADB_EXE%" (
	echo Install to device: "%ADB_EXE%" install -r ..\music-dl.apk
) else (
	echo APK built: ..\music-dl.apk
	echo adb not found, please install platform-tools and run adb install manually.
)