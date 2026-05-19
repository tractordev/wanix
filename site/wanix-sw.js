/**
 * This file provides a generic message-based service worker infrastructure for custom HTTP-like request/response
 * handling between the main thread (web clients) and a service worker.
 * 
 * STILL IN DEVELOPMENT, DO NOT USE IN PRODUCTION
 * 
 * Key Features:
 * --------------
 * - Instantly claims clients and skips waiting (`install` and `activate` logic).
 * - Maintains a per-client registration so the SW can coordinate with responder endpoints in each client.
 * - For requests whose pathname begins with the configured `prefix` (default `/`), the SW:
 *   - Finds the registered responder for the **same** client as the fetch (`event.clientId`). It does not fall
 *     back to another tab’s registration (that produced timeouts after reload when messages went to a dead port).
 *   - If this client has not registered yet (e.g. a fetch before `register()` resolves), the worker uses `fetch`
 *     on the original request instead of messaging.
 *   - Forwards the fetch metadata (method, URL, headers) to the registered responder via a MessageChannel.
 *   - Waits for a reply or timeout, then returns the reply (with headers/body/status) as a `Response` object.
 *   - If the page handler returns `null` or `undefined`, the worker performs `fetch(event.request)` (real network).
 *     This is done in the service worker so the request does not re-enter the message handler and recurse.
 *   - On error or timeout returns a network error response.
 * - The `register(options)` function is designed to be called on the client side:
 *   - It sets up a local responder that can handle SW requests by running `options.onfetch(Request)`.
 *   - It registers the responder with the SW via `postMessage`, and handles communication using MessageChannel ports.
 *   - It allows request routing to be narrowed with `options.prefix` or left at the default `/` to intercept
 *     all requests inside the service worker scope.
 * 
 * Intended Usage:
 * ---------------
 * 1. On the main thread, call `register(options)` to provide a custom request handler for SW requests.
 * 2. Any fetch whose pathname starts with `options.prefix` will go through the SW, which relays the request via
 *    messages to the registered handler, receives the result, and responds to the fetch.
 * 3. Supports advanced scenarios like mocking server APIs, on-the-fly content, offline logic, etc.
 * 
 * Notes:
 * ------
 * - `options.prefix` defaults to `/`, so requests are intercepted by default for the entire service worker scope.
 * - To keep interception opt-in, pass a narrower prefix such as `/:/`.
 * - All communication uses MessageChannel ports for structured, race-condition-safe messaging.
 * - Timeout, error, and registration logic are handled to ensure robust messaging regardless of page state.
 * - A broad `prefix` (e.g. `/`) means every matching asset pays a page round-trip; handlers that often return
 *   `null` for static URLs should use a narrow `prefix` (e.g. `/api/`) so HTML/CSS/JS skip the bridge.
 * - The code is ES module-compatible.
 * 
 * Options:
 * --------
 * - `onfetch` (`(Request) => Response | null | undefined`): Page handler for intercepted fetches.
 * - `oninstall` (`() => void | Promise<void>`, optional): Runs during the worker `install` event before `skipWaiting()`.
 * - `onactivate` (`() => void | Promise<void>`, optional): Runs during the worker `activate` event before `clients.claim()`.
 * - `prefix` (`string`): Pathname prefix to intercept. Defaults to `/`.
 * - `scope` (`string`): Service worker registration scope. Defaults to `/`.
 * - `timeout` (`number`): Milliseconds to wait for the handler before returning a network error. Defaults to `1000`.
 * 
 * Example (client page):
 * ----------------------
 *   import { register } from '/sw.js';
 *   await register({
 *     onfetch: async (req) => {
 *       // handle Request, return a Response
 *       return new Response("Hello!");
 *     },
 *   }); // intercepts all requests under the service worker scope
 *
 *   const resp = await fetch('/api/data'); // routed by service worker to your handler
 *
 *   // Use a custom prefix to keep interception opt-in and preserve the old `/:/` convention.
 *   await register({ onfetch: handler, prefix: '/:/' });
 *   const scopedResp = await fetch('/:/example'); // only prefixed requests are routed
 */
const skipHeader = "x-skip-wanix-sw";

