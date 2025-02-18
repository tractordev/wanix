import { WanixFS, ValueBuffer } from "./wanix.js";

self.onmessage = (e) => {
    if (e.data.worker) {
        const fs = new WanixFS(e.data.worker.fsys);

        const shared = new SharedArrayBuffer(1024);
        const resp = new ValueBuffer(shared);
        
        const syncWorker = new Worker("./wasi_worker_sync.js", {type: "module"});
        syncWorker.postMessage({sync: {shared}});
        syncWorker.onmessage = (e) => {
            if (e.data.sync) {
                switch (e.data.sync) {
                    case "dir":
                        fs.readDir(e.data.name).then(entries => {
                            resp.set(entries);
                        });
                        break;
                    case "file":
                        fs.readFile(e.data.name).then(data => {
                            resp.set(data);
                        });
                        break;
                }
            }
        }
    }
    
}