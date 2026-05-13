# Go Music DL

> ⭐ 如果这个项目正在帮你省时间，欢迎顺手点一个 Star。Star 越多，作者越能确认这个工具确实有人在用，也会更有动力优先修复失效站点、适配新站点和更新版本。

<p align="center">
  <img src="./internal/web/templates/static/images/icon.png" alt="Music Downloader Icon" width="220" />
</p>

Go Music DL 是一个音乐搜索与下载工具，支持 **Web 界面**、**TUI 终端** 和 **桌面应用** 三种使用模式。除了单曲搜索与下载外，还支持 **歌单搜索 / 解析**、**歌单分类浏览**、**我的歌单**、**专辑搜索 / 解析**、整单 / 整专曲目查看与批量处理。你可以在浏览器试听，也可以在终端里批量下载，或使用原生桌面应用享受最佳体验。

## 🚀 快速开始

### 桌面应用 (推荐)

最简单的使用方式，下载即用：

1. 从 [Releases](https://github.com/guohuiyuan/go-music-dl/releases) 下载 `music-dl-desktop-rust.exe` 或 `music-dl-desktop-go.exe`
2. 解压，双击运行
3. 享受原生桌面体验！

移动端下载说明：在 [Releases](https://github.com/guohuiyuan/go-music-dl/releases) 页面可直接下载 Android `music-dl_arm64-v8a.apk`（推荐）/ `music-dl_x86_64.apk` / `music-dl.apk`（无分片兼容包）与 iOS `music-dl-ios-unsigned.ipa`。

### Web 模式

```bash
./music-dl web

```

### TUI 模式

```bash
./music-dl -k "搜索关键词"

```

---

![Web UI 1](./screenshots/web1.png)
![Web UI 2](./screenshots/web2.png)

![TUI 1](./screenshots/tui1.png)
![TUI 2](./screenshots/tui2.png)

## 主要功能

* **多模式支持**: Web 界面、TUI 终端、桌面应用
* **本地自制歌单**: 支持新建本地收藏夹，随时收藏、管理心仪歌曲，数据持久化不丢失
* **歌单分类浏览**: Web 端可按平台查看官方分类，进入分类后浏览推荐歌单并打开详情
* **我的歌单**: Web 端可读取已登录账号的个人歌单 / 收藏歌单，支持进入详情和批量处理
* **Cookie 扫码登录**: Web 设置面板支持扫码获取 Cookie，成功后自动保存到本地配置
* **本地音乐管理**: Web 端可读取本地下载目录音乐，支持上传、添加到自制歌单、封面/歌词读取与删除
* **无损音乐支持**: 支持网易云、QQ 音乐、Bilibili 的 FLAC 无损音乐下载
* 多平台聚合搜索，支持单曲 / 歌单 / 专辑
* 试听、歌词、封面下载
* **歌词双格式支持**: 网易云、QQ 音乐、酷狗支持原文 / 译文 / 罗马音逐字 LRC 展示与卡拉 OK 式逐字高亮；其他渠道保持原文逐行歌词显示
* Range 探测：显示大小与码率
* 汽水音乐等加密音频解密
* 过滤需要付费的资源
* **桌面应用特性**: 原生窗口、自动服务启动、智能缓存管理

## 歌单 / 专辑支持

* **Web**: 支持单曲、歌单、专辑三种搜索类型切换，可直接查看歌单 / 专辑曲目列表
* **链接解析**: 支持直接粘贴歌单链接或专辑链接，自动识别来源并进入详情
* **歌单分类**: Web 端提供“歌单分类”入口，可在不同平台之间切换分类来源，并按分类浏览歌单
* **我的歌单**: Web 端提供“我的歌单”入口，登录后可查看个人创建、收藏或喜欢的歌单
* **TUI**: 输入界面支持在单曲 / 歌单 / 专辑之间切换，适合整单 / 整专处理
* **详情跳转**: Web 歌曲列表支持从歌曲跳转到歌手搜索结果或对应专辑页
* **渠道同步**: 已同步 `music-lib` 中咪咕、Jamendo、JOOX、千千、汽水等歌单 / 专辑函数，JOOX 歌单支持 OpenJOOX 接口和网页数据兜底

## 歌单分类与我的歌单

Web 首页的歌单入口旁提供 **歌单分类** 和 **我的歌单**：

* **歌单分类**: 支持网易云、QQ、酷狗、酷我、咪咕、千千、JOOX、Apple Music。进入后可切换平台标签，选择分类并查看该分类下的歌单。
* **我的歌单**: 支持网易云、QQ、酷狗、汽水。需要先在 Web 右上角“设置”中配置对应平台 Cookie；QQ 支持“我喜欢的歌曲”和收藏歌单，汽水支持“喜欢的音乐”和导入歌单，均可在站内解析歌曲列表。
* **详情与导入**: 分类歌单和我的歌单都可进入详情页，歌曲列表支持播放、下载、批量操作，也可导入到本地自制歌单。

## Cookie 与扫码登录

Web 右上角“设置”可管理各平台 Cookie。支持扫码登录的平台会在 Cookie 输入框右侧显示 **扫码** 按钮：

* **支持平台**: 网易云音乐、QQ 音乐、酷狗音乐、Bilibili、汽水音乐。
* **使用方式**: 点击对应平台“扫码”，用官方 App 扫码并确认登录；登录成功后 Cookie 会自动写入输入框并保存到本地。
* **汽水音乐**: 使用官方 PC passport 流程，界面显示 `scan_login_url` 生成的二维码；扫码确认后如果触发短信验证，会出现发送验证码和输入验证码步骤，验证通过后保存 Cookie。
* **用途**: Cookie 可用于获取更完整的搜索 / 下载能力、读取“我的歌单”、访问部分需要登录态的高音质或个人数据。
* **存储位置**: Cookie 会保存到本地数据目录，不会上传到第三方服务；请勿把包含 Cookie 的配置文件公开分享。

## 本地音乐

Web 端在“我的自制歌单”旁提供 **本地音乐** 入口，用于管理本地下载目录里的音频文件。下载目录来自 Web 右上角“设置”里的“本地下载目录”，默认是 `data/downloads`。Android 端建议把本地下载目录设置为 `/sdcard/Music`，便于系统音乐应用识别。

* **自动读取**: 打开本地音乐列表时会扫描下载目录，返回与普通歌曲列表一致的数据结构，来源标记为 `local`。
* **支持格式**: `mp3`、`flac`、`m4a`、`ogg`、`wav`、`wma`、`aac`。
* **上传音乐**: 可在弹窗中上传音频文件，文件会保存到下载目录；如文件名冲突，会自动追加序号。
* **信息补全**: 优先读取音频文件内的标题、歌手、专辑、时长、封面和歌词；缺失时会使用文件名、`Unknown` 等兜底信息。
* **耗时探测**: 时长、码率、标题、歌手、专辑等缺失信息会尽量通过 `ffprobe` 探测补齐；未安装 `ffprobe` 时仍可显示和添加，只是部分信息可能为空。
* **封面与歌词**: 封面优先读取音频内嵌图片，没有则查找同目录同名图片（`.jpg`、`.jpeg`、`.png`、`.webp`、`.bmp`、`.gif`）；歌词优先读取音频内嵌歌词，没有则查找同目录同名歌词文件（`.lrc`、`.txt`、`.lyric`）。
* **添加到歌单**: 在本地音乐弹窗选择目标自制歌单后，可把本地歌曲加入该歌单；外部导入歌单 / 专辑不支持直接添加本地音乐。
* **删除**: 本地音乐列表支持单曲删除和批量删除，删除前会二次确认；确认后会从本地下载目录移除实际文件。
* **限制**: 本地音乐不参与换源、批量换源、选择无效和批量下载等网络歌曲操作。

## Web 下载模式与 FFmpeg

Web 端“设置”里新增了 **下载时内嵌元数据（封面/歌词）** 开关：

* **默认关闭（推荐）**：走流式下载，速度更快，并支持 `Range` 断点/拖动播放。
* **开启后**：下载时会尝试把封面、歌词写入音频文件（embed）。

> ⚠️ 开启内嵌元数据依赖 **FFmpeg**。未安装 FFmpeg 时，会自动跳过内嵌并返回原始音频。

可先验证 FFmpeg 是否可用：

```bash
ffmpeg -version

```

常见安装方式：

* Windows: `winget install Gyan.FFmpeg`
* macOS: `brew install ffmpeg`
* Ubuntu/Debian: `sudo apt install ffmpeg`

### Docker / Release 包里的 FFmpeg 与 ffprobe

`ffprobe` 属于 FFmpeg 工具集，主要用于本地音乐的时长、码率和标签探测；`ffmpeg` 主要用于非 MP3 音频的封面/歌词元数据写入。缺少它们不会影响程序启动，也不会阻塞本地音乐列表加载，只会降级相关增强能力。

* **Docker 镜像**: `Dockerfile` 已安装 Alpine 的 `ffmpeg` 包，并在构建时校验 `ffmpeg` 与 `ffprobe` 都可用；Docker / Compose 部署通常无需额外安装。
* **GitHub Release 的 CLI / 桌面 / 移动端包**: `release.yml` 产物默认不内置 `ffmpeg/ffprobe`，也不要求 CI 打包机安装它们；如果需要本地音乐探测或非 MP3 元数据内嵌，请在运行机器上按上面的命令自行安装 FFmpeg。
* **Linux deb/rpm/AppImage**: 仍按外部系统工具处理，不强制声明硬依赖，避免在不同发行版上因为 FFmpeg 包源差异导致安装失败。

## 新增改动（简要）

* **Web 架构全面重构**：前端代码彻底模块化（拆分独立的 JS / CSS / HTML 模板），后端路由按业务域拆分（音乐查询、歌单管理、视频生成），大幅提升代码可维护性。
* **新增自制歌单功能**：Web 端支持本地收藏夹，用户可自由创建、编辑歌单，将不同平台的歌曲聚合收藏。
* **新增歌单分类与我的歌单**：Web 端支持官方分类浏览，也支持读取网易云、QQ、酷狗、汽水账号下的个人歌单 / 收藏歌单。
* **新增 Cookie 扫码登录**：设置面板支持网易云、QQ、酷狗、Bilibili、汽水音乐扫码登录，成功后自动保存 Cookie；汽水音乐如触发官方短信验证，会在扫码后继续显示验证码输入。
* **新增本地音乐功能**：Web 端支持读取本地下载目录、上传音频、自动补全元信息、读取同名封面/歌词，并可添加到自制歌单。
* **逐字歌词增强**：Web 首页、歌曲详情页与视频渲染统一支持网易云 / QQ 音乐 / 酷狗的原文、译文、罗马音逐字歌词；其他渠道继续使用原文逐行歌词。
* Web 试听按钮支持播放/停止切换，底部增加全局播放与音量控制栏。
* Web 单曲支持“换源”，按相似度优先、时长接近、可播放验证。
* 换源自动排除 soda 与 fivesing。
* TUI 增加 r 键批量换源，并显示换源进度。
* 增加“每日歌单推荐”，Web 和 TUI 都能看。
* 同步 `music-lib` 歌单 / 专辑渠道函数，补齐咪咕、Jamendo、JOOX、千千等歌单搜索、详情和链接解析映射。
* Web 端支持批量操作：全选、选择无效、批量下载、批量换源。

## 快速开始

### 桌面应用模式

桌面应用提供了原生窗口体验，无需打开浏览器即可使用。

#### 特性

* 🖥️ 原生桌面窗口，无需浏览器
* 🚀 自动启动内置Web服务器
* 🎵 完整Web界面功能
* 📦 单文件分发，绿色免安装
* 🖼️ 自定义窗口图标
* 🔒 使用罕见端口(37777)，避免端口冲突

### Docker 部署

本项目提供了多种 Docker 部署方式。当前默认通过 `./data` 目录挂载到容器内 `/home/appuser/data`，下载文件、配置与收藏数据都会持久化到该目录。

*注意：首次运行前必须先创建 `data` 目录（如 `mkdir -p data && chmod 777 data`），便于宿主机直接访问下载与配置数据。*

#### 1. 生产环境部署（推荐）

项目包含 `docker-compose.yml` 文件，直接拉取云端预编译镜像，无需在本地构建：

```bash
# 拉取最新镜像
docker compose pull

# 后台启动服务
docker compose up -d --remove-orphans

# 或一条命令拉取并启动
docker compose up -d --pull always --remove-orphans

# 查看日志
docker compose logs -f

# 停止服务
docker compose down

```

浏览器访问 `http://localhost:8080`。

**说明：**

* 自动拉取 `guohuiyuan/go-music-dl:latest` 镜像
* 支持后台运行和自动重启
* 默认使用 `./data` 本地目录做数据持久化，便于直接查看和备份
* 设置时区为亚洲上海
* 以非root用户(uid=1000)运行，提高安全性

#### 2. 开发环境部署（本地构建）

如果您修改了源码，希望在本地通过 Docker 重新构建并测试效果，请使用 `docker-compose.dev.yml`：

```bash
# 强制在本地使用 Dockerfile 进行构建并启动
docker compose -f docker-compose.dev.yml up -d --build --remove-orphans

```

#### 3. 纯命令行模式 (docker run)

如果不使用 Compose，也可以直接通过命令行运行：

```bash
docker run -d --name music-dl \
  -p 8080:8080 \
  -v $(pwd)/data:/home/appuser/data \
  -e TZ=Asia/Shanghai \
  --user 1000:1000 \
  --restart unless-stopped \
  guohuiyuan/go-music-dl:latest \
  ./music-dl web --port 8080 --no-browser

# Windows PowerShell
docker run -d --name music-dl -p 8080:8080 -v ${PWD}/data:/home/appuser/data -e TZ=Asia/Shanghai --user 1000:1000 --restart unless-stopped guohuiyuan/go-music-dl:latest ./music-dl web --port 8080 --no-browser

```

视频生成相关的“更换封面 / 更换音频 / 更换歌词 / 导出视频”按钮已迁移到 Web 设置中管理，默认关闭，可在网页右上角设置面板中开启。

### CLI/TUI 模式

```bash
# 搜索
./music-dl -k "周杰伦"

```

TUI 常用按键：

* `↑/↓` 移动
* `空格` 选择
* `a` 全选/清空
* `r` 对勾选项换源
* `Enter` 下载
* `b` 返回
* `w` 每日推荐歌单
* `q` 退出

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

## GitHub Actions 自动构建

本项目已配置 GitHub Actions 工作流。当推送代码并打上版本标签（如 `v1.0.0`）时，会自动触发 `.github/workflows/docker.yml`，构建跨平台镜像（支持 amd64 和 arm64）并推送到 DockerHub。

### Android APK 构建

项目支持通过 Gio 打包 Android APK，输出文件为仓库根目录下的三个 APK：

* `music-dl_arm64-v8a.apk`（推荐，大多数 Android 设备）
* `music-dl_x86_64.apk`（x86_64 设备/模拟器）
* `music-dl.apk`（无分片兼容包，极个别设备无法使用分片包时再下载）

安装 Android APK 后，建议在 Web 右上角“设置”中把“本地下载目录”改为 `/sdcard/Music`，这样下载文件会直接进入系统音乐目录。

#### 1. 本地构建 APK（Windows）

前置条件：

* Go 已安装并可用（建议 1.25+）
* JDK（推荐 17）
* Android SDK 已安装，默认路径 `C:\Android`
* 可用的 NDK（脚本会自动尝试安装 `27.0.12077973`）

执行命令：

```bat
cd go-music-dl
package_app.bat

```

脚本会自动：

* 读取当前 Java 环境（`JAVA_HOME` / `java`）
* 检测/安装 Android NDK
* 安装 `gogio`
* 构建 `music-dl_arm64-v8a.apk`、`music-dl_x86_64.apk`、`music-dl.apk`

若检测到 adb，会打印安装命令，例如：

```bat
adb install -r music-dl.apk

```

#### 2. Release 流程自动构建 APK

`.github/workflows/release.yml` 中新增了 `build-android-apk` 任务。发布时会在 `windows-latest` 环境中：

* 安装 Go、JDK 17、Android SDK
* 安装 `platform-tools`、`platforms;android-33`、`build-tools;34.0.0`、`ndk;27.0.12077973`
* 执行 `package_app.bat`
* 上传 `music-dl_arm64-v8a.apk`、`music-dl_x86_64.apk`、`music-dl.apk` 到 Actions Artifacts 和 GitHub Release

发布后可在 [Releases](https://github.com/guohuiyuan/go-music-dl/releases) 下载三个 APK，推荐优先使用 `music-dl_arm64-v8a.apk`；极个别设备若无法安装/运行，再下载 `music-dl.apk`（无分片兼容包）。

#### 3. Java 17 与 Build-Tools 版本说明（重点）

高版本（`34.0.0` 及以上）的 Android Build-Tools 已修复旧版 `d8.bat` 脚本兼容性问题，可正常配合 Java 17 使用。

如果本地仍有 `33.0.0`，建议升级并清理旧版本：

```cmd
"C:\Android\cmdline-tools\latest\bin\sdkmanager.bat" "build-tools;34.0.0"
```

如果你使用 Android Studio，也可以在 SDK Manager -> SDK Tools 中勾选 Show Package Details，然后安装 `34.0.0` 及以上版本。

非常关键：请到 `C:\Android\build-tools\` 目录下，删除或重命名 `33.0.0` 旧目录，避免 `gogio` 优先命中旧版 `d8`。

完成后再次执行：

```bat
cd go-music-dl
package_app.bat
```

### iOS App 构建

项目已提供 iOS 打包脚本：`package_ios.sh`。

#### 1. 构建环境

* macOS（需安装 Xcode Command Line Tools）
* Go 已安装并可用
* 可使用 `sudo`（脚本会在必要时补齐 Xcode 工具链中的 `libarclite` 文件）

#### 2. 执行构建

```bash
cd go-music-dl
chmod +x package_ios.sh
./package_ios.sh
```

脚本会自动：

* 检查并补齐 `libarclite_iphonesimulator.a` / `libarclite_iphoneos.a`
* 安装 `gogio`
* 构建 iOS App（输出 `music-dl.app`）
* 打包为未签名 IPA（输出 `music-dl-ios-unsigned.ipa`）

#### 3. 产物说明

* `music-dl.app`：iOS 应用包目录
* `music-dl-ios-unsigned.ipa`：未签名安装包，通常用于后续签名与分发流程

发布后可在 [Releases](https://github.com/guohuiyuan/go-music-dl/releases) 下载 `music-dl-ios-unsigned.ipa`。

> 注意：脚本会修改 Xcode 工具链目录（需要管理员权限），建议在受控开发环境中执行。

**如果你 Fork 了本仓库并希望使用自己的构建流：**

1. 在你的仓库 **Settings** -> **Secrets and variables** -> **Actions** 中添加：

* `DOCKERHUB_USERNAME`: 你的 DockerHub 用户名
* `DOCKERHUB_TOKEN`: 你的 DockerHub 访问令牌

2. 将 `docker-compose.yml` 中的镜像地址修改为你自己的：`image: 你的用户名/go-music-dl:latest`

## Web 换源说明

单曲卡片里的“换源”会在其它平台里找更像的版本：

* 先看歌名/歌手相似度
* 再看时长差异（太大就跳过）
* 最后做可播放探测

当前会跳过 soda 与 fivesing。

## 每日歌单推荐

Web 页面有“每日推荐”入口，会聚合网易云、QQ、酷狗、酷我。
TUI 在输入界面按 `w` 直接拉取推荐歌单，然后回车进详情。

## 支持平台

| 平台        | 包名         | 搜索 | 下载 | 歌词 | 歌曲解析 | 歌单搜索 | 歌单推荐 | 歌单分类 | 我的歌单 | 扫码登录    | 歌单歌曲 | 歌单链接解析 | 专辑搜索 | 专辑歌曲 | 专辑链接解析 | 备注                                   |
| ----------- | ------------ | ---- | ---- | ---- | -------- | -------- | -------- | -------- | -------- | ----------- | -------- | ------------ | -------- | -------- | ------------ | -------------------------------------- |
| 网易云音乐  | `netease`  | ✅   | ✅   | ✅   | ✅       | ✅       | ✅       | ✅       | ✅       | ✅          | ✅       | ✅           | ✅       | ✅       | ✅           | 支持 FLAC 无损                         |
| QQ 音乐     | `qq`       | ✅   | ✅   | ✅   | ✅       | ✅       | ✅       | ✅       | ✅       | ✅          | ✅       | ✅           | ✅       | ✅       | ✅           | 支持 FLAC 无损                         |
| 酷狗音乐    | `kugou`    | ✅   | ✅   | ✅   | ✅       | ✅       | ✅       | ✅       | ✅       | ✅          | ✅       | ✅           | ✅       | ✅       | ✅           | 支持普通歌曲 FLAC 无损                 |
| 酷我音乐    | `kuwo`     | ✅   | ✅   | ✅   | ✅       | ✅       | ✅       | ✅       | ❌       | ❌          | ✅       | ✅           | ✅       | ✅       | ✅           |                                        |
| 咪咕音乐    | `migu`     | ✅   | ✅   | ✅   | ✅       | ✅       | ❌       | ✅       | ❌       | ❌          | ✅       | ✅           | ✅       | ✅       | ✅           | 歌单歌曲使用 MIGUM3 接口               |
| 千千音乐    | `qianqian` | ✅   | ✅   | ✅   | ✅       | ⚠️     | ❌       | ✅       | ❌       | ❌          | ✅       | ✅           | ✅       | ✅       | ✅           | 歌单搜索可能返回空，已知 ID/链接可解析 |
| 汽水音乐    | `soda`     | ✅   | ✅   | ✅   | ✅       | ✅       | ❌       | ❌       | ✅       | ✅ 短信验证 | ✅       | ✅           | ✅       | ✅       | ✅           | 音频解密，支持短链和个人歌单           |
| 5sing       | `fivesing` | ✅   | ✅   | ✅   | ✅       | ✅       | ❌       | ❌       | ❌       | ❌          | ✅       | ✅           | ❌       | ❌       | ❌           |                                        |
| Jamendo     | `jamendo`  | ✅   | ✅   | ✅   | ✅       | ⚠️     | ❌       | ❌       | ❌       | ❌          | ✅       | ✅           | ✅       | ✅       | ✅           | 歌单搜索可能返回空，公开歌单链接可解析 |
| JOOX        | `joox`     | ✅   | ✅   | ✅   | ✅       | ✅       | ❌       | ✅       | ❌       | ❌          | ✅       | ✅           | ✅       | ✅       | ✅           | 歌单支持 OpenJOOX 接口和网页数据兜底   |
| Bilibili    | `bilibili` | ✅   | ✅   | ✅   | ✅       | ✅       | ❌       | ❌       | ❌       | ✅          | ✅       | ✅           | ❌       | ❌       | ❌           | 支持 FLAC 无损                         |
| Apple Music | `apple`    | ✅   | ⚠️ | ✅   | ✅       | ✅       | ❌       | ✅       | ❌       | ❌          | ✅       | ✅           | ✅       | ✅       | ✅           | 下载仅 preview，完整需 gamdl 解密      |

> `⚠️` 表示方法已接入，但平台搜索接口结果不稳定；优先使用已知 ID 或链接解析。

## 歌曲链接解析

支持直接解析音乐分享链接：

```bash
./music-dl -k "[https://music.163.com/#/song?id=123456](https://music.163.com/#/song?id=123456)"

```

支持解析的平台：网易云、QQ音乐、酷狗、酷我、咪咕、Bilibili、汽水音乐、5sing、Jamendo、JOOX、千千音乐、Apple Music。

## 歌单链接解析

支持直接解析歌单/合集分享链接：

```bash
./music-dl -k "[https://music.163.com/#/playlist?id=123456](https://music.163.com/#/playlist?id=123456)"

```

支持解析的平台：网易云、QQ音乐、酷狗、酷我、咪咕、Jamendo、JOOX、千千音乐、汽水音乐、5sing、Bilibili、Apple Music。

## 专辑链接解析

支持直接解析专辑分享链接：

```bash
./music-dl -k "[https://music.163.com/#/album?id=123456](https://music.163.com/#/album?id=123456)"

```

支持解析的平台：网易云、QQ音乐、酷狗、酷我、咪咕、Jamendo、JOOX、千千音乐、汽水音乐、Apple Music。

## 常见问题

### 桌面应用相关

**Q: 桌面应用打不开或显示空白？**
检查是否已安装 WebView2 运行时。从 [Microsoft官网](https://developer.microsoft.com/microsoft-edge/webview2/) 下载安装最新版本。

**Q: 桌面应用启动慢或卡顿？**
首次运行需要下载 WebView2 运行时。也可提前安装 Evergreen Bootstrapper 版本。

**Q: 桌面应用启动时提示"另一个程序正在使用此文件"？**
这是因为上一次运行的后台进程没有正常退出。解决方案：

```powershell
# 强制结束残留进程
taskkill /F /IM music-dl.exe

```

**Q: 如何构建桌面应用？**

构建 Rust 桌面应用

```bash
# 1. 构建 Go 二进制
go build -o desktop/music-dl.exe cmd/music-dl/main.go

# 2. 构建 Rust 桌面应用
cd desktop
cargo build --release

```

构建纯 Go 的桌面应用

```
cd desktop

# Windows 
go build -ldflags="-H windowsgui"

# Linux 
go build

```

**Q: 桌面应用支持哪些平台？**
目前支持 Windows (x64/x86/arm64)、macOS (x64/arm64)、Linux (x64)。

### 通用问题

**Q: 有些歌搜不到或下载失败？**
可能是付费限制、平台接口变更或网络问题。

**Q: Web 模式打不开？**
检查端口是否占用，或浏览器插件是否拦截。

**Q: 如何设置 Cookie 获取更高音质？**
Web 右上角“设置”里可添加平台 Cookie。网易云音乐、QQ 音乐、酷狗音乐、Bilibili、汽水音乐支持点击输入框右侧的“扫码”按钮登录，扫码成功后会自动保存 Cookie。汽水音乐如果触发官方短信验证，按界面提示发送并输入验证码即可。

**Q: 如何查看歌单分类和我的歌单？**
Web 首页歌单入口旁提供“歌单分类”和“我的歌单”。歌单分类可直接浏览支持平台的官方分类；我的歌单需要先配置对应平台 Cookie，目前支持网易云音乐、QQ 音乐、酷狗音乐和汽水音乐。

**Q: 开启“内嵌元数据”后没生效？**
先确认系统已安装 FFmpeg 且 `ffmpeg -version` 可执行；若不可用，程序会降级为原始音频下载（不内嵌封面/歌词）。

## 项目结构

```text
go-music-dl/
├── cmd/
│   └── music-dl/          # CLI/TUI 主程序
├── core/                  # 核心业务逻辑
├── internal/
│   ├── cli/               # TUI 界面 (如: ui.go)
│   └── web/               # 重构后的 Web 后端服务
│       ├── templates/     # 前端模板与静态资源分离
│       ├── server.go      # Web 服务主入口
│       ├── music.go       # 音乐搜索与解析路由
│       ├── collection.go  # 本地自制歌单接口 (GORM)
│       ├── local_music.go # 本地音乐扫描、上传、封面/歌词与删除接口
│       └── videogen.go    # 视频生成后端支持
├── desktop/               # 桌面应用 (Rust + Tao/Wry)
├── desktop_go/            # 桌面应用 (Go + webview2 )
├── desktop_app/           # 移动应用 (Go + Gio )
├── data/                  # 🌟 统一数据持久化目录 (Docker挂载点)
│   ├── downloads/         # 下载的音乐文件
│   ├── video_output/      # 生成的视频文件
│   ├── cookies.json       # Cookie 配置文件
│   └── settings.db        # 统一 SQLite 数据库（设置 / Cookie / 自制歌单）
├── .github/workflows/     # GitHub Actions 工作流
├── screenshots/           # 截图资源
├── docker-compose.yml     # Docker 生产环境配置 (直接拉取镜像)
├── docker-compose.dev.yml # Docker 开发环境配置 (本地构建)
├── Dockerfile             # Docker 构建配置
├── go.mod                 # Go 模块配置
├── README.md              # 主项目说明
├── package.bat            # 构建Rust桌面程序脚本
├── package_go.bat         # 构建Go桌面程序脚本
├── package_app.bat        # 构建Android移动应用脚本
├── package_ios.bat        # 构建IOS移动应用脚本
├── run.bat                # Go Music DL - 启动脚本 (Windows)
└── run.sh                 # Go Music DL - 启动脚本 (Linux/macOS)
```

## 技术栈

* **核心库**: [music-lib](https://github.com/guohuiyuan/music-lib) - 音乐平台搜索下载
* **CLI 框架**: [Cobra](https://github.com/spf13/cobra) - 命令行工具
* **Web 框架**: [Gin](https://github.com/gin-gonic/gin) - Web 框架
* **TUI 框架**: [Bubble Tea](https://github.com/charmbracelet/bubbletea) - 终端界面
* **桌面框架**: [Tao](https://github.com/tauri-apps/tao) + [Wry](https://github.com/tauri-apps/wry) - 跨平台桌面应用
* **图像处理**: [image](https://github.com/image-rs/image) - 图标处理

### 桌面应用架构

桌面应用采用前后端分离架构：

* **前端**: Rust + Tao/Wry - 负责窗口管理、WebView 渲染和进程管理
* **后端**: Go 二进制 - 嵌入到桌面应用中，提供 Web 服务和音乐功能
* **通信**: HTTP 本地服务 - 前后端通过 `http://localhost:37777` 通信

详细说明请参考 [desktop/README.md](desktop/README.md)

## 贡献

欢迎提交 Issue 或 Pull Request。

## 许可证

本项目遵循 GNU Affero General Public License v3.0（AGPL-3.0）。详情见 [LICENSE](LICENSE)。

## 致敬

感谢以下优秀的开源项目：

* **下载库**: [0xHJK/music-dl](https://github.com/0xHJK/music-dl) - 音乐下载库
* **下载库**: [CharlesPikachu/musicdl](https://github.com/CharlesPikachu/musicdl) - 音乐下载库
* **接口设计参考**: [metowolf/Meting](https://github.com/metowolf/Meting) - 多平台音乐聚合与接口封装
* **无损音乐**: [Suxiaoqinx/Netease_url](https://github.com/Suxiaoqinx/Netease_url) - 网易云音乐 FLAC 无损音乐解析
* **QQ 音乐**: [Suxiaoqinx/qqmusic_flac](https://github.com/Suxiaoqinx/qqmusic_flac) - QQ 音乐 FLAC 解析
* **逐字歌词展示参考**: [chenmozhijin/LDDC](https://github.com/chenmozhijin/LDDC) - 原文 / 译文 / 罗马音逐字歌词的组织与展示思路参考

## 免责声明

仅供学习和技术交流使用。下载的音乐资源请在 24 小时内删除。

## Star History

[![Star History Chart](https://api.star-history.com/image?repos=guohuiyuan/go-music-dl&type=date&legend=top-left)](https://www.star-history.com/?repos=guohuiyuan%2Fgo-music-dl&type=date&legend=top-left)
