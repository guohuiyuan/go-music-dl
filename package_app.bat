echo JAVA_HOME = %JAVA_HOME%
echo ANDROID_HOME = %ANDROID_HOME%
echo Download gogio 
go install github.com/lianhong2758/gio-cmd/gogio@latest
cd desktop_app
echo Building!
gogio -target android ^
-buildmode exe ^
-o music-dl.apk ^
-appid com.musicdl.app.util ^
-name MusicDL ^
-version 1.0.0.1 ^
-icon ../winres/icon_256x256.png ^
github.com/guohuiyuan/go-music-dl/desktop_app 
echo you can load it onto your mobile phone by: adb install music-dl.apk      