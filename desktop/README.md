# Go Music DL Desktop

桌面客户端版本的 [Go Music DL](https://github.com/guohuiyuan/go-music-dl)，提供原生桌面窗口体验，无需打开浏览器即可使用完整的 Web 界面功能。

## ✨ 特性

- 🖥️ **原生桌面窗口** - 无需浏览器，直接在桌面应用中使用
- 🚀 **自动启动服务** - 内置 Web 服务器，启动即用
- 🎵 **完整功能** - 支持所有 Web 版本的功能（搜索、下载、试听等）
- 📦 **绿色免安装** - 单文件分发，无需安装依赖
- 🖼️ **自定义图标** - 专业的窗口图标设计
- 🔒 **安全端口** - 使用 37777 端口，避免常见端口冲突
- 👻 **静默运行** - 后台启动，不打开额外浏览器窗口
- 🛑 **智能清理** - 关闭窗口时自动终止所有相关进程
- 🗂️ **缓存管理** - WebView 数据存储在系统临时目录，不污染程序目录

## 🏗️ 技术实现

### 核心技术栈

- **Rust** - 主要编程语言
- **Tao** - 跨平台窗口管理
- **Wry** - WebView 渲染引擎
- **WebView2** (Windows) - Microsoft Edge 内核

### 架构设计

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Desktop App   │    │   Web Server    │    │   Web UI        │
│   (Rust + Tao)  │◄──►│   (Go binary)   │◄──►│   (HTML/JS)     │
│                 │    │                 │    │                 │
│ • 窗口管理      │    │ • REST API      │    │ • 搜索界面      │
│ • WebView 渲染  │    │ • 音乐下载      │    │ • 播放控制      │
│ • 进程管理      │    │ • 文件处理      │    │ • 批量操作      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### 关键特性

1. **嵌入式二进制** - Go 后端编译为二进制文件，嵌入到 Rust 可执行文件中
2. **进程隔离** - 前后端分离，桌面应用只负责窗口和进程管理
3. **智能清理** - 程序退出时自动清理临时文件和后台进程
4. **跨平台兼容** - 支持 Windows/macOS/Linux 构建

## 🚀 快速开始

### 下载使用

1. 从 [Releases](https://github.com/guohuiyuan/go-music-dl/releases) 页面下载对应平台的压缩包
2. 解压到任意目录
3. 双击运行可执行文件
4. 应用将自动启动内置 Web 服务器并打开桌面窗口

### 系统要求

- **Windows**: Windows 10+ (WebView2 运行时)
- **macOS**: macOS 10.15+
- **Linux**: 支持 WebKitGTK 的发行版

## 🛠️ 开发指南

### 环境准备

```bash
# 安装 Rust
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh

# 安装依赖 (Windows)
# WebView2 运行时会自动下载

# 安装依赖 (macOS)
brew install webkit2gtk

# 安装依赖 (Linux)
sudo apt-get install libwebkit2gtk-4.0-dev
```

### 构建项目

```bash
# 进入桌面应用目录
cd desktop

# 构建调试版本
cargo build

# 构建发布版本
cargo build --release

# 运行应用
cargo run
```

### 项目结构

```
desktop/
├── src/
│   └── main.rs          # 主程序逻辑
├── Cargo.toml           # Rust 依赖配置
├── build.rs             # 构建脚本 (Windows 图标)
├── icon.png             # 应用图标
└── index.html           # 备用 HTML (未使用)
```

### 构建配置

项目支持跨平台构建，`Cargo.toml` 中包含了不同平台的打包配置：

- **Windows**: MSI 安装包
- **macOS**: DMG 镜像
- **Linux**: DEB/RPM 包

## 📋 功能说明

### 核心功能

- ✅ 音乐搜索与下载
- ✅ 实时试听
- ✅ 批量操作
- ✅ 歌词下载
- ✅ 封面获取
- ✅ 多平台支持
- ✅ 加密音频解密

### 界面特性

- 🎨 现代化 Web 界面
- 📱 响应式设计
- 🌙 深色模式支持
- ⌨️ 键盘快捷键
- 🔍 实时搜索
- 📊 下载进度显示

## 🔧 配置说明

### 端口配置

默认使用端口 `37777`，如需修改请编辑 `src/main.rs` 中的 `server_config::PORT`。

### 数据存储

- **临时文件**: 存储在系统临时目录 (`%TEMP%` 或 `/tmp`)
- **WebView 缓存**: 存储在 `go-music-dl-webview-data` 文件夹
- **下载文件**: 保存在用户选择的位置

## 🐛 故障排除

### 常见问题

#### 1. 程序启动失败

**现象**: 双击后无反应或闪退
**解决**:
- 检查是否已安装 WebView2 运行时 (Windows)
- 查看控制台输出获取错误信息
- 确保端口 37777 未被占用

#### 2. 文件锁定错误

**现象**: "另一个程序正在使用此文件"
**解决**:
```powershell
# 强制结束残留进程
taskkill /F /IM music-dl.exe
```

#### 3. WebView2 相关错误

**现象**: WebView 无法加载
**解决**:
- 安装最新的 WebView2 运行时
- 检查网络连接
- 查看防火墙设置

#### 4. 构建失败

**现象**: 编译时出现依赖错误
**解决**:
```bash
# 更新依赖
cargo update

# 清理构建缓存
cargo clean

# 重新构建
cargo build
```

### 日志调试

程序会在控制台输出详细的启动和运行信息：

```bash
cargo run 2>&1 | tee debug.log
```

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request！

### 开发规范

1. 遵循 Rust 代码规范
2. 添加必要的错误处理
3. 更新文档和注释
4. 测试跨平台兼容性

### 构建发布

```bash
# 创建发布构建
cargo build --release

# Windows 平台
# 生成 MSI 安装包
cargo wix --package music-dl-desktop-rust

# macOS 平台
# 生成 DMG
cargo bundle --release

# Linux 平台
# 生成 DEB 包
cargo deb --no-build
```

## 📄 许可证

本项目采用 AGPL-3.0 许可证，详见主项目 [LICENSE](../LICENSE) 文件。

## 🙏 致谢

- [Tao](https://github.com/tauri-apps/tao) - 窗口管理框架
- [Wry](https://github.com/tauri-apps/wry) - WebView 绑定
- [Go Music DL](https://github.com/guohuiyuan/go-music-dl) - 后端音乐服务

---

**注意**: 这是一个桌面客户端项目，核心音乐功能由 [Go Music DL](https://github.com/guohuiyuan/go-music-dl) 提供。
