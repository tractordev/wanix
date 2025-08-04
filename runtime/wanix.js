import { Terminal } from 'xterm';
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

        const sys = new MessageChannel();
        super(sys.port1);
        
        window.wanix = this.context = {
            config,
            instance: this,
            sys: new duplex.PortConn(sys.port2),
            sw: new MessageChannel(),
            _toport: (port) => new duplex.PortConn(port), // kludge: for worker
        };

        if (config.helpers) {
            setupConsoleHelpers();
        }

        const go = new window.Go(); 
        WebAssembly.instantiateStreaming(fetch("./wanix.wasm"), go.importObject).then(obj => {
            go.run(obj.instance);
        }); 
        
    }
}

if (globalThis.window) {
    window.Wanix = Wanix;
}