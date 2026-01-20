# Go Music DL

一个完整的、工程化的 Go 项目，将 CLI（命令行）和 Web 服务合二为一。基于 **Cobra** (CLI 框架) + **Gin** (Web 框架) + **Bubble Tea** (TUI 交互)，核心逻辑封装在 `github.com/guohuiyuan/music-lib` 中。

## 特性

- **双模式运行**: 支持命令行交互模式和 Web 服务模式
- **多源搜索**: 支持网易云、QQ音乐、酷狗、酷我、咪咕、5sing、Jamendo、JOOX、千千音乐、Soda 等多个音乐源
- **现代化界面**: 
  - CLI: 使用 Bubble Tea 提供交互式表格界面
  - Web: 使用 Gin + Tailwind CSS 提供美观的网页界面
- **统一文件命名**: 下载文件自动命名为 `歌手 - 歌名.mp3` 格式
- **完整元数据**: 显示歌曲时长、大小、专辑、封面等详细信息
- **封面下载**: 支持同时下载歌曲封面图片
- **歌词支持**: 支持歌词显示和同步播放，使用 APlayer 播放器提供优美的歌词体验，专注于当前播放歌词的高亮显示
- **音乐播放器**: 内置 Web 音乐播放器，支持在线试听和歌词同步
- **Live2D 看板娘**: 集成 Live2D 虚拟角色，提供更友好的用户界面
- **Soda 解密**: 支持汽水音乐加密音频的解密功能
- **工程化结构**: 遵循 Go 标准项目布局，模块化设计
- **VIP过滤**: 自动过滤VIP和付费歌曲，仅返回免费可下载的歌曲
- **灵活选择**: 支持范围选择（如 `1-3`）、多选（如 `1 3 5`）和混合选择（如 `1-3,5,7-9`）
- **核心下载**: 使用统一的 `core.DownloadSong` 和 `core.DownloadSongWithCover` 函数，复用 `music-lib` 中封装好的下载逻辑
- **固定源顺序**: 确保 Web 界面和 CLI 的源选择顺序一致，提供更好的用户体验

## 支持的音源

本项目支持以下音乐源，每个源都有其特色和适用场景：

- **网易云音乐 (netease)**: 国内主流音乐平台，曲库丰富，包含大量原创和独立音乐人作品
- **QQ音乐 (qq)**: 腾讯旗下音乐平台，拥有大量正版音乐版权，特别是华语流行音乐
- **酷狗音乐 (kugou)**: 老牌音乐平台，以海量曲库和K歌功能著称
- **酷我音乐 (kuwo)**: 提供高品质音乐，支持多种音质选择，包括无损格式
- **咪咕音乐 (migu)**: 中国移动旗下音乐平台，拥有大量正版音乐资源
- **5sing原创音乐 (fivesing)**: 专注于原创音乐和翻唱作品的平台，适合寻找独立音乐人作品
- **Jamendo (jamendo)**: 国际免费音乐平台，所有音乐均可免费下载和使用
- **JOOX音乐 (joox)**: 腾讯国际版音乐平台，主要面向东南亚市场
- **千千音乐 (qianqian)**: 百度旗下音乐平台，整合了百度音乐资源
- **汽水音乐 (soda)**: 字节跳动旗下音乐平台，主打个性化推荐
- **Bilibili音频 (bilibili)**: 从B站视频中提取音频内容，包含大量二次创作和同人音乐

**注意**: 默认情况下排除 Bilibili 源，因为其内容多为视频音频，可能包含非音乐内容。如需使用可通过 `-s bilibili` 参数显式指定。

## 项目结构

```
go-music-dl/
├── cmd/
│   └── music-dl/
│       ├── main.go           # 程序入口
│       ├── root.go           # CLI 主命令逻辑 (完全对标 Python 版 music-dl)
│       └── web.go            # Web 子命令逻辑
├── core/                     # 核心逻辑层 (新增)
│   └── service.go           # 源映射管理和并发搜索
├── internal/
│   ├── cli/                  # CLI 交互逻辑 (Bubble Tea)
│   │   └── ui.go
│   └── web/                  # Web 服务逻辑 (Gin)
│       ├── server.go
│       └── templates/        # 嵌入的 HTML 模板
│           └── index.html
├── pkg/
│   └── models/               # 扩展数据模型
│       └── song.go           # 格式化方法 (时长、大小、文件名)
├── go.mod
├── go.sum
└── README.md
```

## 安装

### 前提条件
- Go 1.20 或更高版本
- Git

### 从源码安装

```bash
# 克隆项目
git clone https://github.com/guohuiyuan/go-music-dl.git
cd go-music-dl

# 安装依赖并编译
go mod tidy
go build -o music-dl ./cmd/music-dl

# 验证安装
./music-dl --version
```

