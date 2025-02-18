
import { wasi, WASI, File, Directory, OpenDirectory,OpenFile, ConsoleStdout } from "./browser_wasi_shim/browser_wasi_shim.js";
import { ValueBuffer } from "./wanix.js";

class WanixFile extends File {
    constructor(buf, path) {
        super([]);
        this.path = path;
        this.buf = buf;
    }

    path_open(oflags, fs_rights_base, fd_flags) {
        this.data = this.buf.getBytes({sync: "file", name: this.path});
        return super.path_open(oflags, fs_rights_base, fd_flags);
    }
    
    stat() {
        this.data = this.buf.getBytes({sync: "file", name: this.path});
        return super.stat();
    }
}

class WanixMap {
    constructor(buf, path, dir) {
        this.map = new Map();
        this.path = path;
        this.buf = buf;
        this.dir = dir;
    }

    fetch() {
        this.map = new Map(this.buf.get({sync: "dir", name: this.path}).map(e => {
            let prefix = this.path + "/";
            if (this.path == ".") {
                prefix = "";
            }
            if (e.endsWith("/")) {
                return [e.slice(0, -1), new WanixDir(new WanixMap(this.buf, prefix + e.slice(0, -1), this.dir), this.dir)];
            } else {
                return [e, new WanixFile(this.buf, prefix + e)];
            }
        }));
    }

    set(key, value) {
    }

    keys() {
        this.fetch();
        return this.map.keys();
    }

    entries() {
        this.fetch();
        return this.map.entries();
    }
    
    get(key) {
        this.fetch();
        return this.map.get(key);
    }

    delete(key) {
        // this.fetch();
        // return this.map.delete(key);
    }

    get size() {
        this.fetch();
        return this.map.size;
    }
}

class WanixDir extends Directory {
    constructor(contents, parent) {
        super([]);
        this.parent = parent;
        this.contents = contents;
    }
}

export class PreopenDir extends OpenDirectory {
    constructor(name, dir) {
      super(dir);
      this.prestat_name = name;
    }
  
    fd_prestat_get() {
      return {
        ret: 0,
        prestat: wasi.Prestat.dir(this.prestat_name),
      };
    }
}

var buf = undefined;
self.onmessage = async (e) => {
    if (e.data.sync) {
        buf = new ValueBuffer(e.data.sync.shared);
        
        let args = ["bin", "arg1", "arg2"];
        let env = ["FOO=bar"];
        let fds = [
            new OpenFile(new File([])), // stdin
            ConsoleStdout.lineBuffered(msg => console.log(`[WASI stdout] ${msg}`)),
            ConsoleStdout.lineBuffered(msg => console.warn(`[WASI stderr] ${msg}`)),
            new PreopenDir(".", new WanixDir(new WanixMap(buf, ".", null), null)),
        ];
        let wasi = new WASI(args, env, fds);

        let wasm = await WebAssembly.compileStreaming(fetch("./browser_wasi_shim/example_rs.wasm"));
        let inst = await WebAssembly.instantiate(wasm, {
            "wasi_snapshot_preview1": wasi.wasiImport,
        });
        wasi.start(inst);
    }
}