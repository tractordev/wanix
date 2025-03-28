
let cache = undefined;
self.addEventListener("activate", event => {
    event.waitUntil(clients.claim());
    console.log("ServiceWorker: activated:", self.location.href);
    caches.open("v0").then(c => {
        cache = c;
        cache.addAll([
            "/",
            "/index.html",

            // goal:
            // "/wanix-0.3.min.js",
            // "/wanix-0.3.sw.js",
            // "/wanix-0.3.wasm",

            "/wanix.css",
            "/wanix.bundle.js",
            "/wanix.wasm",
        ]);
    });
});

let listener = undefined;
self.addEventListener("message", (event) => {
    if (event.data.listen) {
        listener = event.data.listen;
        console.log("ServiceWorker: handler registered");
        return;
    }
});

self.addEventListener("fetch", async (event) => {
    if (!listener) {
        console.warn("ServiceWorker: no handler registered");
        return;
    }
    if (!cache) {
        cache = await caches.open("v0");
    }

    const req = event.request;
    
    event.respondWith(cache.match(req).then(async (resp) => {
        if (resp) {
            // console.log("ServiceWorker: cached", req.url);
            return resp;
        }

        resp = await handleRequest(req);
        if (resp) {
            cache.put(req, resp.clone());
            // console.log("ServiceWorker: cache-put", req.url);
        }

        return resp;
    }));

});


function timeout(ms) {
    return new Promise((_, reject) => setTimeout(() => reject(new Error('Timeout')), ms));
}

function fetchBypass(request) {
    // Create a new request with mode 'no-cors' to bypass the service worker
    const newRequest = new Request(request.url, {
        method: request.method,
        headers: request.headers,
        mode: 'no-cors',
    });
    return fetch(newRequest);
}

async function handleRequest(req) {
    const url = new URL(req.url);
    const headers = {};
    for (var p of req.headers) {
        headers[p[0]] = p[1];
    }
    headers["X-Service-Worker"] = self.location.href;

    const ch = new MessageChannel();
    const response = new Promise(respond => {
        ch.port1.onmessage = (event) => respond(event.data);
    });
    listener.postMessage({
        request: {
            method: req.method, 
            url: req.url, 
            headers: headers,
            host: url.host,
            hostname: url.hostname,
            pathname: url.pathname,
            port: url.port,
        },
        responder: ch.port2
    }, [ch.port2]);
    try {
        const reply = await Promise.race([response, timeout(1000)]);

        if (reply.error) {
            throw new Error(reply.error);
        }

        if (reply.status === 100) {
            return fetchBypass(req);
        }

        return new Response(reply.body, reply);
        
    } catch (error) {
        if (error.message === 'Timeout') {
            listener = undefined;
        } else {
            console.warn("ServiceWorker:", error);
        }
        return fetchBypass(req);
    }
}