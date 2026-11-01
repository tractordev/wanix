import test from "node:test";
import assert from "node:assert/strict";

import { TAB_WIDTH, editInput } from "./linedisc.js";

test("plain characters are echoed and buffered", () => {
    const r = editInput("", "a", 0);
    assert.deepEqual(r, { buffer: "a", echo: "a", submit: null });
});

test("ctrl-c resets the buffer and submits", () => {
    const r = editInput("abc", "\x03", 3);
    assert.deepEqual(r, { buffer: "", echo: "^C\r\n", submit: "\x03\n" });
});

test("backspace on empty buffer is a no-op", () => {
    const r = editInput("", "\x7f", 0);
    assert.deepEqual(r, { buffer: "", echo: "", submit: null });
});

test("tab at column 9 emits 7 spaces", () => {
    const r = editInput("", "\t", 9);
    assert.deepEqual(r, { buffer: " ".repeat(7), echo: " ".repeat(7), submit: null });
});

test("tab at column 0 emits 8 spaces", () => {
    const r = editInput("", "\t", 0);
    assert.deepEqual(r, { buffer: " ".repeat(8), echo: " ".repeat(8), submit: null });
    assert.equal(TAB_WIDTH, 8);
});

test("backspace removes one tab space per press", () => {
    const tabbed = editInput("cat h", "\t", 5);
    assert.equal(tabbed.buffer, "cat h" + " ".repeat(3));

    let buffer = tabbed.buffer;
    for (let i = 0; i < 3; i++) {
        const r = editInput(buffer, "\x7f", 0);
        assert.equal(r.echo, "\b \b");
        buffer = r.buffer;
        assert.equal(buffer, "cat h" + " ".repeat(2 - i));
    }
    assert.equal(buffer, "cat h");
});

test("enter submits the expanded line", () => {
    const tabbed = editInput("cat h", "\t", 5);
    const r = editInput(tabbed.buffer, "\r", tabbed.echo.length);
    assert.deepEqual(r, { buffer: "", echo: "\r\n", submit: "cat h" + " ".repeat(3) + "\n" });
});

test("echo width matches buffer growth", () => {
    for (let col = 0; col < 16; col++) {
        const r = editInput("", "\t", col);
        assert.equal(r.echo.length, r.buffer.length);
    }
    const r = editInput("", "x", 0);
    assert.equal(r.echo.length, 1);
});
