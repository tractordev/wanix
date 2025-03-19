const std = @import("std");

pub fn main() !void {
    // Get allocator for temporary allocations
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

    // Get current working directory
    // const cwd = try std.fs.cwd().realpathAlloc(allocator, ".");
    // defer allocator.free(cwd);

    // Setup stdout
    const stdout = std.io.getStdOut().writer();

    // Print current directory
    // try stdout.print("Dir: {s}\n", .{cwd});
    try stdout.print("Dir: n/a\n", .{});

    // Print arguments
    try stdout.print("Args:", .{});
    const args = try std.process.argsAlloc(allocator);
    defer std.process.argsFree(allocator, args);
    for (args) |arg| {
        try stdout.print(" {s}", .{arg});
    }
    try stdout.print("\n", .{});

    // Print environment variables
    try stdout.print("Env:\n", .{});
    var env_map = try std.process.getEnvMap(allocator);
    defer env_map.deinit();
    var env_it = env_map.iterator();
    while (env_it.next()) |entry| {
        try stdout.print(" {s}={s}\n", .{ entry.key_ptr.*, entry.value_ptr.* });
    }
    try stdout.print("\n", .{});

    // Print root directory contents
    try stdout.print("Root:", .{});
    var root_dir = try std.fs.openDirAbsolute("/", .{ .iterate = true });
    defer root_dir.close();
    var root_it = root_dir.iterate();
    while (try root_it.next()) |entry| {
        if (entry.kind == .directory) {
            try stdout.print(" {s}/", .{entry.name});
        } else {
            try stdout.print(" {s}", .{entry.name});
        }
    }
    try stdout.print("\n", .{});
} 