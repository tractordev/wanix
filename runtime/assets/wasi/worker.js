import { WanixFS, CallBuffer } from "../wanix.min.js";

self.onmessage = async (e) => {
    if (!e.data.worker) {
        return;
    }

    const pid = e.data.worker.env.pid;
    const fs = new WanixFS(e.data.worker.fsys);

    const shared = new SharedArrayBuffer(16384);
    const call = new CallBuffer(shared);

    const worker = new Worker("./worker_sync.js", {type: "module"});
    worker.onmessage = async (e) => {
        if (!e.data.method) {
            return;
        }
        // console.log(e.data);
        const start = performance.now();
        switch (e.data.method) {
        case "path_open":
            const fd = await fs.open(e.data.path);
            call.respond(fd);
            break;

        case "path_truncate":
            await fs.truncate(e.data.path, e.data.to);
            call.respond(true);
            break;

        case "path_size":
            const stat = await fs.stat(e.data.path);
            call.respond(stat.Size);
            break;

        case "path_readdir":
            const entries = await fs.readDir(e.data.path);
            call.respond(entries);
            break;

        case "path_remove":
            await fs.remove(e.data.path);
            call.respond(true);
            break;

        case "path_mkdir":
            await fs.makeDir(e.data.path);
            call.respond(true);
            break;

        case "path_touch":
            await fs.writeFile(e.data.path, "");
            call.respond(true);
            break;

        case "fd_close":
            await fs.close(e.data.fd);
            call.respond(true);
            break;

        case "fd_flush":
            await fs.sync(e.data.fd);
            call.respond(true);
            break;

        case "fd_read":
            // const at = e.data.at;
            const data = await fs.read(e.data.fd, e.data.count/*, at*/);
            call.respond(data);
            break;

        case "fd_write":
            // const at = e.data.at;
            const wn = await fs.write(e.data.fd, e.data.data/*, at*/);
            call.respond(wn);
            break;

        case "exit":
            await fs.writeFile(`task/${pid}/exit`, e.data.code.toString());
            call.respond(true);
            break;
        
        default:
            console.warn(`unknown method: ${e.data.method}`);
        }
        // console.log(performance.now() - start);
    }

    const cmd = await fs.readFile(`task/${pid}/cmd`);
    const env = await fs.readFile(`task/${pid}/env`);
    worker.postMessage({
        buffer: shared, 
        args: (new TextDecoder()).decode(cmd).trim().split(" "),
        env: (new TextDecoder()).decode(env).trim().split("\n"),
        stdin: `task/${pid}/.sys/fd0`,
        stdout: `task/${pid}/.sys/fd1`,
        stderr: `task/${pid}/.sys/fd2`,
    });
}