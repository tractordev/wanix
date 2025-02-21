import { WanixFS, ValueBuffer } from "./wanix.js";

self.onmessage = async (e) => {
    if (e.data.worker) {
        const fs = new WanixFS(e.data.worker.fsys);

        const shared = new SharedArrayBuffer(1024);
        const resp = new ValueBuffer(shared);

        const pid = e.data.worker.env.pid;
        let stdout = null;
        if (pid) {
            stdout = await fs.open(`proc/${pid}/fd/worker1`);
        }

        const syncWorker = new Worker("./wasi_worker_sync.js", {type: "module"});
        syncWorker.postMessage({sync: {shared}, path: e.data.worker.cmdline.split(" ").slice(-1)});
        syncWorker.onmessage = async (e) => {
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
                    case "stdout":
                        if (stdout) {
                            await fs.write(stdout, (new TextEncoder()).encode(e.data.data));
                        } else {
                            console.log(e.data.data);
                        }
                        resp.set(true);
                        break;
                    case "stderr":
                        console.error(e.data.data);
                        resp.set(true);
                        break;
                    case "exit":
                        await fs.writeFile(`proc/${pid}/exit`, e.data.code.toString());
                        resp.set(true);
                        break;
                }
            }
        }
    }
    
}