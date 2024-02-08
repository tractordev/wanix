
export class Task {
  constructor(initfs, pid=0) {
    this.initfs = initfs;
    this.pid = pid;
    this.finished = undefined;
    this.worker = undefined;
  }

  async exec(path, args, opts={}) {
    const name = `${path.split('/').pop()}.${this.pid}`;

    const blob = new Blob([
      this.initfs["worker.js"].blob, 
      this.initfs["wasm.js"].blob,
      `\n//# sourceURL=${name}\n` // names the worker in logs
    ], { type: 'application/javascript' });
    
    this.worker = new Worker(URL.createObjectURL(blob), {type: "module", name});
    this.worker.onerror = (e) => { throw e };
    this.worker.postMessage({init: {
      pid: this.pid,
      ppid: (globalThis.process) ? globalThis.process.pid : -1,
      fs: this.initfs,
      dir: opts.dir || "/",
      hostURL: location.href,
    }});
    
    const taskReady = new Promise((resolve) => {
      this.worker.addEventListener("message", (e) => {
        if (e.data.ready) {
          resolve();
        }
      })
    });

    const duplex = await import(URL.createObjectURL(this.initfs["duplex.js"].blob));
    this.pipe = duplex.open(new duplex.WorkerConn(this.worker), new duplex.CBORCodec());
    this.pipe.respond();

    // if in kernel worker
    if (globalThis.api) {
      this.pipe.handle("kernel", duplex.handlerFrom(globalThis.api.kernel));
      this.pipe.handle("fs", duplex.handlerFrom(globalThis.api.fs));
      this.pipe.handle("proc", duplex.handlerFrom(globalThis.api.proc));
      this.pipe.handle("tty", duplex.handlerFrom(globalThis.api.tty));  
      this.pipe.handle("host", duplex.handlerFrom(globalThis.api.host));  
    }
    
    await taskReady;

    this.finished = this.call("exec", [path, args, opts]);
    this.finished.then(() => {
      this.terminate();
    });
  } 

  call(selector, args) {
    return this.pipe.call(selector, args);
  }

  async terminate() {
    if (!this.worker) {
      throw new Error("no worker");
    }

    await this.pipe.close();
    this.worker.terminate();
  }

  async wait() {
    if (!this.worker) {
      throw new Error("no worker");
    }

    const resp = await this.finished;
    return resp.value;
  }

  async stdout() {
    if (!this.worker) {
      throw new Error("no worker");
    }
    const resp = await this.call("stdout");
    return resp.channel;
  }

  async stderr() {
    if (!this.worker) {
      throw new Error("no worker");
    }
    const resp = await this.call("stderr");
    return resp.channel;
  }

  async output() {
    if (!this.worker) {
      throw new Error("no worker");
    }
    const resp = await this.call("output");
    return resp.channel;
  }

  async stdin() {
    if (!this.worker) {
      throw new Error("no worker");
    }
    const resp = await this.call("stdin");
    return resp.channel;
  }
}
//# sourceURL=task.js
