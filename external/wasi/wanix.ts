import { wasi, Fd, Inode } from "@bjorn3/browser_wasi_shim";
import { Caller, Responder } from "./callbuffer.ts";
import { Directory, File } from "./fs.ts";


class WanixHandle {
    caller: Caller;
    path: string;

    constructor(caller: Caller, path: string) {
        this.caller = caller;
        this.path = path;
    }

    subpath(path: string): string {
        if (this.path === ".") {
            return path;
        }
        return [this.path, path].join("/");
    }
}

export class FileHandle extends WanixHandle {
    fd: number|undefined;

    open(): void {
        this.fd = this.caller.call("path_open", { path: this.path });
    }

    close(): void {
        this.caller.call("fd_close", { fd: this.fd });
    }

    flush(): void {
        this.caller.call("fd_flush", { fd: this.fd });
    }

    read(buffer: ArrayBuffer|Uint8Array, options?: { at: number }): number {
        let at = 0;
        if (options?.at) {
            at = options.at;
        }
        const count = buffer.byteLength;
        const data = this.caller.call("fd_read", { fd: this.fd, count, at });
        let writeBuffer;
        if (buffer instanceof ArrayBuffer) {
            writeBuffer = new Uint8Array(buffer);
        } else if (buffer instanceof Uint8Array) {
            writeBuffer = buffer;
        } else {
            throw new Error('Buffer must be ArrayBuffer or Uint8Array');
        }
        writeBuffer.set(data, 0);
        return data.length;
    }

    write(buffer: ArrayBuffer, options?: { at: number }): number {
        let at = 0;
        if (options?.at) {
            at = options.at;
        }
        const data = new Uint8Array(buffer);
        return this.caller.call("fd_write", { fd: this.fd, data, at });
    }

    truncate(to: number): void {
        this.caller.call("path_truncate", { path: this.path, to });
    }

    getSize(): number {
        return this.caller.call("path_size", { path: this.path });
    }
  }
  
export class DirectoryHandle extends WanixHandle {
    dirCache: Map<string, Inode>;
    lastReadDir: number;

    newEntry(name: string, isDir: boolean): Inode {
        if (isDir) {
            const handle = new DirectoryHandle(this.caller, this.subpath(name))
            return new Directory(handle);
        } else {
            const handle = new FileHandle(this.caller, this.subpath(name));
            return new File(handle);
        }
    }

    readDir(): Map<string, Inode> {
        if (performance.now() - this.lastReadDir < 1000) {
            return this.dirCache;
        }
        this.lastReadDir = performance.now();
        const m = new Map<string, Inode>();
        const entries = this.caller.call("path_readdir", { path: this.path }) || [];
        for (const entry of entries) {
            let isDir = false;
            let name = entry;
            if (name.slice(-1) === "/") {
                isDir = true;
                name = name.slice(0, -1);
            }
            m.set(name, this.newEntry(name, isDir));
        }
        this.dirCache = m;
        return m;
    }

    removeEntry(name: string): boolean {
        return this.caller.call("path_remove", { path: this.subpath(name) });
    }

    createLink(name: string, entry: Inode): boolean {
        return false;
    }

    createFile(name: string, entry: Inode): boolean {
        return this.caller.call("path_touch", { path: this.subpath(name) });
    }

    createDirectory(name: string, entry: Inode): boolean {
        return this.caller.call("path_mkdir", { path: this.subpath(name) });
    }
}