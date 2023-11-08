if (!globalThis["ServiceWorkerGlobalScope"]) {

  // bootloader starts here
  (async function() {
    const load = async (path) => (await (await fetch(path)).blob());
    globalThis.initfs = {
      "duplex.js":    await load("/~dev/kernel/web/lib/duplex.js"),
      "worker.js":    await load("/~dev/kernel/web/lib/worker.js"),
      "syscall.js":   await load("/~dev/kernel/web/lib/syscall.js"),
      "task.js":      await load("/~dev/kernel/web/lib/task.js"),
      "wasm.js":      await load("/~dev/kernel/web/lib/wasm.js"),
      "host.js":      await load("/~dev/kernel/web/lib/host.js"),
      "kernel":       await load("/~dev/bin/kernel"),
      "shell":        await load("/~dev/bin/shell"),
      "indexedfs.js": await load("/~dev/internal/indexedfs/indexedfs.js"),
    };
    
    globalThis.duplex = await import(URL.createObjectURL(initfs["duplex.js"]));
    globalThis.task = await import(URL.createObjectURL(initfs["task.js"]));

    globalThis.sys = new task.Task(initfs);
    
    // start kernel
    await sys.init("kernel");

    // load host API
    await import(URL.createObjectURL(initfs["host.js"]));

    
    // registers Service Worker using this file (see below) if none is registered,
    // and sets up fetcher as a mechanism to fullfill requests
    async function setupServiceWorker(fetcher) {
      let registration = await navigator.serviceWorker.getRegistration("/");
      if (!registration) {
        await navigator.serviceWorker.register("/bootloader.dev.js?sw", {type: "module"});
        registration = await navigator.serviceWorker.ready;  
      }
      registration.active.postMessage({init: true});
      navigator.serviceWorker.addEventListener("message", async (event) => {
        if (event.data.request) {
          const response = await fetcher(event.data.request);
          registration.active.postMessage({response});
        }
      });
    }

    // uses above to set up Service Worker using kernel's sys.request as fetcher
    setupServiceWorker((req) => new Promise(async (resolve) => {
      const resp = await sys.call("web.request", [req.method, req.url])
      const headers = resp.value;
      const ch = resp.channel;
      const buf = new duplex.Buffer();

      await duplex.copy(buf, ch);
      ch.close();

      resolve({ body: buf.bytes(), headers })
    }));

  })();
}

// this file is also used as the Service Worker source. 
// below is ignored unless in a Service Worker.
if (globalThis["ServiceWorkerGlobalScope"] && self instanceof ServiceWorkerGlobalScope) {
  let kernel = undefined;
  let responder = undefined;
  
  self.addEventListener("message", (event) => {
    if (event.data.init) {
      kernel = event.source;
      return;
    }
    if (responder && event.data.response) {
      responder(event.data.response);
    }
  });

  self.addEventListener("fetch", async (event) => {
    const req = event.request;
    const url = new URL(req.url);
    if (url.pathname === "/" ||
      url.pathname.startsWith("/~") || 
      url.pathname.startsWith("/bootloader") || 
      url.pathname.startsWith("/index.html") ||
      !kernel) return;

    const headers = {}
    for (var p of req.headers) {
      headers[p[0]] = p[1]
    }

    const response = new Promise(resolve => {
      responder = resolve;
    });
    event.respondWith(new Promise(async (resolve) => {
      kernel.postMessage({request: {method: req.method, url: req.url, headers: headers }});
      const reply = await response;
      resolve(new Response(reply.body, {headers: reply.headers}))
    }))
  });
  
  self.addEventListener('activate', event => {
    event.waitUntil(clients.claim());
  });

}
