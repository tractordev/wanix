
import * as duplex from "./duplex.min.js";
import { setupConsoleHelpers } from "./helpers.js";
import { setupWio } from "./wio.js";
import { WanixFS } from "./fs.js";

import untar from "js-untar";

// text loaders set up by esbuild
import wasmExecGo from "./wasm/wasm_exec.go.js";
import wasmExecTinygo from "./wasm/wasm_exec.tinygo.js";

export class Wanix extends WanixFS {
    constructor(config={}) {
        if (window.wanix) {
            throw new Error("Wanix already initialized on this page");
        }

        // todo: make optional
        setupWio(); 

        const sys = new MessageChannel();
        super(sys.port1);
        
        window.wanix = {
            config,
            instance: this,
            sys: new duplex.PortConn(sys.port2),
            // sw: new MessageChannel(),

            // kludge: for worker
            _toport: (port) => new duplex.PortConn(port), 
        };

        if (config.helpers) {
            setupConsoleHelpers();
        }

        if (config.bundle) {
            this.loadBundle(config.bundle);
        } else {
            fetch("./wanix.wasm").then(r => r.arrayBuffer()).then(this.loadWasm);
        }
        
    }

    async loadBundle(bundle) {
        console.log("loading bundle", bundle);
        const response = await fetch(bundle);
        const stream = response.body.pipeThrough(new DecompressionStream('gzip'));
        window.wanix.bundle = await new Response(stream).arrayBuffer();
        const toUntar = window.wanix.bundle.slice();
        let foundWasm = false;
        untar(toUntar).progress((f) => {
            // console.log("found", f.name);
            if (f.name === "./wanix.wasm") {
                foundWasm = true;
                console.log("loading wasm from bundle");
                this.loadWasm(f.buffer);
            } else if (f.name === "./v86/libv86.js") {
                console.log("loading libv86.js from bundle");
                const v86Script = document.createElement('script');
                const blob = new Blob([f.buffer], { type: 'text/javascript' });
                v86Script.src = URL.createObjectURL(blob);
                document.head.appendChild(v86Script);
            } else if (f.name === "./v86/v86.wasm") {
                console.log("loading v86.wasm from bundle");
                const blob = new Blob([f.buffer], { type: 'application/wasm' });
                window.wanix.v86wasm = URL.createObjectURL(blob);
            } else if (f.name === "./v86/seabios.bin") {
                console.log("loading seabios.bin from bundle");
                window.wanix.v86seabios = f.buffer;
            } else if (f.name === "./v86/vgabios.bin") {
                console.log("loading vgabios.bin from bundle");
                window.wanix.v86vgabios = f.buffer;
            } else if (f.name === "./kernel/bzImage") {
                console.log("loading bzImage from bundle");
                window.wanix.bzImage = f.buffer;
            }
        }).then(() => {
            if (!foundWasm) {
                fetch("./wanix.wasm").then(r => r.arrayBuffer()).then(this.loadWasm);
            }
        });
    }

    async loadWasm(buffer) {
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
    window.Wanix = Wanix;
}

