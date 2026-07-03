import * as duplex from "@progrium/duplex";
import { setupDevtools } from "../api/devtools.js";
import { WanixHandle } from "../api/handle.js";
import { WanixElement } from "./base.js";

// text loaders set up by esbuild
import wasmExecGo from "../wasm/wasm_exec.go.js";
import wasmExecTinygo from "../wasm/wasm_exec.tinygo.js";

let instanceID = 0;

const DEFAULT_WASM = new URL('./wanix.wasm', import.meta.url).href;

export class SystemElement extends WanixElement {
    constructor() {
        super();
        instanceID++;
        this.instanceID = instanceID;
        this.isReady = false;
        this.debug = false;

        this._ready = new Promise(resolve => this._wasmReady = resolve);
        this._ready.then(async () => {
            await this._setupNamespace("1", "", this.querySelectorAll(':scope > wanix-bind'));
            this.isReady = true;
            if (this.debug) {
                setupDevtools(this);
            }
            this.dispatchEvent(new CustomEvent("ready", {
                bubbles: true
            }));
        });
        this._portWrap = (port) => new duplex.PortConn(port);
        this._root = null;
    }

    _setupNamespace(tid="", baseFS="", bindings=[]) {
        // replaced by wasm
        throw new Error("wasm not ready");
    }

    _openPort(tid="") {
        // replaced by wasm
        throw new Error("wasm not ready");
    }

    _open9P(tid="") {
        // replaced by wasm
        throw new Error("wasm not ready");
    }

    // no tid means the root task
    openHandle(tid) {
        return new WanixHandle(this._openPort(tid));
    }

    get stdin() {
        this.root.openWritable("#wanix/stdin/data");
    }

    get root() {
        if (!this._root) {
            this._root = this.openHandle();
        }
        return this._root;
    }

    get wasm() {
        if (this.hasAttribute('wasm')) {
            return new URL(this.getAttribute('wasm'), document.baseURI).href;
        } else {
            if (this.debug) {
                return DEFAULT_WASM.replace(".wasm", ".debug.wasm");
            }
            return DEFAULT_WASM;
        }
    }

    async load(buffer) {
        const wasmBytes = new Uint8Array(buffer);
        const wasmString = new TextDecoder('utf-8', { ignoreBOM: true, fatal: false }).decode(wasmBytes);

        const execScript = document.createElement('script');
        if (wasmString.includes("asyncify_start_unwind")) {
            if (this.debug) console.log("TinyGo WASM detected");
            execScript.textContent = wasmExecTinygo;
        } else {
            if (this.debug) console.log("Go WASM detected");
            execScript.textContent = wasmExecGo;
        }
        // executes synchronously
        document.head.appendChild(execScript);

        const go = new window.Go();
        go.importObject["wanix"] = {
            getInstanceID: () => {
                return this.instanceID;
            }
        };
        WebAssembly.instantiate(wasmBytes, go.importObject).then(obj => {
            go.run(obj.instance);
        });
    }

    disconnectedCallback() {
        delete window.__wanix[this.instanceID];
    }

    connectedCallback() {
        super.connectedCallback();

        this.style.display = "flex";
        this.style.flexDirection = "column";
        this.style.height = "100%";
        this.style.minHeight = "0";

        if (!window.__wanix) {
            window.__wanix = {};
        }
        window.__wanix[this.instanceID] = this;

        this.debug = this.hasAttribute('debug');

        this.allowOrigins = (this.getAttribute('allow-origins') || "").split(" ");
        if (this.allowOrigins.length > 0 && this.id) {
            if (this.debug) {
                console.debug("exporting", this.id, "for", this.allowOrigins);
            }
            window.addEventListener("message", async (event) => {
                if (event.data.request != "wanix-import") return;
                if (location.hash.slice(1) != this.id) return;
                if (!this.allowOrigins.includes(event.origin) && !this.allowOrigins.includes("*")) return;
                if (this.debug) {
                    console.debug("import requested for", this.id, "from", event.origin);
                }
                await this._ready;
           
                const p9port = await this._open9P("1");
                event.data.responder.postMessage(p9port, [p9port]);
            });
        }

        fetch(this.wasm)
            .then(r => r.arrayBuffer())
            .then(this.load.bind(this))
            .catch(err => {
                console.error("Failed to load Wanix WASM", err);
                this.dispatchEvent(new CustomEvent("error", {
                    detail: { error: err },
                    bubbles: true
                }));
            });
    }

}

if (typeof window !== "undefined") {
    customElements.define("wanix-system", SystemElement);
}
