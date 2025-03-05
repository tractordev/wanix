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

    async truncate(name, size) {
        await this.peer.call("Truncate", [name, size]);
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

    async sync(fd) {
        return (await this.peer.call("Sync", [fd])).value;
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
