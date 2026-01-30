# 🎵 Go Music DL

Go Music DL 是一个音乐搜索下载工具，支持命令行和网页两种使用方式。它能搜索并下载来自十多个主流音乐平台的歌曲。

![Web UI](./screenshots/web.png)
_Web 界面_

![TUI](./screenshots/tui.png)
_TUI 终端界面_

## 功能

- **双模式操作**:
  - **Web 模式**: 启动本地网页服务，在浏览器中搜索、试听、下载，支持歌词滚动
  - **CLI/TUI 模式**: 在命令行中搜索，或使用交互式 TUI 界面批量下载

- **聚合搜索**: 支持网易云、QQ音乐、酷狗等十多个平台

- **下载功能**:
  - 自动命名: `歌手 - 歌名.mp3`
  - 支持下载封面和歌词文件
  - 清理文件名中的非法字符

- **在线试听**: Web 模式下内置播放器，支持歌词滚动

- **特殊格式支持**: 支持汽水音乐等平台的加密音频解密

- **免费优先**: 自动跳过需要 VIP 或付费的歌曲

- **流式转发**: 支持 Range 请求，实现快速播放

## 快速开始

### 1. 下载程序

从 [GitHub Releases](https://github.com/guohuiyuan/go-music-dl/releases) 下载适用于你操作系统的最新版本。

### 2. 开始使用

#### Web 模式（推荐新手）

```bash
./music-dl web
```

程序会自动打开浏览器，访问 `http://localhost:8080`。

#### CLI/TUI 模式

```bash
# 搜索 "周杰伦" 的歌曲
./music-dl -k "周杰伦"
```

进入 TUI 界面后，使用 `↑` `↓` 键选择歌曲，按 `空格` 选中，最后按 `回车` 开始下载。

**其他用法**:
```bash
# 查看帮助
./music-dl -h

# 搜索 "周杰伦 晴天"，指定从 QQ 音乐和网易云搜索
./music-dl -k "周杰伦 晴天" -s qq,netease

# 指定下载目录
./music-dl -k "周杰伦" -o ./my_music

# 下载时默认包含封面和歌词
./music-dl -k "周杰伦" --cover --lyrics
```

## 支持的音乐平台

| 平台 | 模块名 | 搜索 | 下载 | 歌词 | 链接解析 | 备注 |
| :--- | :--- | :---: | :---: | :---: | :---: | :--- |
| 网易云音乐 | `netease` | ✅ | ✅ | ✅ | ✅ | |
| QQ 音乐 | `qq` | ✅ | ✅ | ✅ | ✅ | |
| 酷狗音乐 | `kugou` | ✅ | ✅ | ✅ | ✅ | |
| 酷我音乐 | `kuwo` | ✅ | ✅ | ✅ | ✅ | |
| 咪咕音乐 | `migu` | ✅ | ✅ | ✅ | ✅ | |
| 千千音乐 | `qianqian` | ✅ | ✅ | ✅ | ❌ | |
| 汽水音乐 | `soda` | ✅ | ✅ | ✅ | ✅ | 支持音频解密 |
| 5sing | `fivesing` | ✅ | ✅ | ✅ | ✅ | |
| Jamendo | `jamendo` | ✅ | ✅ | ❌ | ✅ | |
| JOOX | `joox` | ✅ | ✅ | ✅ | ❌ | |
| Bilibili | `bilibili` | ✅ | ✅ | ❌ | ✅ | |

## 链接解析

除了关键词搜索，还支持直接解析音乐分享链接：

```bash
# 直接粘贴分享链接
./music-dl -k "https://music.163.com/#/song?id=123456"
```

支持解析的平台：网易云、QQ音乐、酷狗、酷我、咪咕、Bilibili、汽水音乐、5sing、Jamendo。

## 常见问题

**Q: 为什么有些歌搜不到或下载失败？**
A: 可能原因：1) 歌曲需要 VIP 或付费；2) 音乐平台接口变更；3) 网络问题。

**Q: Web 模式启动后页面打不开？**
A: 检查：1) 默认端口 8080 是否被占用；2) 浏览器插件是否干扰页面脚本。

**Q: 如何设置 Cookie 获取更高音质？**
A: 在 Web 界面的设置中，可以添加各平台的 Cookie。

## 项目结构

```
go-music-dl/
├── cmd/
│   └── music-dl/       # CLI 命令定义
│       ├── main.go       # 程序入口
│       ├── root.go       # 主命令
│       └── web.go        # Web 子命令
├── core/                 # 核心业务逻辑
│   └── service.go       # 并发搜索、源管理
├── internal/
│   ├── cli/              # TUI 界面
│   └── web/              # Web 服务
├── downloads/            # 默认下载目录
├── screenshots/          # 截图
├── go.mod
└── README.md
```

## 技术栈

- **核心库**: [music-lib](https://github.com/guohuiyuan/music-lib) - 音乐平台搜索下载能力
- **CLI 框架**: [Cobra](https://github.com/spf13/cobra) - 命令行工具
- **Web 框架**: [Gin](https://github.com/gin-gonic/gin) - Web 框架
- **TUI 框架**: [Bubble Tea](https://github.com/charmbracelet/bubbletea) - 终端界面

## 贡献

欢迎提交 Issue 或 Pull Request。

## 许可证

基于 [GNU Affero General Public License v3.0](https://github.com/guohuiyuan/go-music-dl/blob/main/LICENSE) 许可。

## 免责声明

本项目仅供个人学习和技术交流使用。请在遵守相关法律法规的前提下合理使用。通过本工具下载的音乐资源，请于 24 小时内删除。