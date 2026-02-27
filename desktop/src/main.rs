#![windows_subsystem = "windows"]

use std::process::Command;
use std::thread;
use std::time::Duration;
use tao::event_loop::{ControlFlow, EventLoop};
use tao::window::WindowBuilder;
// [关键修正] 引入 WebContext 修复 .with_data_directory 报错
use wry::{WebViewBuilder, WebContext};
// 用于在系统默认浏览器中打开外部链接
use open;

#[cfg(target_os = "windows")]
use std::os::windows::process::CommandExt;

// ==========================================
//                 常量配置区域
// ==========================================

mod window_config {
    pub const TITLE: &str = "Music DL Desktop";
    pub const WIDTH: f64 = 1280.0;
    pub const HEIGHT: f64 = 800.0;
}

mod server_config {
    pub const PORT: &str = "37777";
    pub const URL_PATH: &str = "/music/";
    pub const STARTUP_DELAY_MS: u64 = 2000;

    #[cfg(target_os = "windows")]
    pub const BINARY_NAME: &str = "music-dl.exe";
    #[cfg(not(target_os = "windows"))]
    pub const BINARY_NAME: &str = "music-dl";
}

mod system_config {
    #[cfg(target_os = "windows")]
    pub const CREATE_NO_WINDOW_FLAG: u32 = 0x08000000;
}

// ==========================================
//                 嵌入式二进制文件
// ==========================================
// 确保 music-dl.exe 在项目根目录（即 desktop 的上两级）
#[cfg(target_os = "windows")]
static MUSIC_DL_BINARY: &[u8] = include_bytes!("../../music-dl.exe");
#[cfg(not(target_os = "windows"))]
static MUSIC_DL_BINARY: &[u8] = include_bytes!("../../music-dl");

// ==========================================
//                 主程序逻辑
// ==========================================

fn main() -> wry::Result<()> {
    // 1. 将嵌入的Go二进制文件提取到临时文件
    let temp_dir = std::env::temp_dir();
    
    // 使用 Process ID 生成唯一文件名，防止多开或上次未正常退出导致的文件锁冲突
    let unique_name = format!("{}_{}", std::process::id(), server_config::BINARY_NAME);
    let temp_binary_path = temp_dir.join(unique_name);

    // 防御性删除
    if temp_binary_path.exists() {
        let _ = std::fs::remove_file(&temp_binary_path);
    }

    // 解压文件
    // println!("Extracting binary to: {:?}", temp_binary_path);
    std::fs::write(&temp_binary_path, MUSIC_DL_BINARY)
        .expect("Failed to write embedded binary to temp file");

    // Unix 系统赋予执行权限
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
    
    let mut cmd = Command::new(&path);
    cmd.arg("web")
       .arg("--no-browser")
       .arg("-p").arg(server_config::PORT);

    // Windows 专用：隐藏子进程控制台窗口
    #[cfg(target_os = "windows")]
    {
        cmd.creation_flags(system_config::CREATE_NO_WINDOW_FLAG);
    }

    let mut child = match cmd.spawn() {
        Ok(process) => process,
        Err(e) => {
            eprintln!("Failed to start backend: {}", e);
            let _ = std::fs::remove_file(&temp_binary_path);
            panic!("Failed to start Go backend server");
        }
    };

    // 等待服务启动
    thread::sleep(Duration::from_millis(server_config::STARTUP_DELAY_MS));

    // 3. 加载图标
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
    // ------------------------------------------------------------------
    // [关键修正] 使用 WebContext 管理数据目录，解决 API 报错并隔离缓存
    // ------------------------------------------------------------------
    let data_dir = std::env::temp_dir().join("go-music-dl-webview-data");
    let mut web_context = WebContext::new(Some(data_dir.clone()));

    let server_url = format!("http://localhost:{}{}", server_config::PORT, server_config::URL_PATH);

    // clone 出一个前缀供 handler 捕获（避免引用生命周期问题）
    let server_prefix = server_url.clone();

    let _webview = WebViewBuilder::new(&window)
        .with_url(&server_url)
        .with_web_context(&mut web_context) // 使用 Context 注入配置
        // 拦截 target="_blank" / window.open 请求，交给系统浏览器打开
        .with_new_window_req_handler(move |url| {
            // 打开外部链接，不在 WebView 中创建新窗口
            if let Err(e) = open::that(&url) {
                eprintln!("Failed to open external link: {} : {}", url, e);
            }
            // 返回 false 表示不要由 WebView 自行打开新窗口
            false
        })
        // 拦截顶级导航，只有当导航目标属于本地服务时允许在 WebView 内部跳转
        .with_navigation_handler(move |nav| {
            let url = nav.as_str();
            if !url.starts_with(&server_prefix) {
                if let Err(e) = open::that(url) {
                    eprintln!("Failed to open external nav: {} : {}", url, e);
                }
                return false; // 阻止 WebView 自己导航到外部站点
            }
            true
        })
        .build()?;

    // 6. 事件循环
    event_loop.run(move |event, _, control_flow| {
        *control_flow = ControlFlow::Wait;

        match event {
            tao::event::Event::WindowEvent {
                event: tao::event::WindowEvent::CloseRequested,
                ..
            } => {
                // -----------------------------------------------------------
                // [极速关闭优化] 1. 立即隐藏窗口，给用户“秒退”的视觉反馈
                // -----------------------------------------------------------
                window.set_visible(false);

                // 2. 终止子进程 (Kill)
                // Rust 的 kill() 在 Windows 上调用 TerminateProcess，足以强杀，无需 taskkill
                let _ = child.kill(); 

                // 3. 关键：等待进程完全释放文件锁
                match child.wait() {
                    Ok(_) => {}, 
                    Err(e) => eprintln!("Wait error: {}", e),
                }

                // 4. 删除临时 exe (快速重试)
                // 由于窗口已隐藏，这里稍慢一点用户也感觉不到
                let max_retries = 5;
                for _ in 1..=max_retries {
                    if std::fs::remove_file(&temp_binary_path).is_ok() {
                        break;
                    }
                    // 缩短等待间隔，加速退出流程
                    thread::sleep(Duration::from_millis(50));
                }

                // 5. 清理 WebView 缓存 (最耗时操作)
                if data_dir.exists() {
                    let _ = std::fs::remove_dir_all(&data_dir);
                }

                *control_flow = ControlFlow::Exit;
            }
            _ => (),
        }
    });
}