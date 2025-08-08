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


    // window.makeScreen = () => {
    //     const screen = document.createElement('div');
    //     const div = document.createElement('div');
    //     const canvas = document.createElement('canvas');
    //     screen.appendChild(div);
    //     screen.appendChild(canvas);
    //     screen.id = 'screen';
    //     document.body.appendChild(screen);
    // };

    // window.bootAlpine = async (screen=false) => {
    //     if (screen) {
    //         makeScreen();
    //     }
    //     const w = window.wanix.instance;
    //     const vid = (await w.readText("web/vm/new")).trim();
    //     await w.writeFile("task/1/ctl", `bind #console/data1 web/vm/${vid}/ttyS0`);
    //     await w.writeFile("task/1/ctl", `bind #alpine web/vm/${vid}/fsys`);
    //     await w.writeFile("task/1/ctl", `bind web/opfs/init web/vm/${vid}/fsys/bin/init`);
    //     await w.writeFile("task/1/ctl", `bind web/opfs/bin/post-dhcp web/vm/${vid}/fsys/bin/post-dhcp`);
    //     // await w.writeFile("task/1/ctl", "bind . web/vm/2/fsys");
    //     await w.writeFile(`web/vm/${vid}/ctl`, "start");
    // }
}