### 使用 Nix 安装（实验性）

项目支持 Nix 包管理器，提供可重复的构建环境：

```bash
# 使用 Nix Flakes
nix run github:guohuiyuan/go-music-dl

# 或从本地克隆构建
git clone https://github.com/guohuiyuan/go-music-dl.git
cd go-music-dl
nix build
```

### 作为库使用

```bash
go get github.com/guohuiyuan/go-music-dl
```

### 自动依赖更新

项目使用 gomod2nix 管理 Nix 依赖。当 `go.mod` 或 `go.sum` 更新时，GitHub Actions 会自动运行 `gomod2nix` 更新 `gomod2nix.toml` 文件。

## 使用指南

### CLI 模式

#### 基本搜索
```bash
# 搜索歌曲（使用所有默认源）
./music-dl -k "周杰伦"

# 指定搜索源和结果数量
./music-dl -k "林俊杰" -s netease,qq -n 5

# 指定下载目录
./music-dl -k "邓紫棋" -o ~/Music
```

#### 完整参数
```bash
./music-dl --help
```

输出：
```
Search and download music from netease, qq, kugou, baidu and xiami.

Usage:
  music-dl [OPTIONS] [flags]
  music-dl [command]

Examples:
  music-dl -k "周杰伦"
  music-dl web

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  web         启动 Web 服务模式

Flags:
      --cover             同时下载歌词
      --filter string     按文件大小和歌曲时长过滤搜索结果
  -h, --help              help for music-dl
  -k, --keyword string    搜索关键字，歌名和歌手同时输入可以提高匹配
      --lyrics            同时下载歌词
      --nomerge           不对搜索结果列表排序和去重
  -n, --number int        Number of search results (default 10)
  -o, --outdir string     Output directory (default ".")
      --play              开启下载后自动播放功能
  -p, --playlist string   通过指定的歌单URL下载音乐
  -x, --proxy string      Proxy (e.g. http://127.0.0.1:1087)
  -s, --source strings    Supported music source (default [netease,qq,kugou,kuwo,migu])
  -u, --url string        通过指定的歌曲URL下载音乐
  -v, --verbose           Verbose mode
      --version           Show the version and exit.
```

### Web 模式

#### 启动 Web 服务
```bash
# 默认端口 8080
./music-dl web

# 指定端口
./music-dl web --port 9090
```

#### 访问 Web 界面
打开浏览器访问 `http://localhost:8080`，你将看到：
- 简洁的搜索界面，集成 Live2D 看板娘
- 表格化显示搜索结果（包含序号、歌名、歌手、专辑、时长、大小、来源、封面）
- 内置音乐播放器，支持在线试听和歌词同步显示
- 一键下载功能，文件自动命名为 `歌手 - 歌名.mp3`
- 支持汽水音乐加密音频的解密和播放

### CLI 交互界面

#### 交互式命令行模式
直接运行程序进入交互式命令行模式：
```bash
./music-dl
```
进入交互模式后：
```
🎵 欢迎使用 Go Music DL 交互式命令行
   输入 'q' 退出程序
   或直接输入歌名/歌手进行搜索

>> 命令行模式已启动
>> 默认启用源: [netease qq kugou kuwo migu fivesing jamendo joox qianqian soda] (已自动排除 bilibili)

[搜索] 请输入歌名或歌手 (输入 q 退出): 周杰伦
正在搜索...

找到 104 条结果:
序号      歌名                                       歌手                来源         大小/时长
----    ----                                     ----              ----       ----
[1]     晴天                                       周杰伦               kugou      04:29 (4.12MB)
[2]     青花瓷                                      周杰伦               kugou      03:59 (3.65MB)
[3]     稻香                                       周杰伦               kugou      03:43 (3.41MB)
...

[下载] 请输入序号 (多首用空格或逗号分隔，如 1 3 5): 1-3
```

#### Bubble Tea TUI 模式
使用 `-k` 参数搜索时，会进入交互式 TUI 界面：
```bash
./music-dl -k "周杰伦"
```
```
🔍 正在搜索: 周杰伦 ...
序号    歌名                 歌手           专辑               时长    大小    来源
───────────────────────────────────────────────────────────────────────────────

 >1    晴天                 周杰伦         叶惠美              04:29   4.12 MB kugou
  2    青花瓷               周杰伦         我很忙              03:59   3.65 MB kugou
  3    稻香                 周杰伦         魔杰座              03:43   3.41 MB kugou

j/k: 上下选择 • enter: 下载 • q: 退出
```

使用键盘控制：
- `j` / `↓`: 向下移动
- `k` / `↑`: 向上移动  
- `Enter`: 下载选中歌曲
- `q`: 退出

