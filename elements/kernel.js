import * as duplex from "@progrium/duplex";
import { setupDevtools } from "../api/devtools.js";
import { WanixHandle } from "../api/handle.js";

// text loaders set up by esbuild
import wasmExecGo from "../wasm/wasm_exec.go.js";
import wasmExecTinygo from "../wasm/wasm_exec.tinygo.js";

let instanceID = 0;

const DEFAULT_WASM = new URL("./wanix.wasm", import.meta.url).href;

/**
 * WanixKernel owns the Wasm runtime and root namespace.
 * Registered in window.__wanix so Wasm can attach _setupNamespace, _openPort, etc.
 */
export class WanixKernel {
    constructor(host) {
        instanceID++;
        this.host = host;
        this.instanceID = instanceID;
        this.isReady = false;
        this.debug = host.hasAttribute("debug");

        this._ready = new Promise((resolve) => (this._wasmReady = resolve));
        this._portWrap = (port) => new duplex.PortConn(port);
        this._root = null;
    }

    _setupNamespace(tid = "", baseFS = "", bindings = []) {
        // replaced by wasm
        throw new Error("wasm not ready");
    }

    _openPort(tid = "") {
        // replaced by wasm
        throw new Error("wasm not ready");
    }

    _open9P(tid = "") {
        // replaced by wasm
        throw new Error("wasm not ready");
    }

    // no tid means the root task
    openHandle(tid) {
        return new WanixHandle(this._openPort(tid));
    }

    get root() {
        if (!this._root) {
            this._root = this.openHandle();
        }
        return this._root;
    }

    get wasm() {
        const host = this.host;
        if (host.hasAttribute("wasm")) {
            return new URL(host.getAttribute("wasm"), document.baseURI).href;
        }
        if (this.debug) {
            return DEFAULT_WASM.replace(".wasm", ".debug.wasm");
        }
        return DEFAULT_WASM;
    }

    async load(buffer) {
        const wasmBytes = new Uint8Array(buffer);
        const wasmString = new TextDecoder("utf-8", { ignoreBOM: true, fatal: false }).decode(wasmBytes);

        const execScript = document.createElement("script");
        if (wasmString.includes("asyncify_start_unwind")) {
            if (this.debug) console.log("TinyGo WASM detected");
            execScript.textContent = wasmExecTinygo;
        } else {
            if (this.debug) console.log("Go WASM detected");
            execScript.textContent = wasmExecGo;
        }
        document.head.appendChild(execScript);

        const go = new window.Go();
        go.importObject["wanix"] = {
            getInstanceID: () => this.instanceID,
        };
        WebAssembly.instantiate(wasmBytes, go.importObject).then((obj) => {
            go.run(obj.instance);
        });
    }

    async start() {
        if (!window.__wanix) {
            window.__wanix = {};
        }
        window.__wanix[this.instanceID] = this;

        this.allowOrigins = (this.host.getAttribute("allow-origins") || "").split(" ").filter(Boolean);
        if (this.allowOrigins.length > 0 && this.host.id) {
            if (this.debug) {
                console.debug("exporting", this.host.id, "for", this.allowOrigins);
            }
            window.addEventListener("message", async (event) => {
                if (event.data.request != "wanix-import") return;
                if (location.hash.slice(1) != this.host.id) {
                    console.log("import rejected because no namespace provided");
                    return;
                }
                if (!this.allowOrigins.includes(event.origin) && !this.allowOrigins.includes("*")) {
                    console.log("import rejected because origin not allowed", event.origin, this.allowOrigins);
                    return;
                }
                if (this.debug) {
                    console.debug("import accepted for", this.host.id, "from", event.origin);
                }
                await this._ready;

                const p9port = await this._open9P("1");
                event.data.responder.postMessage(p9port, [p9port]);
            });
        }

        try {
            const buffer = await fetch(this.wasm).then((r) => {
                if (!r.ok) throw new Error(`Failed to fetch wasm: ${r.status}`);
                return r.arrayBuffer();
            });
            await this.load(buffer);
            await this._ready;
        } catch (err) {
            console.error("Failed to load Wanix WASM", err);
            this.host.dispatchEvent(
                new CustomEvent("error", {
                    detail: { error: err },
                    bubbles: true,
                }),
            );
            throw err;
        }
    }

    markReady() {
        this.isReady = true;
        if (this.debug) {
            setupDevtools(this);
        }
    }

    dispose() {
        delete window.__wanix?.[this.instanceID];
    }
}
