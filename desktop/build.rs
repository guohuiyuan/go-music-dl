// build.rs

fn main() {
    // 仅在 Windows 平台上运行此逻辑
    #[cfg(windows)]
    {
        // 创建一个新的 Windows 资源项
        let mut res = winres::WindowsResource::new();
        
        // 设置图标路径。
        // 这里假设 "icon.ico" 位于你的 Cargo.toml 文件所在的目录（即 desktop 目录）。
        res.set_icon("icon.ico");

        // 编译资源。如果失败则 panic 终止构建。
        if let Err(e) = res.compile() {
            eprintln!("Error compiling Windows resources: {}", e);
            // 确保在图标缺失或错误时构建失败，以便你发现问题
            std::process::exit(1);
        }
    }
}