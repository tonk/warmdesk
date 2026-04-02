// Prevents an extra console window from opening on Windows in release mode.
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

fn main() {
    warmdesk_lib::run()
}
