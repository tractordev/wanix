use std::env;
use std::fs;

fn main() {
    // Get current working directory
    let wd = env::current_dir().unwrap_or_default();
    println!("Dir: {}", wd.display());

    // Print arguments
    print!("Args:");
    for arg in env::args() {
        print!(" {}", arg);
    }
    println!();

    // Print environment variables
    println!("Env:");
    for (key, value) in env::vars() {
        println!(" {}={}", key, value);
    }
    println!();

    // Print root directory contents
    print!("Root:");
    if let Ok(entries) = fs::read_dir("/") {
        for entry in entries {
            if let Ok(entry) = entry {
                if let Ok(file_type) = entry.file_type() {
                    if let Ok(name) = entry.file_name().into_string() {
                        if file_type.is_dir() {
                            print!(" {}/", name);
                        } else {
                            print!(" {}", name);
                        }
                    }
                }
            }
        }
    }
    println!();
}