
self.addEventListener("activate", event => {
    event.waitUntil(clients.claim());
});

let listener = undefined;
self.addEventListener("message", (event) => {
    if (event.data.listen) {
        listener = event.data.listen;
        console.log("SW: listener installed");
        return;
    }
});

self.addEventListener("fetch", async (event) => {
    if (!listener) return;

    const req = event.request;
    const url = new URL(req.url);
    const headers = {};
    for (var p of req.headers) {
        headers[p[0]] = p[1];
    }
    headers["X-ServiceWorker"] = self.location.href;

    event.respondWith(new Promise(async (resolve) => {
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
            const reply = await Promise.race([response, timeout(2000)]);

            if (reply.error) {
                throw new Error(reply.error);
            }

            if (reply.status === 100) {
                resolve(fetchBypass(req));
                return;
            }

            resolve(new Response(reply.body, reply));
            
        } catch (error) {
            console.warn("SW:", error);
            if (error.message === 'Timeout') {
                listener = undefined;
            }
            resolve(fetchBypass(req));
        }
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