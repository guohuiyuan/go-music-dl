#!/bin/bash

echo "========================================"
echo "Go Music DL - 启动脚本 (Linux/macOS)"
echo "========================================"

echo "设置 Go 环境..."
go version
go env -w GOPROXY=https://goproxy.cn,direct
go env -w GO111MODULE=on

echo "整理依赖..."
go mod tidy

echo "生成 Windows 资源文件..."
go generate ./...

echo "构建项目..."
go build -ldflags="-s -w" -o music-dl ./cmd/music-dl

echo "运行程序..."
echo ""
echo "使用方法:"
echo "  1. 直接运行: ./music-dl"
echo "  2. 搜索歌曲: ./music-dl -k \"周杰伦\""
echo "  3. 启动Web服务: ./music-dl web"
echo ""

./music-dl web -p 8080