import * as duplex from "@progrium/duplex";
import { setupDevtools } from "../api/devtools.js";
import { WanixHandle } from "../api/handle.js";

// text loaders set up by esbuild
import wasmExecGo from "../wasm/wasm_exec.go.js";
import wasmExecTinygo from "../wasm/wasm_exec.tinygo.js";

let instanceID = 0;

const DEFAULT_WASM = new URL('./wanix.wasm', import.meta.url).href;

export class System extends HTMLElement {
    constructor() {
        super();
        instanceID++;
        this.instanceID = instanceID;
        this.isReady = false;
        this.debug = this.hasAttribute('debug');
        this.bindings = new Promise(resolve => {
            this.__resolveBindings = resolve;
        });
        this.__parsedBindings = false;
        this.__readyPromise = new Promise(resolve => this.__wasmReady = resolve);
        this.__readyPromise.then(() => {
            this.isReady = true;
            this.dispatchEvent(new CustomEvent("ready", {
                bubbles: true
            }));
            if (this.debug) {
                setupDevtools(this);
            }
        });
        this.__portWrap = (port) => new duplex.PortConn(port);
        this.__root = null;
    }

    openPort() {
        // replaced by wasm
        throw new Error("wasm not ready");
    }

    openHandle() {
        return new WanixHandle(this.openPort());
    }

    get root() {
        if (!this.__root) {
            this.__root = this.openHandle();
        }
        return this.__root;
    }

    get wasm() {
        if (this.hasAttribute('wasm')) {
            return new URL(this.getAttribute('wasm'), document.baseURI).href;
        } else {
            return DEFAULT_WASM;
        }
    }

    async load(buffer) {
        const wasmBytes = new Uint8Array(buffer);
        const wasmString = new TextDecoder('utf-8', { ignoreBOM: true, fatal: false }).decode(wasmBytes);

        const execScript = document.createElement('script');
        if (wasmString.includes("tinygo_launch")) {
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
        if (!window.__wanix) {
            window.__wanix = {};
        }
        window.__wanix[this.instanceID] = this;

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

        if (this.__parsedBindings) {
            return;
        }
        const parseBindings = () => {
            const bindings = [...this.querySelectorAll(':scope > wanix-bind')].map(el => ({
                dst: el.getAttribute('dst'),
                src: el.getAttribute('src'),
                mode: el.getAttribute('mode') || "after",
                type: el.getAttribute('type') || "self",
            }));
            bindings.forEach(binding => {
                if (binding.type === "archive") {
                    binding.archive = fetchArchive(binding.src);
                }
            });
            this.__parsedBindings = true;
            this.__resolveBindings(bindings);
        }
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', parseBindings, { once: true });
        } else {
            parseBindings();
        }
    }

}

if (typeof window !== "undefined") {
    customElements.define("wanix-system", System);
}

// fetch an archive and return a tar stream, decompressing if necessary
async function fetchArchive(url) {
    const res = await fetch(url);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);

    const reader = res.body.getReader();

    // Accumulate until we have enough bytes to classify (tar needs 262+)
    const prefixChunks = [];
    let prefixLen = 0;
    const NEEDED = 512; // one tar block

    while (prefixLen < NEEDED) {
        const { value, done } = await reader.read();
        if (done) break;
        prefixChunks.push(value);
        prefixLen += value.byteLength;
    }

    // Flatten prefix
    const prefix = new Uint8Array(prefixLen);
    let off = 0;
    for (const c of prefixChunks) { prefix.set(c, off); off += c.byteLength; }

    // Detect gzip magic number
    const isGzip = prefix[0] === 0x1f && prefix[1] === 0x8b;

    // Rebuild a stream: emit the prefix, then pipe the rest of the reader
    const baseBody = new ReadableStream({
        start(controller) {
            controller.enqueue(prefix);
        },
        async pull(controller) {
            const { value, done } = await reader.read();
            if (done) controller.close();
            else controller.enqueue(value);
        },
        cancel(reason) { reader.cancel(reason); }
    });

    if (!isGzip) {
        // if not gzip, just return the raw stream (tar or otherwise)
        return baseBody;
    } else {
        // if gzip, decompress and return the decompressed stream (assumed tar)
        // Use DecompressionStream if available
        if (typeof DecompressionStream === "undefined") {
            throw new Error("Gzip archives require DecompressionStream support in this browser");
        }
        return baseBody.pipeThrough(new DecompressionStream("gzip"));
    }
}