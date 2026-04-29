const std = @import("std");

pub fn main(init: std.process.Init) !void {
    const io = init.io;

    var in_buf: [256]u8 = undefined;
    var in = std.Io.File.stdin().reader(io, &in_buf);
    const stdin: *std.Io.Reader = &in.interface;

    var out_buf: [256]u8 = undefined;
    var out = std.Io.File.stdout().writer(io, &out_buf);
    const stdout: *std.Io.Writer = &out.interface;

    while (true) {
        try stdout.writeAll("> ");
        try stdout.flush();

        const line = stdin.takeDelimiterExclusive('\n') catch |err| switch (err) {
            error.EndOfStream => return,
            else => return err,
        };
        stdin.toss(1); // consume the '\n'

        const cmd = std.mem.trim(u8, line, " \t\r");

        if (std.mem.eql(u8, cmd, "hello")) {
            try stdout.writeAll("hi there from zig!\n");
        } else if (std.mem.eql(u8, cmd, "ping")) {
            try stdout.writeAll("pong\n");
        } else if (std.mem.eql(u8, cmd, "exit") or std.mem.eql(u8, cmd, "quit")) {
            try stdout.writeAll("bye\n");
            try stdout.flush();
            return;
        } else if (cmd.len > 0) {
            try stdout.writeAll("unknown command\n");
        }
    }
}