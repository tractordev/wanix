export function setupDevtools(el) {
    const handle = el.root;
    globalThis.list = (name) => { 
        handle.readDir(name).then(console.log); 
    };
    globalThis.read = (name) => { 
        handle.readFile(name).then(d => (new TextDecoder()).decode(d)).then(console.log); 
    };
    globalThis.readBytes = (name) => { 
        handle.readFile(name).then(console.log); 
    };
    globalThis.write = (name, content) => { 
        handle.writeFile(name, content); 
    };
    globalThis.mkdir = (name) => { 
        handle.makeDir(name); 
    };
    globalThis.bind = (name, newname) => { 
        handle.bind(name, newname); 
    };
    globalThis.unbind = (name, newname) => { 
        handle.unbind(name, newname); 
    };
    globalThis.rm = (name) => { 
        handle.remove(name); 
    };
    globalThis.stat = (name) => { 
        handle.stat(name).then(console.log); 
    };
    globalThis.tail = async (name) => {
        const fd = await handle.open(name);
        while (true) {
            const data = await handle.read(fd, 1024);
            if (!data) {
                break;
            }
            console.log((new TextDecoder()).decode(data));
        }
        handle.close(fd);
    };


    globalThis.makeScreen = () => {
        const screen = document.createElement('div');
        const div = document.createElement('div');
        const canvas = document.createElement('canvas');
        screen.appendChild(div);
        screen.appendChild(canvas);
        screen.id = 'screen';
        screen.style.display = 'none';
        document.body.appendChild(screen);
    };

    globalThis.showScreen = () => {
        const screen = document.getElementById('screen');
        screen.style.display = 'block';
    };

    globalThis.hideScreen = () => {
        const screen = document.getElementById('screen');
        screen.style.display = 'none';
    };

}
