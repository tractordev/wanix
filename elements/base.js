import { WanixKernel } from "./kernel.js";

/** Elements that may own a kernel and/or declare a namespace. */
export const NAMESPACE_HOSTS = [
    "WANIX-NAMESPACE",
    "WANIX-TASK",
    "WANIX-VM",
    "WANIX-WORKBENCH",
    "WANIX-TERM",
];

/** Hosts that own a resource namespace (not merely viewers). */
export const RESOURCE_HOSTS = [
    "WANIX-NAMESPACE",
    "WANIX-TASK",
    "WANIX-VM",
];

/** Viewer hosts apply child binds to the root namespace when they own the kernel. */
export const VIEWER_HOSTS = [
    "WANIX-WORKBENCH",
    "WANIX-TERM",
];

/**
 * Parse the `ns` attribute.
 * Missing → "." (inherit). Present empty string → "" (empty namespace).
 */
export function parseNsAttribute(el) {
    if (!el.hasAttribute("ns")) return ".";
    return el.getAttribute("ns");
}

export function isNamespaceHost(el) {
    return el && NAMESPACE_HOSTS.includes(el.tagName);
}

export function isResourceHost(el) {
    return el && RESOURCE_HOSTS.includes(el.tagName);
}

export function closestNamespaceHost(from) {
    let el = from;
    while (el) {
        if (isNamespaceHost(el)) return el;
        el = el.parentElement;
    }
    return null;
}

export class WanixElement extends HTMLElement {
    constructor() {
        super();
        this._taskpath = "#task";
        this._termpath = "#term";
        this._vmpath = "#vm";
        this._kernel = null;
        this._kernelHost = null;
        this._connectStarted = false;
        this._activated = false;
        this._nsReady = new Promise((resolve) => {
            this._resolveNsReady = resolve;
        });
        this._kernelReady = null;
    }

    get root() {
        return this._kernel?.root;
    }

    openHandle(tid) {
        return this._kernel.openHandle(tid);
    }

    connectedCallback() {
        if (this.hasAttribute("task-ns")) {
            this._taskpath = this.getAttribute("task-ns");
        }
        if (this.hasAttribute("term-ns")) {
            this._termpath = this.getAttribute("term-ns");
        }
        if (this.hasAttribute("vm-ns")) {
            this._vmpath = this.getAttribute("vm-ns");
        }

        queueMicrotask(() => this._connect());
    }

    disconnectedCallback() {
        if (this._kernelHost === this && this._kernel) {
            this._kernel.dispose();
            this._kernel = null;
        }
    }

    async _connect() {
        if (this._connectStarted) return;
        this._connectStarted = true;

        try {
            const kernelHost = this._resolveKernelHost();
            this._kernelHost = kernelHost;

            if (kernelHost === this) {
                await this._bootstrapKernel();
            } else {
                if (!kernelHost._kernelReady) {
                    // Parent host has not started yet; wait for it to expose readiness.
                    await new Promise((resolve) => {
                        const check = () => {
                            if (kernelHost._kernelReady) {
                                resolve();
                            } else {
                                queueMicrotask(check);
                            }
                        };
                        check();
                    });
                }
                this._kernel = kernelHost._kernel;
                await kernelHost._kernelReady;
            }

            const parent = closestNamespaceHost(this.parentElement);
            if (parent && parent !== this && isResourceHost(parent)) {
                await parent._nsReady;
            }

            await this._activate();
        } catch (err) {
            console.error(`${this.tagName.toLowerCase()}: activation failed`, err);
            this.dispatchEvent(
                new CustomEvent("error", {
                    detail: { error: err },
                    bubbles: true,
                }),
            );
            throw err;
        } finally {
            this._resolveNsReady();
        }
    }

    _resolveKernelHost() {
        if (this.hasAttribute("for")) {
            const el = document.getElementById(this.getAttribute("for"));
            if (!el || !isNamespaceHost(el)) {
                throw new Error("for must reference a wanix namespace host element");
            }
            return el;
        }
        // Outermost namespace host in the ancestor chain owns the kernel.
        let host = isNamespaceHost(this) ? this : null;
        let el = this.parentElement;
        while (el) {
            if (isNamespaceHost(el)) host = el;
            el = el.parentElement;
        }
        if (!host) {
            throw new Error(`${this.tagName.toLowerCase()} must be inside a namespace host or use for=`);
        }
        return host;
    }

    async _bootstrapKernel() {
        this._kernel = new WanixKernel(this);
        // Expose before any await so children can wait on it.
        this._kernelReady = this._runKernel();
        await this._kernelReady;
    }

    async _runKernel() {
        await this._kernel.start();

        // Explicit namespace and viewer hosts configure the root namespace.
        // Resource hosts (task/vm) apply binds to their own namespace in _awake.
        if (this.tagName === "WANIX-NAMESPACE" || VIEWER_HOSTS.includes(this.tagName)) {
            const binds = this.querySelectorAll(":scope > wanix-bind");
            await this._kernel._setupNamespace("1", "", binds);
        }

        this._kernel.markReady();
        this.dispatchEvent(
            new CustomEvent("ready", {
                bubbles: true,
            }),
        );
    }

    async _activate() {
        if (this._activated) return;
        this._activated = true;
        await this._awake();
    }

    async _awake() {}

    /** Direct child wanix-bind elements declaring this host's namespace. */
    _childBinds() {
        return this.querySelectorAll(":scope > wanix-bind");
    }
}
