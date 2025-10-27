
import * as duplex from "@progrium/duplex";
import { setupConsoleHelpers } from "./helpers.js";
import { setupWio } from "./wio.js";
import { WanixHandle } from "./handle.js";

import untar from "js-untar";

// text loaders set up by esbuild
import wasmExecGo from "./wasm/wasm_exec.go.js";
import wasmExecTinygo from "./wasm/wasm_exec.tinygo.js";

export class WanixRuntime extends WanixHandle {
    constructor(config = {}) {
        const sys = new MessageChannel();
        super(sys.port2);
        this.config = config;
        this._id = Math.random().toString(36).substring(2, 15);
        this._sys = sys;
        this._ready = new Promise(resolve => this._wasmReady = resolve);

        if (!window.__wanix) {
            window.__wanix = { pending: [] };
        }
        window.__wanix.pending.push(this);
        window.__wanix[this._id] = this;

        if (!config.wasm) {
            config.wasm = "./wanix.wasm";
        }

        if (config.helpers) {
            setupConsoleHelpers(this._id);
        }

        if (config.bundle) {
            this._loadBundle(config.bundle);
        } else {
            fetch(config.wasm).then(r => r.arrayBuffer()).then(this._loadWasm);
        }


        // window.wanix = {
        //     config,
        //     instance: this,
        //     sys: new duplex.PortConn(sys.port2),
        //     // sw: new MessageChannel(),

        //     // kludge: for worker
        //     _toport: (port) => new duplex.PortConn(port), 
        // };
    }

    ready() {
        return this._ready;
    }

    createPort() {
        // replaced by wasm
    }

    _portConn(port) {
        return new duplex.PortConn(port);
    }

    async _loadBundle(bundle) {
        console.log("loading bundle", bundle);
        const response = await fetch(bundle);
        const stream = response.body.pipeThrough(new DecompressionStream('gzip'));

        this._bundle = await new Response(stream).arrayBuffer();

        let foundWasm = false;
        await untar(this._bundle.slice()).progress((f) => {
            const fname = f.name.replace(/^\.\//, "");
            // console.log("bundle file:", fname);
            if (fname === "wanix.wasm") {
                foundWasm = true;
                console.log("loading wasm from bundle");
                this._loadWasm(f.buffer);

            }
            // else if (fname === "v86/libv86.js") {
            //     console.log("loading libv86.js from bundle");
            //     const v86Script = document.createElement('script');
            //     const blob = new Blob([f.buffer], { type: 'text/javascript' });
            //     v86Script.src = URL.createObjectURL(blob);
            //     document.head.appendChild(v86Script);

            // } else if (fname === "v86/v86.wasm") {
            //     console.log("loading v86.wasm from bundle");
            //     const blob = new Blob([f.buffer], { type: 'application/wasm' });
            //     this._v86wasm = URL.createObjectURL(blob);

            // } else if (fname === "v86/seabios.bin") {
            //     console.log("loading seabios.bin from bundle");
            //     this._v86seabios = f.buffer;
            //     console.log(this._v86seabios)

            // } else if (fname === "v86/vgabios.bin") {
            //     console.log("loading vgabios.bin from bundle");
            //     this._v86vgabios = f.buffer;

            // } else if (fname === "kernel/bzImage") {
            //     console.log("loading bzImage from bundle");
            //     this._bzImage = f.buffer;
            //     console.log(this._bzImage)
            // }
        })
        if (!foundWasm) {
            fetch(this.config.wasm).then(r => r.arrayBuffer()).then(this._loadWasm);
        }
    }

    async _loadWasm(buffer) {
        const wasmBytes = new Uint8Array(buffer);
        const wasmString = new TextDecoder('utf-8', { ignoreBOM: true, fatal: false }).decode(wasmBytes);

        const execScript = document.createElement('script');
        if (wasmString.includes("tinygo_launch")) {
            console.log("TinyGo WASM detected");
            execScript.textContent = wasmExecTinygo;
        } else {
            console.log("Go WASM detected");
            execScript.textContent = wasmExecGo;
        }
        document.head.appendChild(execScript);

        const go = new window.Go();
        WebAssembly.instantiate(wasmBytes, go.importObject).then(obj => {
            go.run(obj.instance);
        });
    }
}

if (globalThis.window) {
    window.WanixRuntime = WanixRuntime;
    window.WanixHandle = WanixHandle;
}

