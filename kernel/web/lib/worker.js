// import is a keyword not a global.
// this makes it usable from go.
globalThis.import = (url) => import(url);

addEventListener("message", async (e) => {
  if (e.data.duplex) return;
  if (e.data.init) {
    
    globalThis.initfs = e.data.init.fs;
    globalThis.process.pid = e.data.init.pid;
    globalThis.process.ppid = e.data.init.ppid;
    globalThis.process.dir = e.data.init.dir;

    globalThis.duplex = await import(URL.createObjectURL(initfs["duplex.js"]));
    globalThis.task = await import(URL.createObjectURL(initfs["task.js"])); // only for kernel
    
    globalThis.sys = duplex.open(new duplex.WorkerConn(globalThis), new duplex.CBORCodec());
    
    sys.handle("exec", duplex.HandlerFunc(async (resp, call) => {
      const params = await call.receive();
      const go = new Go();
      go.argv = [params[0], ...(params[1]||[])];
      if (params[2].env) {
        go.env = params[2].env;
      }
      const res = await WebAssembly.instantiate(await blobToArrayBuffer(initfs[params[0]]), go.importObject);
      await go.run(res.instance);
      resp.return(go.exitcode);
    }))
    sys.handle("stdout", duplex.HandlerFunc(async (resp, call) => {
      const ch = await resp.continue();
      globalThis.stdout = (buf) => {
        ch.write(buf);
      }
    }));
    sys.handle("stderr", duplex.HandlerFunc(async (resp, call) => {
      const ch = await resp.continue();
      globalThis.stderr = (buf) => {
        ch.write(buf);
      }
    }));
    sys.handle("output", duplex.HandlerFunc(async (resp, call) => {
      const ch = await resp.continue();
      globalThis.stdout = (buf) => {
        ch.write(buf);
      }
      globalThis.stderr = (buf) => {
        ch.write(buf);
      }
    }));
    sys.handle("stdin", duplex.HandlerFunc(async (resp, call) => {
      await call.receive();
      const ch = await resp.continue();
      globalThis.stdin = (buf, cb) => {
        ch.read(buf).then(n => cb(null, n));
      }
    }));
    sys.respond();
    
    postMessage({ready: true});
  }
});

function blobToArrayBuffer(blob) {
  return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = event => resolve(event.target.result);
      reader.onerror = reject;
      reader.readAsArrayBuffer(blob);
  });
}

//# sourceURL=worker.js