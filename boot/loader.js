if (!globalThis["ServiceWorkerGlobalScope"]) {
  const basePath = window.location.pathname.replace("index.html", "");

  // registers Service Worker using this file (see bottom) if none is registered,
  // and sets up a mechanism to fullfill requests from bootfs or kernel
  async function setupServiceWorker() {
    const timeout = (ms) => new Promise((resolve, reject) => setTimeout(() => reject(new Error("Timeout")), ms));

    let registration = await navigator.serviceWorker.getRegistration();
    if (!registration) {
      console.log("Registering service worker...");
      await navigator.serviceWorker.register("./wanix-bootloader.js?sw", {type: "module"});
      registration = await navigator.serviceWorker.ready;
      await Promise.race([
        new Promise((resolve) => {
          navigator.serviceWorker.addEventListener("controllerchange", async (event) => {
            resolve();
          });
        }),
        timeout(3000)
      ]).catch(err => console.warn(err));
    }
    if (registration.active && !navigator.serviceWorker.controller) {
      // Perform a soft reload to load everything from the SW and get
      // a consistent set of resources.
      window.location.reload();
    }
    
    let readyResolver = undefined;
    const ready = new Promise((resolve) => {
      readyResolver = resolve;
    });

    navigator.serviceWorker.addEventListener("message", async (event) => {
      if (event.data.ready) {
        readyResolver();
        return;
      }
      const req = event.data.request;
      if (!req) {
        return;
      }

      if (!globalThis.sys) {
        registration.active.postMessage({response: { reqId: req.id,  error: `kernel not loaded yet for ${req.path}` }});
        return;
      }

      // handle request using kernel via rpc
      const resp = await globalThis.sys.call("web.request", [req.method, req.url.replace(basePath, "/")]);
      const headers = resp.value;
      const ch = resp.channel;
      const buf = new duplex.Buffer();

      await duplex.copy(buf, ch);
      ch.close();

      registration.active.postMessage({response: { reqId: req.id, body: buf.bytes(), headers }});
    });

    console.log("Initializing service worker...");
    registration.active.postMessage({init: true, basePath});
    await ready;
  }

  async function fetchKernel() {
    const resp = await fetch("./wanix-kernel.gz");
    const gzipBlob = await resp.blob();
    const ds = new DecompressionStream('gzip');
    const out = gzipBlob.stream().pipeThrough(ds);
    const response = new Response(out);
    globalThis.bootfs["kernel"] = { mtimeMs: Date.now(), blob: await response.blob() };
  }

  const unzipb64 = async (b64data) => {
    const gzipData = atob(b64data);
    const gzipBuf = new Uint8Array(gzipData.length);
    for (let i = 0; i < gzipData.length; i++) {
      gzipBuf[i] = gzipData.charCodeAt(i);
    }
    const gzipBlob = new Blob([gzipBuf], { type: 'application/gzip' });
    const ds = new DecompressionStream('gzip');
    const out = gzipBlob.stream().pipeThrough(ds);
    const response = new Response(out);
    const buf = await response.arrayBuffer();
    return new Uint8Array(buf);
  }

  // bootloader starts here
  globalThis.bootWanix = (async function() {
    console.log("Wanix booting...")
    
    globalThis.bootfs = {};
    const kernelReady = fetchKernel();
    await setupServiceWorker();

    // allow loading concurrently
    let loads = [];
    for(const filename in globalThis.bootdata) {
      loads.push((async () => {
        console.log(`Loading ${filename}...`)
        const file = globalThis.bootdata[filename];
        const data = await unzipb64(file.data);
        globalThis.bootfs[filename] = { mtimeMs: file.mtimeMs, blob: new Blob([data], { type: file.type }) };
      })());
    }
    await Promise.all(loads);

    globalThis.duplex = await import(URL.createObjectURL(bootfs["duplex.js"].blob));
    globalThis.task = await import(URL.createObjectURL(bootfs["task.js"].blob));

    await kernelReady;
    globalThis.sys = new task.Task(bootfs);
    
    console.log("Starting kernel...")
    await sys.exec("kernel");

    // load host API
    await import(URL.createObjectURL(bootfs["host.js"].blob));
  });
}

// this file is also used as the Service Worker source. 
// below is ignored unless in a Service Worker.
if (globalThis["ServiceWorkerGlobalScope"] && self instanceof ServiceWorkerGlobalScope) {
  let host = undefined;
  let responders = {};
  let reqId = 0;
  let basePath = "/";

  self.addEventListener("message", (event) => {
    if (event.data.init) {
      host = event.source;
      basePath = event.data.basePath;
      host.postMessage({ready: true});
      return;
    }
    if (responders && event.data.response) {
      responders[event.data.response.reqId](event.data.response);
    }
  });

  self.addEventListener("fetch", async (event) => {
    const req = event.request;
    const url = new URL(req.url);
    if (url.pathname === "/favicon.ico" || 
      url.pathname === basePath ||
      url.pathname.startsWith(`${basePath}wanix-bootloader.js`) ||
      url.pathname.startsWith(`${basePath}sys/dev`) || 
      url.pathname.startsWith(`${basePath}bootloader`) || 
      url.pathname.startsWith(`${basePath}index.html`) ||
      url.pathname.startsWith(`${basePath}loading.gif`) ||
      url.pathname.startsWith(`${basePath}wanix-kernel.gz`) ||
      url.pathname.startsWith("/auth") ||
      url.hostname !== location.hostname ||
      !host) return;

    reqId++;

    const headers = {}
    for (var p of req.headers) {
      headers[p[0]] = p[1]
    }

    const response = new Promise(resolve => {
      responders[reqId] = resolve;
    });
    event.respondWith(new Promise(async (resolve) => {
      host.postMessage({request: {method: req.method, url: req.url, path: url.pathname, headers: headers, id: reqId }});
      const reply = await response;
      if (reply.error) {
        console.warn(reply.error);
        resolve(Response.error());
        return;
      }
      resolve(new Response(reply.body, {
        headers: reply.headers, 
        status: reply.headers["Wanix-Status-Code"], 
        statusText: reply.headers["Wanix-Status-Text"]
      }))
    }))
  });
  
  self.addEventListener('activate', event => {
    event.waitUntil(clients.claim());
  });

}
