# Go Music DL

Go Music DL 是一个音乐搜索与下载工具，带 Web 和 TUI 两种入口。你可以在浏览器试听，也可以在终端里批量下载。

![Web UI 1](./screenshots/web1.png)
![Web UI 2](./screenshots/web2.png)

![TUI 1](./screenshots/tui1.png)
![TUI 2](./screenshots/tui2.png)

## 主要功能

- Web 与 TUI 双模式
- 多平台聚合搜索与歌单搜索
- 试听、歌词、封面下载
- Range 探测：显示大小与码率
- 汽水音乐等加密音频解密
- 过滤需要付费的资源

## 新增改动（简要）

- Web 试听按钮支持播放/停止切换
- Web 单曲支持“换源”，按相似度优先、时长接近、可播放验证
- 换源自动排除 soda 与 fivesing
- TUI 增加 r 键批量换源，并显示换源进度
- 增加“每日歌单推荐”，Web 和 TUI 都能看
- Web 端支持批量操作：全选、选择无效、批量下载、批量换源

## 快速开始

### 桌面应用模式

桌面应用提供了原生窗口体验，无需打开浏览器即可使用。

#### 特性
- 🖥️ 原生桌面窗口，无需浏览器
- 🚀 自动启动内置Web服务器
- 🎵 完整Web界面功能
- 📦 单文件分发，绿色免安装
- 🖼️ 自定义窗口图标

#### 下载使用

