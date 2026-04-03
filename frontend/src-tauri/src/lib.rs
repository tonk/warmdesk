#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let args: Vec<String> = std::env::args().collect();

    if args.iter().any(|a| a == "--version" || a == "-V") {
        println!("WarmDesk {}", env!("CARGO_PKG_VERSION"));
        std::process::exit(0);
    }

    let maximized = args.iter().any(|a| a == "--maximized");

    // On Linux, WebKitGTK's DMA-BUF renderer silently fails on many GPU
    // configurations (integrated GPUs, NVIDIA, VMs, some Wayland compositors),
    // producing a completely blank window.  Disabling it forces the fallback
    // compositing path, which works reliably across all configurations.
    // The env var check lets users override the behaviour if they prefer.
    #[cfg(target_os = "linux")]
    if std::env::var("WEBKIT_DISABLE_DMABUF_RENDERER").is_err() {
        // SAFETY: single-threaded at this point, before the Tauri runtime starts.
        unsafe { std::env::set_var("WEBKIT_DISABLE_DMABUF_RENDERER", "1") };
    }

    tauri::Builder::default()
        .plugin(tauri_plugin_http::init())
        .setup(move |app| {
            if maximized {
                if let Some(win) = tauri::Manager::get_webview_window(app, "main") {
                    win.maximize()?;
                }
            }
            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running WarmDesk");
}
