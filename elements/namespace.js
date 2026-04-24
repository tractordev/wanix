
export class NamespaceElement extends HTMLElement {
    constructor() {
        super();
        this.__parsed = false;
        this.bindings = new Promise(resolve => {
            this.__resolveBindings = resolve;
        });
        this.files = new Promise(resolve => {
            this.__resolveFiles = resolve;
        });
    }

    connectedCallback() {
        if (this.__parsed) {
            return;
        }
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', () => this.parseNamespace(), { once: true });
        } else {
            this.parseNamespace();
        }
    }

    parseNamespace() {
        this.__parsed = true;
        this.__resolveBindings(this.parseBindings());
        this.__resolveFiles(this.parseFiles());
    }

    parseBindings() {
        const bindings = [...this.querySelectorAll(':scope > wanix-bind')].map(el => ({
            el,
            dst: el.getAttribute('dst'),
            src: el.getAttribute('src'),
            mode: el.getAttribute('mode') || "after",
            type: el.getAttribute('type') || "ns",
        }));
        bindings.forEach(binding => {
            binding.el.style.display = "none";
            switch (binding.type) {
                case "archive":
                    binding.archive = fetchArchive(binding.src);
                    binding.archive.catch(err => {
                        console.error("Failed to fetch archive", binding.src, err);
                        this.dispatchEvent(new CustomEvent("error", {
                            detail: { error: err },
                            bubbles: true
                        }));
                    });
                    break;
                case "fetch":
                    binding.fetch = fetch(binding.src);
                    binding.fetch.catch(err => {
                        console.error("Failed to fetch", binding.src, err);
                        this.dispatchEvent(new CustomEvent("error", {
                            detail: { error: err },
                            bubbles: true
                        }));
                    });
                    break;
            }
        });
        return bindings;
    }

    parseFiles() {
        const files = [...this.querySelectorAll(':scope > wanix-file')].map(el => ({
            el,
            dst: el.getAttribute('dst'),
            mode: el.getAttribute('mode') || "644", // 0644
            encoding: el.getAttribute('encoding') || "utf-8",
            content: el.textContent,
        }));
        files.forEach(file => {
            file.el.style.display = "none";
        });
        return files;
    }
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