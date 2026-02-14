use std::process::Command;
use tao::event_loop::{ControlFlow, EventLoop};
use tao::window::WindowBuilder;
use wry::WebViewBuilder;

#[cfg(target_os = "windows")]
use std::os::windows::process::CommandExt;

fn main() -> wry::Result<()> {
    // Start the Go web server
    // Try multiple possible paths for music-dl binary
    let music_dl_paths = [
        "./music-dl",           // Same directory (packaged)
        "./music-dl.exe",       // Windows exe in same directory
        "../music-dl",          // Parent directory (development)
        "../music-dl.exe",      // Windows exe in parent directory
        "../../music-dl",       // Grandparent directory
        "../../music-dl.exe",   // Windows exe in grandparent directory
        "../../../music-dl",    // Great-grandparent directory
        "../../../music-dl.exe", // Windows exe in great-grandparent directory
        "music-dl",             // In PATH
    ];

    let mut child = None;
    for path in &music_dl_paths {
        let mut cmd = Command::new(path);
        cmd.arg("web")
           .arg("--no-browser")  // Don't auto-open browser
           .arg("-p").arg("37777");  // Use uncommon port

        // On Windows, hide the console window
        #[cfg(target_os = "windows")]
        {
            cmd.creation_flags(0x08000000); // CREATE_NO_WINDOW
        }

        if let Ok(process) = cmd.spawn() {
            child = Some(process);
            break;
        }
    }

    let mut child = child.expect("Failed to start Go web server - music-dl binary not found in any expected location");

    // Give the server a moment to start
    std::thread::sleep(std::time::Duration::from_secs(2));

    // Load icon - embedded at compile time
    const ICON_DATA: &[u8] = include_bytes!("../icon.png");
    let icon = match image::load_from_memory(ICON_DATA) {
        Ok(img) => {
            let icon_rgba = img.to_rgba8();
            let (width, height) = icon_rgba.dimensions();
            Some(tao::window::Icon::from_rgba(icon_rgba.into_raw(), width, height).unwrap())
        }
        Err(_) => None,
    };

    let event_loop = EventLoop::new();
    let window = WindowBuilder::new()
        .with_title("Music DL Desktop")
        .with_inner_size(tao::dpi::LogicalSize::new(1280.0, 800.0))
        .with_window_icon(icon)
        .build(&event_loop)
        .unwrap();

    let _webview = WebViewBuilder::new(&window)
        .with_url("http://localhost:37777/music")
        .build()?;

    event_loop.run(move |event, _, control_flow| {
        *control_flow = ControlFlow::Wait;

        match event {
            tao::event::Event::WindowEvent {
                event: tao::event::WindowEvent::CloseRequested,
                ..
            } => {
                // Kill the child process when closing
                println!("Terminating web server...");
                if let Err(e) = child.kill() {
                    eprintln!("Failed to kill child process: {}", e);
                    // Fallback: try to kill by process name on Windows
                    #[cfg(target_os = "windows")]
                    {
                        let _ = std::process::Command::new("taskkill")
                            .args(&["/F", "/IM", "music-dl.exe"])
                            .output();
                    }
                }
                // Wait a moment for the process to terminate
                std::thread::sleep(std::time::Duration::from_millis(500));
                println!("Web server terminated. Exiting...");
                *control_flow = ControlFlow::Exit;
            }
            _ => (),
        }
    });
}
