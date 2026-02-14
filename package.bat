@echo off
echo Building Go binary...
go build -o music-dl.exe ./cmd/music-dl

echo Building Rust desktop app...
cd desktop
cargo build --release
copy target\release\go-music-dl-desktop.exe ..\music-dl-desktop.exe
cd ..

echo Build complete!
echo You can now run music-dl-desktop.exe
pause