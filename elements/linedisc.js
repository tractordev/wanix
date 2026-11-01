// Userspace line discipline for <wanix-term>. The #term device stays
// byte-transparent, so echo, backspace and tab expansion happen here.
export const TAB_WIDTH = 8;

// editInput applies one input chunk to the line buffer and returns what to
// echo and what, if anything, to submit to the pty.
export function editInput(buffer, data, col) {
    if (data === '\x03') {          // Ctrl-C
        return { buffer: '', echo: '^C\r\n', submit: '\x03\n' };
    }
    if (data === '\r') {
        return { buffer: '', echo: '\r\n', submit: buffer + '\n' };
    }
    if (data === '\x7f') {          // backspace
        if (buffer.length === 0) {
            return { buffer, echo: '', submit: null };
        }
        return { buffer: buffer.slice(0, -1), echo: '\b \b', submit: null };
    }
    // shortcut: only a lone tab chunk expands; a paste containing \t keeps the
    // old 1-byte/1-cell mismatch, upgrade if pasted tabs are reported
    if (data === '\t') {
        const spaces = ' '.repeat(TAB_WIDTH - (col % TAB_WIDTH));
        return { buffer: buffer + spaces, echo: spaces, submit: null };
    }
    return { buffer: buffer + data, echo: data, submit: null };
}
