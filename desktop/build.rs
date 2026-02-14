fn main() {
    if cfg!(target_os = "windows") {
        let mut res = winres::WindowsResource::new();
        res.set_icon("icon.png");
        if let Err(e) = res.compile() {
            eprintln!("Failed to compile resources: {}", e);
        }
    }
}