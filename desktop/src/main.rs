use std::process::Command;
use std::thread;
use std::time::Duration;
use tao::event_loop::{ControlFlow, EventLoop};
use tao::window::WindowBuilder;
use wry::WebViewBuilder;

#[cfg(target_os = "windows")]
use std::os::windows::process::CommandExt;

// ==========================================
//                 常量配置区域
// ==========================================

/// 窗口配置
mod window_config {
    pub const TITLE: &str = "Music DL Desktop";
    pub const WIDTH: f64 = 1280.0;
    pub const HEIGHT: f64 = 800.0;
    // 注意：include_bytes! 宏必须使用字符串字面量，不能使用常量变量
    // 所以这里只定义路径作为注释参考，实际代码中仍需写死
    // pub const ICON_PATH: &str = "../icon.png";
}

/// 后端服务配置 (Go程序)
mod server_config {
    pub const PORT: &str = "37777";
    pub const URL_PATH: &str = "/music/";
    pub const STARTUP_DELAY_MS: u64 = 2000;
    pub const SHUTDOWN_DELAY_MS: u64 = 500;

    // 根据操作系统决定二进制文件名
    #[cfg(target_os = "windows")]
    pub const BINARY_NAME: &str = "music-dl.exe";
    #[cfg(not(target_os = "windows"))]
    pub const BINARY_NAME: &str = "music-dl";
}

// ==========================================
//                 嵌入式二进制文件
// ==========================================

/// 嵌入的Go后端二进制文件
/// 注意：此文件在构建时由build.rs自动生成到项目根目录
/// 路径解析：desktop/src/main.rs -> ../../ -> go-music-dl/music-dl.exe
#[cfg(target_os = "windows")]
static MUSIC_DL_BINARY: &[u8] = include_bytes!("../../music-dl.exe");
#[cfg(not(target_os = "windows"))]
static MUSIC_DL_BINARY: &[u8] = include_bytes!("../../music-dl");

/// 系统/进程相关配置
mod system_config {
    #[cfg(target_os = "windows")]
    pub const CREATE_NO_WINDOW_FLAG: u32 = 0x08000000;
}

// ==========================================
//                 主程序逻辑
// ==========================================

fn main() -> wry::Result<()> {
    // 1. 将嵌入的Go二进制文件提取到临时文件
    let temp_dir = std::env::temp_dir();
    let temp_binary_path = temp_dir.join(server_config::BINARY_NAME);

    println!("Extracting embedded music-dl binary to: {:?}", temp_binary_path);
    std::fs::write(&temp_binary_path, MUSIC_DL_BINARY)
        .expect("Failed to write embedded binary to temp file");

    // 设置临时文件为可执行权限 (Unix系统)
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        let mut perms = std::fs::metadata(&temp_binary_path)
            .expect("Failed to get metadata")
            .permissions();
        perms.set_mode(0o755);
        std::fs::set_permissions(&temp_binary_path, perms)
            .expect("Failed to set executable permissions");
    }

    // 2. 启动 Go Web 服务

    let path = temp_binary_path.to_str().unwrap();
    println!("Starting backend server with embedded binary: {}", path);

    let mut cmd = Command::new(&path);
    cmd.arg("web")
       .arg("--no-browser")
       .arg("-p").arg(server_config::PORT);

    // Windows 专用：隐藏控制台窗口
    #[cfg(target_os = "windows")]
    {
        cmd.creation_flags(system_config::CREATE_NO_WINDOW_FLAG);
    }

    let mut child = if let Ok(process) = cmd.spawn() {
        println!("Backend server started successfully");
        process
    } else {
        eprintln!("Failed to start backend server");
        // 清理临时文件
        let _ = std::fs::remove_file(&temp_binary_path);
        panic!("Failed to start Go backend server");
    };

    // 等待服务启动
    thread::sleep(Duration::from_millis(server_config::STARTUP_DELAY_MS));

    // 2. 加载图标
    // include_bytes! 必须使用字面量路径，无法使用 const
    const ICON_DATA: &[u8] = include_bytes!("../icon.png");
    let icon = match image::load_from_memory(ICON_DATA) {
        Ok(img) => {
            let icon_rgba = img.to_rgba8();
            let (width, height) = icon_rgba.dimensions();
            Some(tao::window::Icon::from_rgba(icon_rgba.into_raw(), width, height).unwrap())
        }
        Err(_) => None,
    };

    // 3. 创建窗口
    let event_loop = EventLoop::new();
    let window = WindowBuilder::new()
        .with_title(window_config::TITLE)
        .with_inner_size(tao::dpi::LogicalSize::new(window_config::WIDTH, window_config::HEIGHT))
        .with_window_icon(icon)
        .build(&event_loop)
        .unwrap();

    // 4. 加载 WebView
    // 动态构建 URL：http://localhost:PORT/PATH
    let server_url = format!("http://localhost:{}{}", server_config::PORT, server_config::URL_PATH);
    let _webview = WebViewBuilder::new(&window)
        .with_url(&server_url)
        .build()?;

    // 5. 事件循环
    event_loop.run(move |event, _, control_flow| {
        *control_flow = ControlFlow::Wait;

        match event {
            tao::event::Event::WindowEvent {
                event: tao::event::WindowEvent::CloseRequested,
                ..
            } => {
                println!("Terminating web server...");

                // 尝试优雅关闭子进程
                if let Err(e) = child.kill() {
                    eprintln!("Failed to kill child process: {}", e);

                    // Windows 兜底策略：使用 taskkill 强制结束
                    #[cfg(target_os = "windows")]
                    {
                        let _ = std::process::Command::new("taskkill")
                            .args(&["/F", "/IM", server_config::BINARY_NAME])
                            .output();
                    }
                }

                thread::sleep(Duration::from_millis(server_config::SHUTDOWN_DELAY_MS));

                // 清理临时文件
                if let Err(e) = std::fs::remove_file(&temp_binary_path) {
                    eprintln!("Failed to clean up temp binary file: {}", e);
                } else {
                    println!("Cleaned up temporary binary file");
                }

                println!("Web server terminated. Exiting...");
                *control_flow = ControlFlow::Exit;
            }
            _ => (),
        }
    });
}