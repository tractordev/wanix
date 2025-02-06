import * as duplex from "./duplex.min.js";

export class Wanix {
    constructor() {
        const channel = new MessageChannel();
        const sess = new duplex.Session(new duplex.PortConn(channel.port1));
        this.peer = new duplex.Peer(sess, new duplex.CBORCodec());

        const go = new window.Go(); 
        window.wanixPort = new duplex.PortConn(channel.port2)
        WebAssembly.instantiateStreaming(fetch("./wanix.wasm"), go.importObject).then(obj => {
            go.run(obj.instance);
        });   
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

    async writeFile(name, contents) {
        if (typeof contents === "string") {
            contents = (new TextEncoder()).encode(contents);
        }
        return (await this.peer.call("WriteFile", [name, contents])).value;
    }

    async openInode(name) {
        return (await this.peer.call("OpenInode", [name])).value;
    }

    async read(name, offset, count) {
        return (await this.peer.call("Read", [name, offset, count])).value;
    }

    async remove(name) {
        await this.peer.call("Remove", [name]);
    }
}



