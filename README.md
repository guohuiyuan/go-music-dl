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

## 快速开始

### Web 模式

```bash
./music-dl web
```

浏览器会自动打开 `http://localhost:8080`。

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

## 支持平台

| 平台 | 模块名 | 搜索 | 下载 | 歌词 | 歌曲链接解析 | 歌单搜索 | 歌单歌曲 | 歌单链接解析 | 备注 |
| :--- | :--- | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :--- |
| 网易云音乐 | `netease` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | |
| QQ 音乐 | `qq` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | |
| 酷狗音乐 | `kugou` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | |
| 酷我音乐 | `kuwo` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | |
| 咪咕音乐 | `migu` | ✅ | ✅ | ✅ | ❌ | ✅ | ❌ | ❌ | |
| 千千音乐 | `qianqian` | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ | ❌ | |
| 汽水音乐 | `soda` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 支持音频解密 |
| 5sing | `fivesing` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | |
| Jamendo | `jamendo` | ✅ | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | |
| JOOX | `joox` | ✅ | ✅ | ✅ | ❌ | ✅ | ❌ | ❌ | |
| Bilibili | `bilibili` | ✅ | ✅ | ❌ | ✅ | ✅ | ✅ | ✅ | |

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
│   └── music-dl/
├── core/
├── internal/
│   ├── cli/
│   └── web/
├── downloads/
├── screenshots/
└── README.md
```

## 技术栈

- **核心库**: [music-lib](https://github.com/guohuiyuan/music-lib) - 音乐平台搜索下载
- **CLI 框架**: [Cobra](https://github.com/spf13/cobra) - 命令行工具
- **Web 框架**: [Gin](https://github.com/gin-gonic/gin) - Web 框架
- **TUI 框架**: [Bubble Tea](https://github.com/charmbracelet/bubbletea) - 终端界面
- **下载库**: [music-dl](https://github.com/0xHJK/music-dl) - 音乐下载库
- **下载库**: [musicdl](https://github.com/CharlesPikachu/musicdl) - 音乐下载库

## 贡献

欢迎提交 Issue 或 Pull Request。

## 许可证

本项目基于 [CharlesPikachu/musicdl](https://github.com/CharlesPikachu/musicdl) 的设计思路开发，遵循 [PolyForm Noncommercial License 1.0.0](https://polyformproject.org/licenses/noncommercial/1.0.0)。禁止商业使用。

## 免责声明

仅供学习和技术交流使用。下载的音乐资源请在 24 小时内删除。