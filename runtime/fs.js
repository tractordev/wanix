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

    async bind(name, newname) {
        await this.peer.call("Bind", [name, newname]);
    }

    async unbind(name, newname) {
        await this.peer.call("Unbind", [name, newname]);
    }
    
    async readFile(name) {
        return (await this.peer.call("ReadFile", [name])).value;
    }

    async readText(name) {
        return (new TextDecoder()).decode(await this.readFile(name));
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