if (globalThis["ServiceWorkerGlobalScope"] && self instanceof ServiceWorkerGlobalScope) {
    const registered = new Map();
    const lifecycle = new Map();
    let registrationSeen = false;
    let lifecycleExpected = false;

    async function waitForLifecyclePorts(maxMs = 5000) {
        if (lifecycle.size > 0) return;
        const deadline = Date.now() + maxMs;
        while (Date.now() < deadline) {
            if (lifecycle.size > 0) return;
            if (registrationSeen && !lifecycleExpected) return;
            await new Promise((r) => setTimeout(r, 50));
        }
    }

    async function invokeLifecycleHooks(phase) {
        if (lifecycle.size === 0) return;
        const tasks = [];
        for (const port of lifecycle.values()) {
            tasks.push(
                new Promise((resolve) => {
                    const t = setTimeout(resolve, 10000);
                    port.onmessage = () => {
                        clearTimeout(t);
                        resolve();
                    };
                    port.postMessage({ phase });
                })
            );
        }
        await Promise.allSettled(tasks);
    }

    async function cleanupDeadClients() {
        const clientsToDelete = [];
        for (const clientId of registered.keys()) {
            const client = await clients.get(clientId);
            if (!client) clientsToDelete.push(clientId);
        }
        for (const clientId of clientsToDelete) {
            registered.delete(clientId);
            lifecycle.delete(clientId);
        }
    }

    async function requestFromHandler(registration, req) {
        const { timeout = 1000 } = registration.options;
        const headers = {};
        for (var p of req.headers) {
            headers[p[0]] = p[1];
        }

        const ch = new MessageChannel();
        const reply = await new Promise((resolve, reject) => {
            const t = setTimeout(() => reject(new Error("Timeout")), timeout);
            ch.port1.onmessage = (e) => {
                clearTimeout(t);
                resolve(e.data);
            };
            registration.responder.postMessage(
                {
                    request: {
                        method: req.method,
                        url: req.url,
                        headers: headers,
                    },
                    responder: ch.port2,
                },
                [ch.port2]
            );
        });
        if (reply.error) {
            throw new Error(reply.error);
        }
        if (reply.passthrough) {
            // Network from the worker is not routed back through this worker’s fetch listener (no handler loop).
            return fetch(req);
        }
        return new Response(reply.body, reply);
    }

    self.addEventListener("install", (event) =>
        event.waitUntil(
            (async () => {
                await waitForLifecyclePorts();
                await invokeLifecycleHooks("install");
                self.skipWaiting();
            })()
        )
    );
    self.addEventListener("activate", (event) =>
        event.waitUntil(
            (async () => {
                await waitForLifecyclePorts();
                await invokeLifecycleHooks("activate");
                await clients.claim();
                await cleanupDeadClients();
            })()
        )
    );

    self.addEventListener("message", (event) => {
        if (event.data.responder) {
            const task = (async () => {
                await cleanupDeadClients();
                registrationSeen = true;
                lifecycleExpected = event.data.lifecycleExpected;
                registered.set(event.source.id, {clientId: event.source.id, ...event.data});
                if (event.data.lifecycle) {
                    lifecycle.set(event.source.id, event.data.lifecycle);
                }
                event.data.ready.postMessage({ ok: true });
            })().catch((error) => {
                console.error(error);
                event.data.ready.postMessage({ error: error.message || String(error) });
            });
            event.waitUntil?.(task);
        }
    });

    self.addEventListener("fetch", (event) => {
        const req = event.request;
        const url = new URL(req.url);

        event.respondWith((async () => {
            // Only use this fetch's client — never another tab's MessagePort (reload gives a new clientId).
            let registration = registered.get(event.clientId);
            if (!registration) {
                const client = event.clientId ? await clients.get(event.clientId) : null;
                // Iframe navigations often use a different clientId than the document that called
                // register(), and sometimes report an empty clientId until commit.
                const shareParentHandler =
                    !event.clientId || !client || client.frameType === "nested";
                if (shareParentHandler) {
                    for (const clientId of registered.keys()) {
                        const owner = await clients.get(clientId);
                        if (owner?.frameType === "top-level") {
                            registration = registered.get(clientId);
                            break;
                        }
                    }
                }
            }
            if (!registration) {
                return fetch(req);
            }

            const { prefix = "/" } = registration.options;
            if (!url.pathname.startsWith(prefix) || req.headers.get(skipHeader) === "1") {
                return fetch(req);
            }

            try {
                return await requestFromHandler(registration, req);
            } catch (error) {
                console.error(error);
                return Response.error();
            }
        })());
    });

} 

export async function networkFetch(path) {
    return fetch(new URL(path, location.origin), {
        headers: { [skipHeader]: "1" },
    });
}

