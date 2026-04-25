import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";

export class TerminalElement extends HTMLElement {
    static get observedAttributes() {
        return ["for", "path"];
    }

    constructor() {
        super();
        this.path = this.getAttribute('path');
        this.term = null;
        this.fitAddon = null;
        this._resizeObserver = null;
        this._system = null;
        this._reader = null;
        this._writer = null;
        this._dataDisposable = null;
        this._connected = false;
    }

    connectedCallback() {
        this.term = new Terminal({
            fontFamily: `ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace`,
            // theme: {
            //     background: "rgba(0, 0, 0, 0)",
            //     foreground: "white",
            // },
            ...this._getOptionsFromAttributes()
        });

        if (FitAddon) {
            this.fitAddon = new FitAddon();
            this.term.loadAddon(this.fitAddon);
        }

        this.term.open(this);

        if (this.fitAddon) {
            this.fitAddon.fit();
            this._resizeObserver = new ResizeObserver(() => {
                this.fitAddon.fit();
            });
            this._resizeObserver.observe(this);
            // this._resizeObserver.observe(this.parentElement);
        }

        this._connected = true;
        this._resolveSystemRef();
        this._connectToSource();

        this.dispatchEvent(new CustomEvent("ready", { detail: { term: this.term } }));

        this.style.flex = "1";
        this.style.display = "flex";
        this.style.flexDirection = "column";
        this.style.height = "100%";
    }
    

    disconnectedCallback() {
        this._connected = false;
        this._disconnectFromSource();
        if (this._resizeObserver) {
            this._resizeObserver.disconnect();
            this._resizeObserver = null;
        }
        if (this.term) {
            this.term.dispose();
            this.term = null;
        }
        this.fitAddon = null;
        this._system = null;
    }

    attributeChangedCallback(name, oldValue, newValue) {
        if (oldValue === newValue) return;

        if (name === "for") {
            this._resolveSystemRef();
            this._connectToSource();
        } else if (name === "path") {
            this._connectToSource();
        }
    }

    _resolveSystemRef() {
        const forId = this.getAttribute("for");
        if (forId) {
            this._system = document.getElementById(forId);
        } else {
            this._system = this.closest('wanix-system') || document.querySelector('wanix-system');
        }
        this._ready = new Promise(resolve => this._system.addEventListener('ready', resolve))
    }

    async _connectToSource() {
        if (!this._connected || !this.term) return;

        const dataPath = this.path + "/data";
        if (!dataPath || !this._system) return;

        this._disconnectFromSource();

        try {
            await this._ready;
            await new Promise(resolve => setTimeout(resolve, 1000));
       

            const readable = await this._system.root.openReadable(dataPath);
            this._reader = readable.getReader();
            this._readLoop();

            const writable = await this._system.root.openWritable(dataPath);
            this._writer = writable.getWriter();

            const encoder = new TextEncoder();
            let buffer = '';
            this._dataDisposable = this.term.onData((data) => {
                // may add line discipline as mode to terminals but for now we
                // do as plan 9 and handle it here in "userspace"
                if (data === '\r') {
                    this.term.write('\r\n');           // echo newline
                    if (this._writer) {
                        this._writer.write(encoder.encode(buffer+"\n"));
                    }
                    buffer = '';
                } else if (data === '\x7f') {   // backspace
                    if (buffer.length > 0) {
                      buffer = buffer.slice(0, -1);
                      this.term.write('\b \b');
                    }
                } else {
                    buffer += data;
                    this.term.write(data);             // echo
                }
            });
        } catch (err) {
            console.error("wanix-terminal: failed to connect to terminal:", err);
        }
    }

    async _readLoop() {
        if (!this._reader || !this.term) return;

        try {
            while (true) {
                const { done, value } = await this._reader.read();
                if (done) break;
                if (value && this.term) {
                    this.term.write(value);
                }
            }
        } catch (err) {
            if (this._connected) {
                console.error("wanix-terminal: read error:", err);
            }
        }
    }

    _disconnectFromSource() {
        if (this._dataDisposable) {
            this._dataDisposable.dispose();
            this._dataDisposable = null;
        }
        if (this._reader) {
            this._reader.cancel().catch(() => {});
            this._reader = null;
        }
        if (this._writer) {
            this._writer.close().catch(() => {});
            this._writer = null;
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
        if (this.fitAddon) {
            this.fitAddon.fit();
        }
    }

    write(data) {
        if (this.term) {
            this.term.write(data);
        }
    }

    onData(callback) {
        if (this.term) {
            return this.term.onData(callback);
        }
        return null;
    }

    reset() {
        if (this.term) {
            this.term.reset();
        }
    }

    focus() {
        if (this.term) {
            this.term.focus();
        }
    }

    clear() {
        if (this.term) {
            this.term.clear();
        }
    }
}

if (typeof window !== "undefined") {
    customElements.define("wanix-term", TerminalElement);
}
