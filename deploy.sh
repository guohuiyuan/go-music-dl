#!/bin/bash
# ---------------------------------------------------------
# 修复 Windows Git Bash 下路径自动转换导致的问题
export MSYS_NO_PATHCONV=1
# ---------------------------------------------------------

# Go-Music-DL 远程镜像部署脚本

set -e

# ================= 配置项 =================
# 镜像名称 (请确保已推送到 Docker Hub)
IMAGE_NAME="guohuiyuan/go-music-dl:latest"
# 部署目录
WORK_DIR="music-dl"
# =========================================

echo "🎵 开始部署 Go-Music-DL (远程镜像版)..."

# 1. 检查 Docker 环境
if ! command -v docker &> /dev/null; then
    echo "❌ Docker 未安装"
    exit 1
fi

if docker compose version &> /dev/null; then
    DOCKER_COMPOSE_CMD="docker compose"
elif command -v docker-compose &> /dev/null; then
    DOCKER_COMPOSE_CMD="docker-compose"
else
    echo "❌ 未找到 Docker Compose"
    exit 1
fi

# 2. 准备工作目录
if [ ! -d "$WORK_DIR" ]; then
    echo "📂 创建部署目录: $WORK_DIR"
    mkdir -p "$WORK_DIR"
fi

# !!! 进入目录 !!!
cd "$WORK_DIR"
echo "📂 已进入目录: $(pwd)"

# 3. 清理旧进程
echo "🧹 清理旧服务..."
$DOCKER_COMPOSE_CMD down 2>/dev/null || true

# 强力清理可能存在的同名容器
if docker ps -a --format '{{.Names}}' | grep -q "^music-dl$"; then
    echo "   ⚠️ 发现旧容器实例，正在强制删除..."
    docker rm -f music-dl
fi

# 4. 创建挂载目录与权限 (关键)
# 必须给 downloads 目录 777 权限，因为容器内是 appuser (uid:1000)
if [ ! -d "downloads" ]; then
    echo "📁 创建下载目录 downloads/ ..."
    mkdir -p downloads
fi

echo "🔐 修正目录权限 (chmod 777 downloads) ..."
chmod -R 777 downloads

# 5. 生成 docker-compose.yml
# 注意：这里不再包含 build 字段，而是直接指定 image
echo "📝 生成 docker-compose.yml..."
cat > docker-compose.yml <<EOF
services:
  music-dl:
    image: ${IMAGE_NAME}
    container_name: music-dl
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./downloads:/home/appuser/downloads
    environment:
      - TZ=Asia/Shanghai
    user: "1000:1000"
EOF

# 6. 拉取并启动
echo "☁️  正在拉取最新镜像: $IMAGE_NAME ..."
$DOCKER_COMPOSE_CMD pull

echo "🚀 启动服务..."
$DOCKER_COMPOSE_CMD up -d

# 7. 检查状态
echo "⏳ 等待初始化 (3秒)..."
sleep 3

if docker ps | grep -q "music-dl"; then
    echo ""
    echo "✅ 部署成功！"
    echo "------------------------------------------------"
    echo "🎵 Web 访问: http://localhost:8080"
    echo "📂 本地目录: $(pwd)/downloads"
    echo ""
    echo "👇 常用命令 (请先 cd $WORK_DIR):"
    echo "   查看日志: $DOCKER_COMPOSE_CMD logs -f"
    echo "   更新镜像: $DOCKER_COMPOSE_CMD pull && $DOCKER_COMPOSE_CMD up -d"
    echo "------------------------------------------------"
else
    echo ""
    echo "❌ 启动失败！"
    echo "请检查日志: cd $WORK_DIR && $DOCKER_COMPOSE_CMD logs"
    exit 1
fi