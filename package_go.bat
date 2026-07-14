go env -w GOPROXY=https://goproxy.cn,direct
echo Download go-winres
go install github.com/tc-hib/go-winres@latest

echo Generate Windows resource manifests
go generate ./...

go-winres make --in winres\desktop_go.winres.json
move rsrc_windows_386.syso desktop_go\
move rsrc_windows_amd64.syso desktop_go\
cd desktop_go

echo Build Go desktop app
go mod tidy
go build -ldflags="-H windowsgui -w -s" -o ..\music-dl-desktop-go.exe

del *.syso
cd ..
echo Build complete!
echo You can now run music-dl-desktop-go.exe
pause