1. 从 [Releases](https://github.com/guohuiyuan/go-music-dl/releases) 页面下载最新版本的 `go-music-dl-desktop-windows.zip`
2. 解压到任意目录
3. 双击运行 `go-music-dl-desktop.exe`
4. 应用会自动启动Web服务器并打开桌面窗口

#### 手动构建

如果需要自定义构建：

```bash
# 1. 构建Go二进制文件
cd go-music-dl
go build -o ../go-music-dl-desktop/music-dl cmd/music-dl/main.go

# 2. 构建Rust桌面应用
cd ../go-music-dl-desktop
cargo build --release

# 3. 打包
# Windows
./package.bat
```

#### 系统要求
- Windows 10/11 (推荐)
- 已安装 WebView2 运行时 (通常已预装)
- 如遇WebView2错误，可从 [Microsoft官网](https://developer.microsoft.com/microsoft-edge/webview2/) 下载安装

### Web 模式

```bash
./music-dl web
```

浏览器会自动打开 `http://localhost:8080/music`。

#### 反向代理配置

如果需要通过 Nginx 等反向代理访问，可以配置路由前缀：

```nginx
location /music/ {
    proxy_pass http://127.0.0.1:8080/music/;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
}
```

访问地址：`http://your-domain.com/music/`

**注意：** 应用程序已内置路由前缀支持，无需额外配置即可在子路径下正常工作。

### Docker 模式

```bash
# 构建镜像
docker build -t go-music-dl .

# 运行 Web 模式
docker run -p 8080:8080 -v $(pwd)/downloads:/home/appuser/downloads go-music-dl
```

浏览器会自动打开 `http://localhost:8080`。

**说明：**
- downloads目录会挂载到容器内，便于下载文件持久化
- 如需修改端口，使用 `-p 新端口:8080`

#### Docker Compose 模式

创建 `docker-compose.yml` 文件：

```yaml
version: '3.8'
services:
  go-music-dl:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./downloads:/home/appuser/downloads
    restart: unless-stopped
```

运行：

```bash
# 构建并启动
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止
docker-compose down
```

浏览器访问 `http://localhost:8080`。

**说明：**
- 自动构建镜像并管理容器
- 支持后台运行和自动重启
- 可轻松添加 Nginx 等反向代理服务

### 远程部署

使用部署脚本自动拉取最新镜像并启动服务：

```bash
# 下载部署脚本
wget https://raw.githubusercontent.com/guohuiyuan/go-music-dl/main/deploy.sh

# 运行部署
bash deploy.sh
```

脚本会自动：

- 检查Docker环境
- 创建部署目录
- 拉取最新Docker镜像
- 生成docker-compose.yml
- 启动服务

访问：http://localhost:8080

**说明：**
- 部署目录为 `music-dl/`
- 下载文件保存在 `music-dl/downloads/`
- Cookies文件为 `music-dl/cookies.json`

### CLI/TUI 模式

```bash
# 搜索
./music-dl -k "周杰伦"
```

TUI 常用按键：

- `↑/↓` 移动
- `空格` 选择
- `a` 全选/清空
- `r` 对勾选项换源
- `Enter` 下载
- `b` 返回
- `w` 每日推荐歌单
- `q` 退出

更多用法：

```bash
# 查看帮助
./music-dl -h

# 指定搜索源
./music-dl -k "周杰伦 晴天" -s qq,netease

# 指定下载目录
./music-dl -k "周杰伦" -o ./my_music

# 下载时包含封面和歌词
./music-dl -k "周杰伦" --cover --lyrics
```

## Web 换源说明

单曲卡片里的“换源”会在其它平台里找更像的版本：

- 先看歌名/歌手相似度
- 再看时长差异（太大就跳过）
- 最后做可播放探测

当前会跳过 soda 与 fivesing。

## 每日歌单推荐

Web 页面有“每日推荐”入口，会聚合网易云、QQ、酷狗、酷我。
TUI 在输入界面按 `w` 直接拉取推荐歌单，然后回车进详情。

## 支持平台

| 平台       | 包名         | 搜索 | 下载 | 歌词 | 歌曲解析 | 歌单搜索 | 歌单推荐 | 歌单歌曲 | 歌单链接解析 | 备注     |
| :--------- | :----------- | :--: | :--: | :--: | :------: | :------: | :------: | :------: | :----------: | :------- |
| 网易云音乐 | `netease`  |  ✅  |  ✅  |  ✅  |    ✅    |    ✅    |    ✅    |    ✅    |      ✅      |          |
| QQ 音乐    | `qq`       |  ✅  |  ✅  |  ✅  |    ✅    |    ✅    |    ✅    |    ✅    |      ✅      |          |
| 酷狗音乐   | `kugou`    |  ✅  |  ✅  |  ✅  |    ✅    |    ✅    |    ✅    |    ✅    |      ✅      |          |
| 酷我音乐   | `kuwo`     |  ✅  |  ✅  |  ✅  |    ✅    |    ✅    |    ✅    |    ✅    |      ✅      |          |
| 咪咕音乐   | `migu`     |  ✅  |  ✅  |  ✅  |    ❌    |    ✅    |    ❌    |    ❌    |      ❌      |          |
| 千千音乐   | `qianqian` |  ✅  |  ✅  |  ✅  |    ❌    |    ❌    |    ❌    |    ✅    |      ❌      |          |
| 汽水音乐   | `soda`     |  ✅  |  ✅  |  ✅  |    ✅    |    ✅    |    ❌    |    ✅    |      ✅      | 音频解密 |
| 5sing      | `fivesing` |  ✅  |  ✅  |  ✅  |    ✅    |    ✅    |    ❌    |    ✅    |      ✅      |          |
| Jamendo    | `jamendo`  |  ✅  |  ✅  |  ❌  |    ✅    |    ❌    |    ❌    |    ❌    |      ❌      |          |
| JOOX       | `joox`     |  ✅  |  ✅  |  ✅  |    ❌    |    ✅    |    ❌    |    ❌    |      ❌      |          |
| Bilibili   | `bilibili` |  ✅  |  ✅  |  ❌  |    ✅    |    ✅    |    ❌    |    ✅    |      ✅      |          |

## 歌曲链接解析

支持直接解析音乐分享链接：

```bash
./music-dl -k "https://music.163.com/#/song?id=123456"
```

支持解析的平台：网易云、QQ音乐、酷狗、酷我、咪咕、Bilibili、汽水音乐、5sing、Jamendo。

## 歌单链接解析

支持直接解析歌单/合集分享链接：

```bash
./music-dl -k "https://music.163.com/#/playlist?id=123456"
```

支持解析的平台：网易云、QQ音乐、酷狗、酷我、汽水音乐、5sing、Bilibili。

## 常见问题
**Q: 桌面应用打不开或显示空白？**
检查是否已安装 WebView2 运行时。从 [Microsoft官网](https://developer.microsoft.com/microsoft-edge/webview2/) 下载安装最新版本。

**Q: 桌面应用启动慢或卡顿？**
首次运行需要下载 WebView2 运行时。也可提前安装 Evergreen Bootstrapper 版本。
**Q: 有些歌搜不到或下载失败？**
可能是付费限制、平台接口变更或网络问题。

**Q: Web 模式打不开？**
检查端口是否占用，或浏览器插件是否拦截。

**Q: 如何设置 Cookie 获取更高音质？**
Web 右上角“设置”里可添加平台 Cookie。

## 项目结构

```
go-music-dl/
├── cmd/
│   └── music-dl/          # CLI/TUI 主程序
├── core/                  # 核心业务逻辑
├── internal/
│   ├── cli/              # TUI 界面
│   └── web/              # Web 服务器和模板
├── downloads/            # 下载文件目录
├── screenshots/          # 截图资源
├── go-music-dl-desktop/  # 桌面应用 (Rust + Tao/Wry)
│   ├── src/
│   ├── Cargo.toml
│   └── music-dl          # Go二进制文件
└── README.md
```

## 技术栈

- **核心库**: [music-lib](https://github.com/guohuiyuan/music-lib) - 音乐平台搜索下载
- **CLI 框架**: [Cobra](https://github.com/spf13/cobra) - 命令行工具
- **Web 框架**: [Gin](https://github.com/gin-gonic/gin) - Web 框架
- **TUI 框架**: [Bubble Tea](https://github.com/charmbracelet/bubbletea) - 终端界面
- **桌面框架**: [Tao](https://github.com/tauri-apps/tao) + [Wry](https://github.com/tauri-apps/wry) - 跨平台桌面应用
- **图像处理**: [image](https://github.com/image-rs/image) - 图标处理
- **下载库**: [music-dl](https://github.com/0xHJK/music-dl) - 音乐下载库
- **下载库**: [musicdl](https://github.com/CharlesPikachu/musicdl) - 音乐下载库

## 贡献

欢迎提交 Issue 或 Pull Request。

## 许可证

本项目遵循 GNU Affero General Public License v3.0（AGPL-3.0）。详情见 [LICENSE](LICENSE)。

## 免责声明

仅供学习和技术交流使用。下载的音乐资源请在 24 小时内删除。