import * as duplex from "@progrium/duplex";

export class WanixHandle {
    constructor(port) {
        const sess = new duplex.Session(new duplex.PortConn(port));
        this.peer = new duplex.Peer(sess, new duplex.CBORCodec());
        this.logger = () => null;
    }

    async readDir(name) {
        this.logger(`readDir ${name}`);
        return (await this.peer.call("ReadDir", [name])).value;
    }

    async makeDir(name) {
        this.logger(`makeDir ${name}`);
        await this.peer.call("Mkdir", [name]);
    }

    async makeDirAll(name) {
        this.logger(`makeDirAll ${name}`);
        await this.peer.call("MkdirAll", [name]);
    }

    async bind(name, newname) {
        this.logger(`unbind ${name} ${newname}`);
        await this.peer.call("Bind", [name, newname]);
    }

    async unbind(name, newname) {
        this.logger(`unbind ${name} ${newname}`);
        await this.peer.call("Unbind", [name, newname]);
    }
    
    async readFile(name) {
        this.logger(`readFile ${name}`);
        return (await this.peer.call("ReadFile", [name])).value;
    }

    // not sure if readFile approach is good, but this is an option for now
    async readFile2(name) {
        this.logger(`readFile2 ${name}`);
        const rd = await this.openReadable(name);
        const response = new Response(rd); // cute trick
        return new Uint8Array(await response.arrayBuffer());
    }

    async readText(name) {
        return (new TextDecoder()).decode(await this.readFile(name));
    }

    async waitFor(name, timeoutMs=1000) {
        this.logger(`waitFor ${name} ${timeoutMs}ms`);
        await this.peer.call("WaitFor", [name, timeoutMs]);
    }

    async stat(name) {
        this.logger(`stat ${name}`);
        return (await this.peer.call("Stat", [name])).value;
    }

    async writeFile(name, contents) {
        this.logger(`writeFile ${name} len(${contents.length})`);
        if (typeof contents === "string") {
            contents = (new TextEncoder()).encode(contents);
        }
        return (await this.peer.call("WriteFile", [name, contents])).value;
    }

    async appendFile(name, contents) {
        this.logger(`appendFile ${name} len(${contents.length})`);
        if (typeof contents === "string") {
            contents = (new TextEncoder()).encode(contents);
        }
        return (await this.peer.call("AppendFile", [name, contents])).value;
    }

    async rename(oldname, newname) {
        this.logger(`rename ${oldname} ${newname}`);
        await this.peer.call("Rename", [oldname, newname]);
    }

    async copy(oldname, newname) {
        this.logger(`copy ${oldname} ${newname}`);
        await this.peer.call("Copy", [oldname, newname]);
    }

    async remove(name) {
        this.logger(`remove ${name}`);
        await this.peer.call("Remove", [name]);
    }

    async removeAll(name) {
        this.logger(`removeAll ${name}`);
        await this.peer.call("RemoveAll", [name]);
    }

    async truncate(name, size) {
        this.logger(`truncate ${name} ${size}`);
        await this.peer.call("Truncate", [name, size]);
    }

    async create(name) {
        this.logger(`create ${name}`);
        return (await this.peer.call("Create", [name])).value;
    }

    async open(name) {
        this.logger(`open ${name}`);
        return (await this.peer.call("Open", [name])).value;
    }

    async openFile(name, flags, mode) {
        this.logger(`openFile ${name} ${flags} ${mode}`);
        return (await this.peer.call("OpenFile", [name, flags, mode])).value;
    }

    async read(fd, count) {
        this.logger(`read ${fd} ${count}`);
        return (await this.peer.call("Read", [fd, count])).value;
    }

    async write(fd, data) {
        this.logger(`write ${fd} len(${data.length})`);
        return (await this.peer.call("Write", [fd, data])).value;
    }

    async writeAt(fd, data, offset) {
        this.logger(`writeAt ${fd} ${offset}`);
        return (await this.peer.call("WriteAt", [fd, data, offset])).value;
    }

    async close(fd) {
        this.logger(`close ${fd}`);
        return (await this.peer.call("Close", [fd])).value;
    }

    async sync(fd) {
        this.logger(`sync ${fd}`);
        return (await this.peer.call("Sync", [fd])).value;
    }

    async fstat(fd) {
        this.logger(`fstat ${fd}`);
        return (await this.peer.call("Fstat", [fd])).value;
    }

    async lstat(name) {
        this.logger(`lstat ${name}`);
        return (await this.peer.call("Lstat", [name])).value;
    }

    async chmod(name, mode) {
        this.logger(`chmod ${name} ${mode}`);
        await this.peer.call("Chmod", [name, mode]);
    }

    async chown(name, uid, gid) {
        this.logger(`chown ${name} ${uid} ${gid}`);
        await this.peer.call("Chown", [name, uid, gid]);
    }

    async fchmod(fd, mode) {
        this.logger(`fchmod ${fd} ${mode}`);
        await this.peer.call("Fchmod", [fd, mode]);
    }

    async fchown(fd, uid, gid) {
        this.logger(`fchown ${fd} ${uid} ${gid}`);
        await this.peer.call("Fchown", [fd, uid, gid]);
    }

    async ftruncate(fd, length) {
        this.logger(`ftruncate ${fd} ${length}`);
        await this.peer.call("Ftruncate", [fd, length]);
    }

    async readlink(name) {
        this.logger(`readlink ${name}`);
        return (await this.peer.call("Readlink", [name])).value;
    }

    async symlink(oldname, newname) {
        this.logger(`symlink ${oldname} ${newname}`);
        await this.peer.call("Symlink", [oldname, newname]);
    }

    async chtimes(name, atime, mtime) {
        this.logger(`chtimes ${name} ${atime} ${mtime}`);
        await this.peer.call("Chtimes", [name, atime, mtime]);
    }

    async openReadable(name) {
        this.logger(`openReadable ${name}`);
        const fd = await this.open(name);
        return this.readable(fd);
    }

    async openWritable(name) {
        this.logger(`openWritable ${name}`);
        const fd = await this.openFile(name, 1, 0);
        return this.writable(fd);
    }

    writable(fd) {
        const self = this;
        return new WritableStream({
            write(chunk) {
                return self.write(fd, chunk);
            },
        });
    }

    readable(fd) {
        const self = this;
        return new ReadableStream({
            async pull(controller) {
                const data = await self.read(fd, 1024);
                if (data === null) {
                    controller.close();
                }
                controller.enqueue(data);
            },
        });
    }
}
