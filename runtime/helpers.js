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
