
import * as duplex from "./duplex.min.js";
import { setupConsoleHelpers } from "./helpers.js";
import { setupWio } from "./wio.js";
import { WanixFS } from "./fs.js";

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

        // todo: put this somewhere else
        const v86Script = document.createElement('script');
        v86Script.src = "./v86/libv86.js";
        document.head.appendChild(v86Script);

        const sys = new MessageChannel();
        super(sys.port1);
        
        window.wanix = this.context = {
            config,
            instance: this,
            sys: new duplex.PortConn(sys.port2),
            sw: new MessageChannel(),

            // kludge: for worker
            _toport: (port) => new duplex.PortConn(port), 
        };

        if (config.helpers) {
            setupConsoleHelpers();
        }

        this.loadWasm("./wanix.wasm");
        
    }

    async loadWasm(url) {
        const response = await fetch(url);
        const wasmBytes = new Uint8Array(await response.arrayBuffer());
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

