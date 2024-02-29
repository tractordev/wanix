if (!globalThis["ServiceWorkerGlobalScope"]) {
  const basePath = window.location.pathname.replace("index.html", "");

  // registers Service Worker using this file (see bottom) if none is registered,
  // and sets up a mechanism to fullfill requests from initfs or kernel
  async function setupServiceWorker() {
    const unzip = async (b64data) => {
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

    let registration = await navigator.serviceWorker.getRegistration();
    if (!registration) {
      await navigator.serviceWorker.register("./wanix-bootloader.js?sw", {type: "module"});
      registration = await navigator.serviceWorker.ready;
      await new Promise((resolve) => {
        navigator.serviceWorker.addEventListener("controllerchange", async (event) => {
          resolve();
        });
      });
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

      // handle requests for compressed embedded initfs files if present
      if (globalThis.initdata && req.path.startsWith(`${basePath}~init/`)) {
        const f = globalThis.initdata[req.path.replace(`${basePath}~init/`, "")];
        if (f) {
          const data = await unzip(f.data);
          registration.active.postMessage({response: { reqId: req.id, body: data, headers: {"content-type": f.type}}});
          return;
        }
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

    registration.active.postMessage({init: true, basePath});
    await ready;
  }

  // bootloader starts here
  (async function() {
    console.log("Wanix booting...")
    await setupServiceWorker();

    globalThis.initfs = {};
    const load = async (name, file) => {
      // Determine if file contains a path to fetch or embedded file contents to load.
      if(file.type === "text/plain") {
        globalThis.initfs[name] = { mtimeMs: file.mtimeMs, blob: await (await fetch(`./sys/dev/${file.data}`)).blob() };
      } else {
        globalThis.initfs[name] = { mtimeMs: file.mtimeMs, blob: await (await fetch(`./~init/${name}`)).blob() };
      }
    };

    // allow loading concurrently
    let loads = [];
    for(const property in globalThis.initdata) {
      loads.push(load(property, globalThis.initdata[property]));
    }
    await Promise.all(loads);

    globalThis.duplex = await import(URL.createObjectURL(initfs["duplex.js"].blob));
    globalThis.task = await import(URL.createObjectURL(initfs["task.js"].blob));

    globalThis.sys = new task.Task(initfs);
    
    // start kernel
    console.log("Starting kernel...")
    await sys.exec("kernel");

    // load host API
    await import(URL.createObjectURL(initfs["host.js"].blob));

  })();
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
      url.hostname !== "localhost" || // TODO: something else to allow cross-domain requests
      url.pathname === basePath ||
      url.pathname.startsWith(`${basePath}wanix-bootloader.js`) ||
      url.pathname.startsWith(`${basePath}sys/dev`) || 
      url.pathname.startsWith(`${basePath}bootloader`) || 
      url.pathname.startsWith(`${basePath}index.html`) ||
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
