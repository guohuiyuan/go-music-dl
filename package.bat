@echo off
echo Building Go binary...
go build -o music-dl.exe ./cmd/music-dl

echo Building Rust desktop app...
cd desktop
cargo build --release
copy target\release\music-dl-desktop-rust.exe ..\music-dl-desktop-rust.exe
cd ..

echo Build complete!
echo You can now run music-dl-desktop-rust.exe
pause
