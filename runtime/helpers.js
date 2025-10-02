export function setupConsoleHelpers(id) {
    const instance = window.__wanix[id];
    window.list = (name) => { 
        instance.readDir(name).then(console.log); 
    };
    window.read = (name) => { 
        instance.readFile(name).then(d => (new TextDecoder()).decode(d)).then(console.log); 
    };
    window.readBytes = (name) => { 
        instance.readFile(name).then(console.log); 
    };
    window.write = (name, content) => { 
        instance.writeFile(name, content); 
    };
    window.mkdir = (name) => { 
        instance.makeDir(name); 
    };
    window.bind = (name, newname) => { 
        instance.bind(name, newname); 
    };
    window.unbind = (name, newname) => { 
        instance.unbind(name, newname); 
    };
    window.rm = (name) => { 
        instance.remove(name); 
    };
    window.stat = (name) => { 
        instance.stat(name).then(console.log); 
    };
    window.tail = async (name) => {
        const fd = await instance.open(name);
        while (true) {
            const data = await instance.read(fd, 1024);
            if (!data) {
                break;
            }
            console.log((new TextDecoder()).decode(data));
        }
        instance.close(fd);
    };


    window.makeScreen = () => {
        const screen = document.createElement('div');
        const div = document.createElement('div');
        const canvas = document.createElement('canvas');
        screen.appendChild(div);
        screen.appendChild(canvas);
        screen.id = 'screen';
        screen.style.display = 'none';
        document.body.appendChild(screen);
    };

    window.showScreen = () => {
        const screen = document.getElementById('screen');
        screen.style.display = 'block';
    };

    window.hideScreen = () => {
        const screen = document.getElementById('screen');
        screen.style.display = 'none';
    };

}