#### 灵活的选择方式
- **单个选择**: `1`
- **多选**: `1 3 5` 或 `1,3,5`
- **范围选择**: `1-3`（选择第1到第3首）
- **混合选择**: `1-3,5,7-9`（选择第1-3首、第5首、第7-9首）

## 开发指南

### 项目架构

#### 1. 核心逻辑层 (`core/`)
- `service.go`: 源映射管理和并发搜索
  - `SourceMap`: 统一管理所有音乐源的搜索函数映射
  - `SearchAndFilter()`: 支持多源并发搜索和 VIP 过滤
  - `GetDownloadURL()`: 根据源类型获取下载链接的统一接口
  - `GetAllSourceNames()`: 获取所有可用源的名称列表（固定顺序）
  - `DownloadSong()`: 基础下载函数，复用 `music-lib` 的下载逻辑
  - `DownloadSongWithCover()`: 增强下载函数，支持同时下载歌曲封面
  - `downloadCoverImage()`: 封面图片下载工具函数

#### 2. 命令行入口 (`cmd/music-dl/`)
- `root.go`: 主命令逻辑，完全对标 Python 版 music-dl 参数
- `web.go`: Web 子命令，支持端口配置
- `main.go`: 程序入口，集成 Cobra 框架

#### 3. CLI 交互模块 (`internal/cli/`)
- `ui.go`: 基于 Bubble Tea 的 TUI 界面
  - 表格化显示搜索结果，支持分页和键盘导航
  - 自动排除 bilibili 源，避免非音乐内容干扰
  - 调用 `core.DownloadSong` 进行下载，复用核心下载逻辑
- `handler.go`: 交互式命令行处理器
  - 提供更简单的命令行交互界面
  - 支持灵活的选择方式（范围选择、多选、混合选择）
  - 使用 `parseSelection` 函数解析用户输入
  - 自动过滤无效选择，确保下载稳定性

#### 4. Web 服务模块 (`internal/web/`)
- `server.go`: Gin Web 服务器
  - 多源选择支持（前端复选框）
  - 代理下载模式，解决 `.crdownload` 后缀问题
  - 双重 `Content-Disposition` 头部，兼容所有浏览器
  - 统一文件名设置：`歌手 - 歌名.mp3`
- `templates/index.html`: 响应式 Web 界面
  - 现代化 CSS 设计（CSS 变量、Flexbox、卡片布局）
  - 动态源选择器，支持全选/清空功能
  - 完整的歌曲元数据显示（时长、大小、专辑、来源）

#### 5. 数据模型 (`pkg/models/`)
- `song.go`: 扩展的 Song 结构体
  - `FormatDuration()`: 格式化时长 (03:45)
  - `FormatSize()`: 格式化大小 (4.5 MB)
  - `Filename()`: 生成统一文件名
  - 文件名清洗，防止非法字符

### 添加新的音乐源

1. 在 `music-lib` 中实现新的音乐源包
2. 在 `internal/cli/ui.go` 的 `Run()` 函数中添加搜索逻辑
3. 在 `internal/cli/handler.go` 的默认源列表中添加新源
4. 在 `internal/web/server.go` 的 `handleIndex()` 和 `handleDownload()` 中添加对应逻辑
5. 在 `core/service.go` 的 `SourceMap` 和 `GetDownloadURL` 函数中添加新源支持
6. 更新默认源列表（如果需要）

### 构建和测试

```bash
# 构建
go build -o music-dl ./cmd/music-dl

# 运行测试
go test ./...

# 清理
go clean
```

### 快速运行脚本

项目提供了跨平台的运行脚本：

**Windows (run.bat):**
```batch
run.bat
```

**Linux/macOS (run.sh):**
```bash
chmod +x run.sh
./run.sh
```

这些脚本会自动设置 Go 代理、整理依赖并运行程序。

### Windows 资源文件

项目包含 Windows 资源文件 (`winres/` 目录)，用于为 Windows 可执行文件添加图标和版本信息：
- `winres.json`: Windows 资源配置文件
- `icon.png`, `icon16.png`: 应用程序图标
- `init.go`: 资源初始化文件

使用 `go generate` 命令生成资源文件：
```bash
go generate ./...
```

### 自动化发布

项目使用 GoReleaser 进行自动化构建和发布：

**配置文件**: `.goreleaser.yml`
- 支持多平台构建 (Linux, Windows, macOS)
- 自动生成 Windows 资源
- 生成压缩包和校验和
- 支持 DEB 和 RPM 包格式

**GitHub Actions 工作流**:
- `release.yml`: 发布新版本时自动构建和发布
- `push.yml`: 推送代码时运行测试
- `pull.yml`: 拉取请求时运行测试
- `nightly.yml`: 每日构建
- `gomod2nix.yml`: 自动更新 Nix 依赖

### 开发工作流

1. **本地开发**:
   ```bash
   # 使用运行脚本
   ./run.sh  # Linux/macOS
   run.bat   # Windows
   ```

