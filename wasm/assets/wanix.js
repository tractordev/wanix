
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

function setupConsoleHelpers() {
    window.list = (name) => { 
        window.wanix.instance.readDir(name).then(console.log); 
    };
    window.read = (name) => { 
        window.wanix.instance.readFile(name).then(d => (new TextDecoder()).decode(d)).then(console.log); 
    };
    window.readBytes = (name) => { 
        window.wanix.instance.readFile(name).then(console.log); 
    };
    window.write = (name, content) => { 
        window.wanix.instance.writeFile(name, content); 
    };
    window.mkdir = (name) => { 
        window.wanix.instance.makeDir(name); 
    };
    window.bind = (name, newname) => { 
        window.wanix.instance.bind(name, newname); 
    };
    window.unbind = (name, newname) => { 
        window.wanix.instance.unbind(name, newname); 
    };
    window.rm = (name) => { 
        window.wanix.instance.remove(name); 
    };
    window.stat = (name) => { 
        window.wanix.instance.stat(name).then(console.log); 
    };
    window.tail = async (name) => {
        const fd = await window.wanix.instance.open(name);
        while (true) {
            const data = await window.wanix.instance.read(fd, 1024);
            if (!data) {
                break;
            }
            console.log((new TextDecoder()).decode(data));
        }
        window.wanix.instance.close(fd);
    };

    window.bootShell = async (screen=false) => {
        if (screen) {
            const screen = document.createElement('div');
            const div = document.createElement('div');
            const canvas = document.createElement('canvas');
            screen.appendChild(div);
            screen.appendChild(canvas);
            screen.id = 'screen';
            document.body.appendChild(screen);
        }
        const w = window.wanix.instance;

        const query = new URLSearchParams(window.location.search);
        const url = query.get("tty");
        if (url) {
            // websocket tty mode 
            await w.readFile("cap/new/ws");
            await w.writeFile("cap/1/ctl", `mount ${url}`);
            await w.readFile("web/vm/new");
            await w.writeFile("task/1/ctl", "bind cap/1/data web/vm/1/ttyS0");
        } else {
            // xterm.js mode 
            await w.readFile("web/dom/new/xterm");
            await w.writeFile("web/dom/body/ctl", "append-child 1");
            await w.readFile("web/vm/new");
            await w.writeFile("task/1/ctl", "bind web/dom/1/data web/vm/1/ttyS0");
        }
        
        await w.writeFile("task/1/ctl", "bind . web/vm/1/fsys");
        await w.writeFile("task/1/ctl", "bind #shell web/vm/1/fsys");
        await w.writeFile("web/vm/1/ctl", "start");
    }

    window.bootAlpine = async (screen=false) => {
        if (screen) {
            const screen = document.createElement('div');
            const div = document.createElement('div');
            const canvas = document.createElement('canvas');
            screen.appendChild(div);
            screen.appendChild(canvas);
            screen.id = 'screen';
            document.body.appendChild(screen);
        }
        const w = window.wanix.instance;

        await w.readFile("web/dom/new/xterm");
        await w.writeFile("web/dom/body/ctl", "append-child 2");
        await w.readFile("web/vm/new");
        await w.writeFile("task/1/ctl", "bind web/dom/2/data web/vm/2/ttyS0");
        await w.writeFile("task/1/ctl", "bind #alpine web/vm/2/fsys");
        // await w.writeFile("task/1/ctl", "bind . web/vm/2/fsys");
        await w.writeFile("web/vm/2/ctl", "start");
    }
}
