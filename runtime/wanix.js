import { Terminal } from 'xterm';
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

        // todo: put this somewhere else
        const v86Script = document.createElement('script');
        v86Script.src = "./v86/libv86.js";
        document.head.appendChild(v86Script);

<<<<<<< HEAD
=======
        // todo: properly detect which to use
        const execScript = document.createElement('script');
        execScript.textContent = wasmExecGo;
        document.head.appendChild(execScript);

        const term = new Terminal({
            cursorBlink: true,
            convertEol: true,
        });
        term.open(document.getElementById('terminal'));

        const ws = new WebSocket("ws://" + window.location.host + "/wanix/ws");
        ws.binaryType = "arraybuffer";

        ws.onopen = () => {
            console.log("wanix: websocket connected");
            term.onData(data => {
                ws.send(data);
            });
            ws.onmessage = (event) => {
                term.write(new Uint8Aray(event.data));
            };
        };

        ws.onclose = () => {
            console.log("wanix: websocket disconnected");
            term.write("\r\n\r\n[Connection closed]\r\n");
        };

>>>>>>> 3238d25cd13dd2a9d7bfcd5e609f8c2f4866356a
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
        const bundleFiles = await untar(await new Response(stream).arrayBuffer()).progress((f) => {
            if (f.name === "wanix.wasm") {
                console.log("loading wasm from bundle");
                this.loadWasm(f.buffer);
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