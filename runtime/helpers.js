export function setupConsoleHelpers() {
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
            await w.readFile("web/vm/new");

            await w.writeFile("task/1/ctl", "bind #console/data web/dom/1/data");
            await w.writeFile("task/1/ctl", "bind #console/data1 web/vm/1/ttyS0");
            
            // await w.writeFile("task/1/ctl", "bind web/dom/1/data web/vm/1/ttyS0");

            await w.writeFile("web/dom/body/ctl", "append-child 1");
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
        await w.writeFile("task/1/ctl", "bind web/opfs/init web/vm/2/fsys/bin/init");
        await w.writeFile("task/1/ctl", "bind web/opfs/bin/post-dhcp web/vm/2/fsys/bin/post-dhcp");
        // await w.writeFile("task/1/ctl", "bind . web/vm/2/fsys");
        await w.writeFile("web/vm/2/ctl", "start");
    }
}