2. **构建发布版本**:
   ```bash
   # 使用 GoReleaser 进行本地测试
   goreleaser release --snapshot --clean
   ```

3. **生成 Windows 资源**:
   ```bash
   go generate ./...
   ```

4. **跨平台构建**:
   ```bash
   # 构建所有平台
   gox -osarch="linux/amd64 linux/386 windows/amd64 darwin/amd64"
   ```

## 配置说明

### 环境变量
暂无必需的环境变量，所有配置通过命令行参数传递。

### 代理设置
支持通过 `-x` 或 `--proxy` 参数设置代理：
```bash
./music-dl -k "歌曲名" -x "http://127.0.0.1:1087"
```

### 文件命名规则
下载的文件会自动重命名为 `歌手 - 歌名.mp3` 格式，并自动过滤文件名中的非法字符。

## 常见问题

### Q: 为什么某些歌曲无法下载？
A: 可能的原因：
1. 歌曲需要 VIP 权限
2. 音乐源 API 变更
3. 网络连接问题
4. 使用了 `core.DownloadSong` 进行下载，该函数会复用 `music-lib` 中封装好的下载逻辑，确保 Headers 伪装和防盗链处理

### Q: 为什么默认排除 Bilibili 源？
A: Bilibili 源通常包含大量非音乐视频音频，且格式复杂，容易导致搜索结果混乱。如果需要使用 Bilibili 源，可以通过 `-s bilibili` 参数显式指定。

### Q: 如何选择多首歌曲？
A: 支持多种选择方式：
- 空格分隔: `1 3 5`
- 逗号分隔: `1,3,5`
- 范围选择: `1-3`（选择第1到第3首）
- 混合选择: `1-3,5,7-9`

### Q: Web 模式启动失败？
A: 检查：
1. 端口是否被占用
2. 模板文件是否正确嵌入
3. 依赖是否完整安装

### Q: 如何添加自定义音乐源？
A: 参考 `music-lib` 中的实现，按照标准接口添加新的包。

### Q: 如何下载歌曲封面？
A: 当前版本支持封面下载功能：
1. 在 Web 界面中，搜索结果会显示封面图片
2. 下载歌曲时，封面图片会自动保存为 `歌手 - 歌名.jpg` 格式
3. 封面下载功能由 `core.DownloadSongWithCover` 函数提供支持
4. 如果封面下载失败，不会影响主音频文件的下载，仅记录警告日志

### Q: 如何查看歌词？
A: 当前版本支持歌词显示功能：
1. 在 Web 界面中，点击"试听"按钮会启动音乐播放器
2. 播放器会自动加载并显示同步歌词
3. 歌词使用 APlayer 播放器提供优美的同步显示效果，专注于当前播放歌词的高亮显示
4. 如果歌曲没有歌词，播放器会显示"暂无歌词"提示

### Q: 汽水音乐（Soda）的音频为什么需要解密？
A: 汽水音乐对音频文件进行了加密保护：
1. 下载的音频文件是加密的，无法直接播放
2. 项目内置了 AES-CTR 解密算法，自动解密音频
3. 解密过程对用户透明，下载后即可正常播放
4. 支持在线试听和下载后的解密播放

### Q: Live2D 看板娘是什么？
A: Live2D 是一种2D虚拟角色技术：
1. 在 Web 界面右下角显示虚拟角色
2. 提供更友好的用户交互体验
3. 支持鼠标悬停、点击等交互
4. 在移动端会自动隐藏以保持界面简洁

## 性能优化

- **并发搜索**: Web 和 CLI 模式都支持多源并发搜索
- **内存优化**: 使用流式处理，避免大文件内存占用
- **缓存策略**: 可扩展的缓存接口设计

## 许可证

GNU Affero General Public License v3.0

## 致谢

- [music-lib](https://github.com/guohuiyuan/music-lib): 核心音乐搜索库
- [Cobra](https://github.com/spf13/cobra): CLI 框架
- [Gin](https://github.com/gin-gonic/gin): Web 框架  
- [Bubble Tea](https://github.com/charmbracelet/bubbletea): TUI 框架
- [Tailwind CSS](https://tailwindcss.com/): CSS 框架
- [musicdl](https://github.com/CharlesPikachu/musicdl): 轻量级无损音乐下载器
- [music-dl](https://github.com/0xHJK/music-dl): 从网易云音乐、QQ音乐、酷狗音乐、百度音乐、虾米音乐、咪咕音乐等搜索和下载歌曲

## 贡献指南

1. Fork 项目
2. 创建功能分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

## 免责声明

本项目仅供学习和研究使用，请遵守相关法律法规和音乐平台的使用条款。下载的版权音乐请于24小时内删除，支持正版音乐。
