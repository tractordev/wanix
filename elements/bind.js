
export class BindElement extends HTMLElement {
    constructor() {
        super();
    }

    connectedCallback() {
        this.style.display = "none";

        this.dst = this.getAttribute('dst');
        this.src = this.getAttribute('src') || null;
        this.perm = this.getAttribute('perm') || "0644";
        this.union = this.getAttribute('union') || "after";
        this.type = this.getAttribute('type') || "ns";
        this.opts = Object.fromEntries(
            Object.entries(this.dataset).map(([k, v]) => [
                k.startsWith("opt-") ? k.slice(4) : k,
                v
            ])
        );
   
        // this.trim = this.hasAttribute('trim');


        switch (this.type) {
        case "archive":
            this.data = new Promise((resolve, reject) => {
                fetchArchive(this.src).then(data => {
                    resolve(data);
                }).catch(err => {
                    console.error("Failed to fetch archive", this.src, err);
                    reject(err);
                });
            });
            break;
        case "fetch": // deprecated, use "file" instead
        case "file":
            if (this.src) {
                this.data = new Promise((resolve, reject) => {
                    fetch(this.src).then(resp => {
                        if (!resp.ok) {
                            reject(new Error(`HTTP ${resp.status}: ${resp.statusText}`));
                        }
                        resolve(resp.body);
                    }).catch(err => {
                        console.error("Failed to fetch", this.src, err);
                        reject(err);
                    });
                });
            } else {
                // no src, use the text content as the data
                this.data = new Promise((resolve, reject) => {
                    // todo: proper trim handling? dedenting? tricky issue..
                    resolve(new Response(this.innerText.trim()+"\n").body);
                });
            }
            break;
        case "import":
            if (this.src.startsWith("ws://") || this.src.startsWith("wss://")) {
                this.import = new Promise((resolve, reject) => {
                    const ws = new WebSocket(this.src);
                    ws.onopen = () => {
                        resolve(websocketToMessagePort(ws));
                    };
                });
                break;
            }
            this.import = new Promise((resolve, reject) => {
                const iframe = document.createElement('iframe');
                iframe.style.display = "none";
                iframe.src = this.src;
                iframe.onload = () => {
                    try {
                        const ch = new MessageChannel();
                        iframe.contentWindow.postMessage({
                            request: "wanix-import",
                            responder: ch.port2,
                        }, "*", [ch.port2]);
                        ch.port1.onmessage = (event) => {
                            resolve(event.data);
                        };
                    } catch (err) {
                        reject(err);
                    }
                };
                iframe.onerror = (err) => {
                    reject(err);
                };
                document.body.appendChild(iframe);
            });
    
            break;
        }
    }
}

if (typeof window !== "undefined") {
    customElements.define("wanix-bind", BindElement);
}



// fetch an archive and return a tar stream, decompressing if necessary
async function fetchArchive(url) {
    const res = await fetch(url);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);

    const reader = res.body.getReader();

    // Accumulate until we have enough bytes to classify (tar needs 262+)
    const prefixChunks = [];
    let prefixLen = 0;
    const NEEDED = 512; // one tar block

    while (prefixLen < NEEDED) {
        const { value, done } = await reader.read();
        if (done) break;
        prefixChunks.push(value);
        prefixLen += value.byteLength;
    }

    // Flatten prefix
    const prefix = new Uint8Array(prefixLen);
    let off = 0;
    for (const c of prefixChunks) { prefix.set(c, off); off += c.byteLength; }

    // Detect gzip magic number
    const isGzip = prefix[0] === 0x1f && prefix[1] === 0x8b;

    // Rebuild a stream: emit the prefix, then pipe the rest of the reader
    const baseBody = new ReadableStream({
        start(controller) {
            controller.enqueue(prefix);
        },
        async pull(controller) {
            const { value, done } = await reader.read();
            if (done) controller.close();
            else controller.enqueue(value);
        },
        cancel(reason) { reader.cancel(reason); }
    });

    if (!isGzip) {
        // if not gzip, just return the raw stream (tar or otherwise)
        return baseBody;
    } else {
        // if gzip, decompress and return the decompressed stream (assumed tar)
        // Use DecompressionStream if available
        if (typeof DecompressionStream === "undefined") {
            throw new Error("Gzip archives require DecompressionStream support in this browser");
        }
        return baseBody.pipeThrough(new DecompressionStream("gzip"));
    }
}

function websocketToMessagePort(ws) {
    // Create a MessageChannel and grab one port to return
    const { port1, port2 } = new MessageChannel();

    // Forward messages from WebSocket to port1
    ws.onmessage = (event) => {
        if (event.data instanceof Blob) {
            event.data.arrayBuffer().then(arr => {
                const buf = new Uint8Array(arr);
                // Only ArrayBuffer is transferable, not TypedArray views like Uint8Array
                port1.postMessage(buf, [buf.buffer]);
            });
            return;
        } else {
            console.warn("Unsupported data type", event.data);
        }
    };
    ws.onclose = () => port1.close();

    // Forward postMessage on port1 to WebSocket send
    port1.onmessage = (event) => {
        // Binary or string: just forward
        ws.send(event.data);
    };

    port1.onclose = () => {
        try { ws.close(); } catch(e) {}
    };

    return port2;
}