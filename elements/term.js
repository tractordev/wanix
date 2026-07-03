import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import xtermCss from "@xterm/xterm/css/xterm.css";
import { WanixElement } from "./base.js";

if (typeof document !== "undefined" && !document.getElementById("wanix-xterm-css")) {
    const style = document.createElement("style");
    style.id = "wanix-xterm-css";
    style.textContent = xtermCss;
    document.head.appendChild(style);
}

export class TerminalElement extends WanixElement {
    #resizeObserver;
    #reader;
    #writer;
    #dataDisposable;

    constructor() {
        super();
        this.rid = null; // not used yet, see path

        this._term = null;
        this._fitAddon = null;
        this.#resizeObserver = null;
        this.#reader = null;
        this.#writer = null;
        this.#dataDisposable = null;
    }

    connectedCallback() {
        super.connectedCallback();

        // this should be optional and cause
        // allocation if no path attribute
        this.path = this.getAttribute('path');

        this.raw = this.hasAttribute('raw');

        this._term = new Terminal({
            fontFamily: `ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace`,
            // theme: {
            //     background: "rgba(0, 0, 0, 0)",
            //     foreground: "white",
            // },
            ...this._getOptionsFromAttributes()
        });

        this._fitAddon = new FitAddon();
        this._term.loadAddon(this._fitAddon);
        this._term.open(this);

        this._fitAddon.fit();
        this.#resizeObserver = new ResizeObserver(() => {
            this._fitAddon.fit();
        });
        this.#resizeObserver.observe(this);
        // this._resizeObserver.observe(this.parentElement);

        this.style.flex = "1";
        this.style.display = "flex";
        this.style.flexDirection = "column";
        this.style.height = "100%";
    }
    

    disconnectedCallback() {
        super.disconnectedCallback();

        this.disconnect();
        if (this.#resizeObserver) {
            this.#resizeObserver.disconnect();
            this.#resizeObserver = null;
        }
        if (this._term) {
            this._term.dispose();
            this._term = null;
        }
        this._fitAddon = null;
    }

    async _awake() {
        await this._resolvePath();
        await this.connect();
    }

    async _resolvePath() {
        if (this.path) return;

        // Parent task/vm with a terminal device.
        const parent = this.parentElement;
        if (parent?.tagName === "WANIX-TASK" || parent?.tagName === "WANIX-VM") {
            await parent._nsReady;
            if (parent.term) {
                this.path = parent.term;
                return;
            }
        }

        // Child task/vm declarations under this term (standalone term host).
        const children = [
            ...this.querySelectorAll(":scope > wanix-task"),
            ...this.querySelectorAll(":scope > wanix-vm"),
        ];
        if (children.length) {
            await Promise.all(children.map((c) => c._nsReady));
            const withTerm = children.find((c) => c.term);
            if (withTerm) {
                this.path = withTerm.term;
                return;
            }
        }

        // for= host: pick a task/vm with a term inside the referenced host.
        const host = this._kernelHost;
        if (host && host !== this) {
            const peers = [
                ...host.querySelectorAll(":scope > wanix-task"),
                ...host.querySelectorAll(":scope > wanix-vm"),
            ];
            await Promise.all(peers.map((c) => c._nsReady));
            const withTerm = peers.find((c) => c.term);
            if (withTerm) {
                this.path = withTerm.term;
            }
        }
    }

    async connect() {
        if (!this._term) return;

        const dataPath = this.path + "/data";
        if (!this.path || !this._kernel) return;

        this.disconnect();

        try {
            await this._kernel.root.waitFor(dataPath, 30000);

            // todo: use this for kvm updates
            this._kernel._updateTerminals?.(this);

            const readable = await this._kernel.root.openReadable(dataPath);
            this.#reader = readable.getReader();
            this._readLoop();

            const writable = await this._kernel.root.openWritable(dataPath);
            this.#writer = writable.getWriter();

            const encoder = new TextEncoder();
            let buffer = '';
            this.#dataDisposable = this._term.onData((data) => {
                if (this.raw) {
                    this.#writer.write(encoder.encode(data));
                    return;
                }
                // may add line discipline as mode to terminals but for now we
                // do as plan 9 and handle it here in "userspace"
                if (data === '\r') {
                    this._term.write('\r\n');           // echo newline
                    if (this.#writer) {
                        this.#writer.write(encoder.encode(buffer+"\n"));
                    }
                    buffer = '';
                } else if (data === '\x7f') {   // backspace
                    if (buffer.length > 0) {
                      buffer = buffer.slice(0, -1);
                      this._term.write('\b \b');
                    }
                } else {
                    buffer += data;
                    this._term.write(data);             // echo
                }
            });
        } catch (err) {
            console.error("wanix-term: failed to connect terminal:", err);
        }
    }

    async _readLoop() {
        if (!this.#reader || !this._term) return;

        try {
            while (true) {
                const { done, value } = await this.#reader.read();
                if (done) break;
                if (value && this._term) {
                    this._term.write(value);
                }
            }
        } catch (err) {
            console.error("wanix-terminal: read error:", err);
        }
    }

    disconnect() {
        if (this.#dataDisposable) {
            this.#dataDisposable.dispose();
            this.#dataDisposable = null;
        }
        if (this.#reader) {
            this.#reader.cancel().catch(() => {});
            this.#reader = null;
        }
        if (this.#writer) {
            this.#writer.close().catch(() => {});
            this.#writer = null;
        }
    }

    _getOptionsFromAttributes() {
        const options = {};
        
        if (this.hasAttribute("font-size")) {
            options.fontSize = parseInt(this.getAttribute("font-size"), 10);
        }
        if (this.hasAttribute("font-family")) {
            options.fontFamily = this.getAttribute("font-family");
        }
        if (this.hasAttribute("cursor-blink")) {
            options.cursorBlink = this.getAttribute("cursor-blink") !== "false";
        }
        if (this.hasAttribute("cursor-style")) {
            options.cursorStyle = this.getAttribute("cursor-style");
        }
        if (this.hasAttribute("scrollback")) {
            options.scrollback = parseInt(this.getAttribute("scrollback"), 10);
        }

        return options;
    }

    fit() {
        if (this._fitAddon) {
            this._fitAddon.fit();
        }
    }

    write(data) {
        if (this._term) {
            this._term.write(data);
        }
    }

    reset() {
        if (this._term) {
            this._term.reset();
        }
    }

    focus() {
        if (this._term) {
            this._term.focus();
        }
    }

    clear() {
        if (this._term) {
            this._term.clear();
        }
    }
}

if (typeof window !== "undefined") {
    customElements.define("wanix-term", TerminalElement);
}
