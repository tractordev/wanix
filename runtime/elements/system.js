import { WanixRuntime } from "../runtime.js";

export class System extends HTMLElement {
    constructor() {
        super();
        this.runtime = null;
        this._readyPromise = null;
    }

    connectedCallback() {
        const config = this._getConfigFromAttributes();
        
        this.runtime = new WanixRuntime(config);
        
        this._readyPromise = this.runtime.ready().then(() => {
            this.dispatchEvent(new CustomEvent("ready", { 
                detail: { runtime: this.runtime },
                bubbles: true 
            }));
            return this.runtime;
        });
    }

    disconnectedCallback() {
        this.runtime = null;
        this._readyPromise = null;
    }

    _getConfigFromAttributes() {
        const config = {};

        if (this.hasAttribute("wasm")) {
            config.wasm = this.getAttribute("wasm");
        }
        if (this.hasAttribute("bundle")) {
            config.bundle = this.getAttribute("bundle");
        }
        if (this.hasAttribute("network")) {
            config.network = this.getAttribute("network");
        }
        if (this.hasAttribute("helpers")) {
            config.helpers = this.getAttribute("helpers") !== "false";
        }
        if (this.hasAttribute("export9p")) {
            config.export9p = this.getAttribute("export9p") !== "false";
        }
        if (this.hasAttribute("screen")) {
            config.screen = this.getAttribute("screen") !== "false";
        }
        if (this.hasAttribute("debug9p")) {
            config.debug9p = this.getAttribute("debug9p") !== "false";
        }

        return config;
    }

    ready() {
        return this._readyPromise;
    }

    createPort() {
        return this.runtime?.createPort();
    }

    get peer() {
        return this.runtime?.peer;
    }

    async readDir(name) {
        return this.runtime?.readDir(name);
    }

    async makeDir(name) {
        return this.runtime?.makeDir(name);
    }

    async makeDirAll(name) {
        return this.runtime?.makeDirAll(name);
    }

    async bind(name, newname) {
        return this.runtime?.bind(name, newname);
    }

    async unbind(name, newname) {
        return this.runtime?.unbind(name, newname);
    }

    async readFile(name) {
        return this.runtime?.readFile(name);
    }

    async readText(name) {
        return this.runtime?.readText(name);
    }

    async writeFile(name, contents) {
        return this.runtime?.writeFile(name, contents);
    }

    async appendFile(name, contents) {
        return this.runtime?.appendFile(name, contents);
    }

    async stat(name) {
        return this.runtime?.stat(name);
    }

    async waitFor(name, timeoutMs) {
        return this.runtime?.waitFor(name, timeoutMs);
    }

    async rename(oldname, newname) {
        return this.runtime?.rename(oldname, newname);
    }

    async copy(oldname, newname) {
        return this.runtime?.copy(oldname, newname);
    }

    async remove(name) {
        return this.runtime?.remove(name);
    }

    async removeAll(name) {
        return this.runtime?.removeAll(name);
    }

    async open(name) {
        return this.runtime?.open(name);
    }

    async create(name) {
        return this.runtime?.create(name);
    }

    async openReadable(name) {
        return this.runtime?.openReadable(name);
    }

    async openWritable(name) {
        return this.runtime?.openWritable(name);
    }
}

if (typeof window !== "undefined") {
    customElements.define("wanix-system", System);
}