export async function templateFetch(req) {
    const url = new URL(req.url);
    for (const template of document.querySelectorAll("template[data-path]")) {
      if (template.dataset.path === url.pathname) {
        return new Response(template.innerHTML, {
          headers: { "Content-Type": template.dataset.type },
        });
      }
    }
    return null;
}


let cachePopulate = null;
export async function ensureCache(name, prefetch=[]) {
  const cache = await caches.open(name);
  if (prefetch.length > 0 && await cache.match(prefetch[0])) return cache;
  if (!cachePopulate) {
    // console.log("cache not populated, populating");
    cachePopulate = Promise.all(
      prefetch.map(async (path) => {
        const res = await networkFetch(path);
        if (res.ok) await cache.put(path, res);
      })
    ).finally(() => {
      cachePopulate = null;
    });
  }
  await cachePopulate;
  return cache;
}

function cacheURL(url) {
  if (url.pathname.endsWith("/")) return new URL(`${url.pathname}index.html`, url.origin);
//   if (CACHE_PATHS.includes(url.pathname)) return url;
  return url;
}

export async function cacheFetch(req, cache="main") {
  const url = cacheURL(new URL(req.url));
  if (!url) return null;
  const c = await ensureCache(cache);
  return (await c.match(url)) || null;
}

/**
 * Register a page-local request handler with the service worker.
 *
 * `options.prefix` controls which request pathnames are intercepted and defaults
 * to `/`, meaning all requests inside the registration scope are eligible.
 * Pass a narrower prefix such as `/:/` to make interception opt-in.
 *
 * Return `null` or `undefined` from `onfetch` to defer to the real network; the
 * service worker performs `fetch(event.request)` so the request does not loop
 * through the handler again.
 *
 * Optional `oninstall` and `onactivate` run during the worker lifecycle (before
 * the built-in `skipWaiting` and `clients.claim` steps). They do not replace those defaults.
 */
export async function register(options = {}) {
    const {
        onfetch,
        oninstall,
        onactivate,
        scope = "/",
        prefix,
        timeout,
    } = options;

    const respondOptions = { prefix, timeout };
    const responder = new MessageChannel();
    const ready = new MessageChannel();

    let handler = onfetch ?? (() => new Response("No handler yet", { status: 503 }));

    let lifecyclePort = null;
    if (oninstall || onactivate) {
        const lifecycle = new MessageChannel();
        lifecyclePort = lifecycle.port2;
        lifecycle.port1.onmessage = async (event) => {
            try {
                if (event.data.phase === "install" && oninstall) await oninstall();
                if (event.data.phase === "activate" && onactivate) await onactivate();
                lifecycle.port1.postMessage({ ok: true });
            } catch (error) {
                lifecycle.port1.postMessage({ error: error.message || String(error) });
            }
        };
    }

    responder.port1.onmessage = async (event) => {
        const req = new Request(event.data.request.url, {
            method: event.data.request.method,
            headers: event.data.request.headers,
            body: event.data.request.body,
        });
        const resp = await handler(req);
        if (resp === null || resp === undefined) {
            // Let the service worker call fetch(originalRequest); page-side fetch would recurse through the SW.
            event.data.responder.postMessage({ passthrough: true });
            return;
        }
        const body =
            typeof resp.bytes === "function"
                ? await resp.bytes()
                : new Uint8Array(await resp.arrayBuffer());
        event.data.responder.postMessage({
            body,
            headers: Object.fromEntries(resp.headers.entries()),
            status: resp.status,
            statusText: resp.statusText,
        });
    };

    const readyResult = new Promise((resolve, reject) => {
        ready.port1.onmessage = (event) => {
            if (event.data?.error) {
                reject(new Error(event.data.error));
                return;
            }
            resolve(event.data);
        };
    });
    const transfer = [responder.port2, ready.port2];
    const message = {
        responder: responder.port2,
        ready: ready.port2,
        options: respondOptions,
        lifecycleExpected: !!(oninstall || onactivate),
    };
    if (lifecyclePort) {
        message.lifecycle = lifecyclePort;
        transfer.push(lifecyclePort);
    }

    const reg = await navigator.serviceWorker.register(import.meta.url, { type: "module", scope });
    const worker = reg.installing || reg.waiting || reg.active;
    if (!worker) throw new Error("service worker has no worker instance");
    worker.postMessage(message, transfer);

    await readyResult;
    await navigator.serviceWorker.ready;

    return (h) => (handler = h);
}
