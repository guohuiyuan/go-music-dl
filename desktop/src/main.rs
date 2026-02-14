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
}

/// 后端服务配置 (Go程序)
mod server_config {
    pub const PORT: &str = "37777";
    pub const URL_PATH: &str = "/music/"; // 确保路径以 / 结尾或开头匹配你的后端路由
    pub const STARTUP_DELAY_MS: u64 = 2000;

    // 根据操作系统决定二进制文件名
    #[cfg(target_os = "windows")]
    pub const BINARY_NAME: &str = "music-dl.exe";
    #[cfg(not(target_os = "windows"))]
    pub const BINARY_NAME: &str = "music-dl";
}

/// 系统/进程相关配置
mod system_config {
    #[cfg(target_os = "windows")]
    pub const CREATE_NO_WINDOW_FLAG: u32 = 0x08000000;
}

// ==========================================
//                 嵌入式二进制文件
// ==========================================

/// 嵌入的Go后端二进制文件
/// 路径解析：go-music-dl/desktop/src/main.rs -> ../../../ -> music-dl.exe
#[cfg(target_os = "windows")]
static MUSIC_DL_BINARY: &[u8] = include_bytes!("../../../music-dl.exe");
#[cfg(not(target_os = "windows"))]
static MUSIC_DL_BINARY: &[u8] = include_bytes!("../../../music-dl");

// ==========================================
//                 主程序逻辑
// ==========================================

fn main() -> wry::Result<()> {
    // 1. 将嵌入的Go二进制文件提取到临时文件
    let temp_dir = std::env::temp_dir();
    let temp_binary_path = temp_dir.join(server_config::BINARY_NAME);

    // 如果临时文件已存在，先尝试删除（防止旧版本残留）
    if temp_binary_path.exists() {
        let _ = std::fs::remove_file(&temp_binary_path);
    }

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

    let mut child = match cmd.spawn() {
        Ok(process) => {
            println!("Backend server started successfully");
            process
        }
        Err(e) => {
            eprintln!("Failed to start backend server: {}", e);
            // 清理临时文件
            let _ = std::fs::remove_file(&temp_binary_path);
            panic!("Failed to start Go backend server");
        }
    };

    // 等待服务启动
    thread::sleep(Duration::from_millis(server_config::STARTUP_DELAY_MS));

    // 3. 加载图标
    // include_bytes! 必须使用字面量路径
    const ICON_DATA: &[u8] = include_bytes!("../icon.png");
    let icon = match image::load_from_memory(ICON_DATA) {
        Ok(img) => {
            let icon_rgba = img.to_rgba8();
            let (width, height) = icon_rgba.dimensions();
            Some(tao::window::Icon::from_rgba(icon_rgba.into_raw(), width, height).unwrap())
        }
        Err(_) => None,
    };

    // 4. 创建窗口
    let event_loop = EventLoop::new();
    let window = WindowBuilder::new()
        .with_title(window_config::TITLE)
        .with_inner_size(tao::dpi::LogicalSize::new(window_config::WIDTH, window_config::HEIGHT))
        .with_window_icon(icon)
        .build(&event_loop)
        .unwrap();

    // 5. 加载 WebView
    let server_url = format!("http://localhost:{}{}", server_config::PORT, server_config::URL_PATH);
    let _webview = WebViewBuilder::new(&window)
        .with_url(&server_url)
        .build()?;

    // 6. 事件循环
    event_loop.run(move |event, _, control_flow| {
        *control_flow = ControlFlow::Wait;

        match event {
            tao::event::Event::WindowEvent {
                event: tao::event::WindowEvent::CloseRequested,
                ..
            } => {
                println!("Terminating web server...");

                // -----------------------------------------------------------
                // 步骤 1: 终止子进程
                // -----------------------------------------------------------
                let _ = child.kill(); // 发送终止信号
                
                // Windows 兜底策略：如果 kill 之后进程还在，用 taskkill 强杀
                #[cfg(target_os = "windows")]
                {
                    let _ = std::process::Command::new("taskkill")
                        .args(&["/F", "/IM", server_config::BINARY_NAME])
                        .output();
                }

                // -----------------------------------------------------------
                // 步骤 2: 关键点 —— 等待进程真正退出 (释放文件锁的关键)
                // -----------------------------------------------------------
                // wait() 会阻塞直到子进程彻底消失。
                // 如果不加这一行，代码会立即跑去删文件，而此时文件还被占用。
                match child.wait() {
                    Ok(status) => println!("Backend process exited with: {}", status),
                    Err(e) => eprintln!("Error waiting for process: {}", e),
                }

                // -----------------------------------------------------------
                // 步骤 3: 带重试机制的文件删除
                // -----------------------------------------------------------
                // 即使 wait() 返回了，Windows 有时也需要几十毫秒来释放文件句柄
                let max_retries = 5;
                let mut deleted = false;
                
                for i in 1..=max_retries {
                    if let Err(e) = std::fs::remove_file(&temp_binary_path) {
                        eprintln!("Cleanup attempt {}/{} failed: {}", i, max_retries, e);
                        // 如果删除失败，等待一小会儿再试
                        thread::sleep(Duration::from_millis(200));
                    } else {
                        println!("Successfully cleaned up temporary binary file.");
                        deleted = true;
                        break;
                    }
                }

                if !deleted {
                    eprintln!("WARNING: Failed to delete temp file after multiple attempts. System may clean it up later.");
                }

                println!("Web server terminated. Exiting...");
                *control_flow = ControlFlow::Exit;
            }
            _ => (),
        }
    });
}