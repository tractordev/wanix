const std = @import("std");
const wasi = std.os.wasi;

// Write bytes to a file descriptor using the WASI fd_write syscall.
fn fdWrite(fd: wasi.fd_t, str: []const u8) void {
    const iov = [1]wasi.ciovec_t{.{ .base = str.ptr, .len = str.len }};
    var nwritten: usize = undefined;
    _ = wasi.fd_write(fd, &iov, 1, &nwritten);
}

// Write formatted text to stdout.
fn print(comptime fmt: []const u8, args: anytype) void {
    var buf: [4096]u8 = undefined;
    const s = std.fmt.bufPrint(&buf, fmt, args) catch return;
    fdWrite(1, s);
}

pub fn main() !void {
    var backing: [65536]u8 = undefined;
    var fba = std.heap.FixedBufferAllocator.init(&backing);
    const allocator = fba.allocator();

    // Print current directory (not available without Io object)
    fdWrite(1, "Dir: n/a\n");

    // Print arguments via WASI args_get
    fdWrite(1, "Args:");
    {
        var count: usize = undefined;
        var buf_size: usize = undefined;
        switch (wasi.args_sizes_get(&count, &buf_size)) {
            .SUCCESS => {},
            else => count = 0,
        }
        if (count > 0) {
            const buf = try allocator.alloc(u8, buf_size);
            defer allocator.free(buf);
            const ptrs = try allocator.alloc([*:0]u8, count);
            defer allocator.free(ptrs);
            switch (wasi.args_get(ptrs.ptr, buf.ptr)) {
                .SUCCESS => {},
                else => {},
            }
            for (ptrs) |ptr| {
                const arg = std.mem.sliceTo(ptr, 0);
                print(" {s}", .{arg});
            }
        }
    }
    fdWrite(1, "\n");

    // Print environment variables via WASI environ_get
    fdWrite(1, "Env:\n");
    {
        var count: usize = undefined;
        var buf_size: usize = undefined;
        switch (wasi.environ_sizes_get(&count, &buf_size)) {
            .SUCCESS => {},
            else => count = 0,
        }
        if (count > 0) {
            const buf = try allocator.alloc(u8, buf_size);
            defer allocator.free(buf);
            const ptrs = try allocator.alloc([*:0]u8, count);
            defer allocator.free(ptrs);
            switch (wasi.environ_get(ptrs.ptr, buf.ptr)) {
                .SUCCESS => {},
                else => {},
            }
            for (ptrs) |ptr| {
                const entry = std.mem.sliceTo(ptr, 0);
                print(" {s}\n", .{entry});
            }
        }
    }
    fdWrite(1, "\n");

    // Print root directory contents by reading fd 3 (first preopen, typically "/")
    fdWrite(1, "Root:");
    {
        // Try to list the root preopen directory using readdir
        var dirent_buf: [2048]u8 = undefined;
        var cookie: wasi.dircookie_t = wasi.DIRCOOKIE_START;
        while (true) {
            var nread: usize = undefined;
            const rc = wasi.fd_readdir(3, &dirent_buf, dirent_buf.len, cookie, &nread);
            if (rc != .SUCCESS or nread == 0) break;
            var offset: usize = 0;
            while (offset + @sizeOf(wasi.dirent_t) <= nread) {
                const dirent: *const wasi.dirent_t = @ptrCast(@alignCast(&dirent_buf[offset]));
                const name_start = offset + @sizeOf(wasi.dirent_t);
                const name_len = dirent.namlen;
                if (name_start + name_len > nread) break;
                const name = dirent_buf[name_start .. name_start + name_len];
                if (dirent.type == .DIRECTORY) {
                    print(" {s}/", .{name});
                } else {
                    print(" {s}", .{name});
                }
                cookie = dirent.next;
                offset = name_start + name_len;
            }
            if (nread < dirent_buf.len) break;
        }
    }
    fdWrite(1, "\n");
}
