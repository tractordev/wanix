import * as duplex from "./duplex.min.js";


export class WanixFS {
    constructor(port) {
        const sess = new duplex.Session(new duplex.PortConn(port));
        this.peer = new duplex.Peer(sess, new duplex.CBORCodec());
    }

    async readDir(name) {
        return (await this.peer.call("ReadDir", [name])).value;
    }

    async makeDir(name) {
        await this.peer.call("Mkdir", [name]);
    }

    async readFile(name) {
        return (await this.peer.call("ReadFile", [name])).value;
    }

    async stat(name) {
        return (await this.peer.call("Stat", [name])).value;
    }

    async writeFile(name, contents) {
        if (typeof contents === "string") {
            contents = (new TextEncoder()).encode(contents);
        }
        return (await this.peer.call("WriteFile", [name, contents])).value;
    }

    async remove(name) {
        await this.peer.call("Remove", [name]);
    }

    async open(name) {
        return (await this.peer.call("Open", [name])).value;
    }

    async read(fd, count) {
        return (await this.peer.call("Read", [fd, count])).value;
    }

    async write(fd, data) {
        return (await this.peer.call("Write", [fd, data])).value;
    }

    async close(fd) {
        return (await this.peer.call("Close", [fd])).value;
    }

    // deprecated
    async openInode(name) {
        return (await this.peer.call("OpenInode", [name])).value;
    }

}


export class Wanix extends WanixFS {
    constructor(config={}) {
        if (window.wanix) {
            throw new Error("Wanix already initialized on this page");
        }

        const sys = new MessageChannel();
        super(sys.port1);
        
        window.wanix = this.context = {
            config,
            instance: this,
            sys: new duplex.PortConn(sys.port2),
            sw: new MessageChannel(),
            _toport: (port) => new duplex.PortConn(port), // kludge: for worker
            // virtioRecv: (buf) => console.log("virtioRecv", buf),
        };

        const go = new window.Go(); 
        WebAssembly.instantiateStreaming(fetch("./wanix.wasm"), go.importObject).then(obj => {
            go.run(obj.instance);
        }); 
        
    }
}

export class ValueBuffer {
    constructor(buf) {
        this.shared = buf;
        this.ctrl = new Int32Array(this.shared, 0, 2);
        this.status = new Int32Array(this.shared, 4, 1);
        this.len = new Int32Array(this.shared, 8, 1);
        this.data = new Uint8Array(this.shared, 12);
    }

    set(value) {
        const buf = (new TextEncoder()).encode(JSON.stringify(value));
        this.len[0] = buf.length;
        this.data.set(buf, 0);
        this.status[0] = 0;

        Atomics.store(this.ctrl, 0, 1);
        Atomics.notify(this.ctrl, 0);
    }

    get(msg) {
        this.ctrl[0] = 0; 
        postMessage(msg);
        Atomics.wait(this.ctrl, 0, 0);
        const data = this.data.slice(0, this.len[0]);
        return JSON.parse(new TextDecoder().decode(data));
    }

    getBytes(msg) {
        this.ctrl[0] = 0; 
        postMessage(msg);
        Atomics.wait(this.ctrl, 0, 0);
        return this.data.slice(0, this.len[0]);
    }

}