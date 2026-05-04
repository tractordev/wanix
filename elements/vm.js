import { NamespaceElement } from "./namespace.js";

export class VMElement extends NamespaceElement {
    constructor() {
        super();
        this.cmd = this.getAttribute('cmd');
        this.type = this.getAttribute('type') || "v86";
        this.mem = this.getAttribute('mem') || "512M";
        this.append = this.getAttribute('append') || "";
        
        this.for = this.getAttribute('for');
        this.fsys = this.getAttribute('fsys');
        this._term = this.hasAttribute('term');
        this.autostart = this.hasAttribute('start');

        this.vmNS = "#vm";
        if (this.hasAttribute('vm-ns')) {
            this.vmNS = this.getAttribute('vm-ns');
        }

        this.taskNS = "#task";
        if (this.hasAttribute('task-ns')) {
            this.taskNS = this.getAttribute('task-ns');
        }

        this.termNS = "#term";
        if (this.hasAttribute('term-ns')) {
            this.termNS = this.getAttribute('term-ns');
        }
    }

    get path() {
        return [this.vmNS, this.rid].join("/");
    }

    connectedCallback() {
        super.connectedCallback();

        if (this.for) {
            this.system = document.getElementById(this.for);
        } else {
            this.system = this.closest('wanix-system');
        }

        if (this.system) {
            this.system.addEventListener('ready', async () => {
                await this.allocate();
                if (this.autostart) {
                    this.start();
                }
            });
        }
    }

    async allocate() {
        if (this.rid) {
            throw new Error('VM already allocated');
        }
        this.rid = (await this.system.root.readText([this.vmNS, "new", this.type].join("/"))).trim();
        
        if (this.id) {
            await this.system.root.writeFile([this.vmNS, this.rid, "alias"].join("/"), this.id);
        }
        if (this._term) {
            const termID = (await this.system.root.readText([this.termNS, "new"].join("/"))).trim();
            this.term = [this.termNS, termID].join("/");
            // should this binding be done in task vfs?
            await this.system.root.bind(this.term, [this.path, "term"].join("/"));
            // this is def a hack, but it works for now.
            // this is in addition to the above since aliased path needs its own binding.
            if (this.id) {
                await this.system.root.bind(this.term, [this.taskNS, this.id, "term"].join("/"));
            }

            const program = [this.term, "program"].join("/");
            await this.system.root.bind(program, [this.path, "ttyS0"].join("/"));
            // await this.root.bind(program, [this.path, "fd/1"].join("/"));
            // await this.root.bind(program, [this.path, "fd/2"].join("/"));
        } else {
            await this.system.root.bind("#web/console", [this.path, "ttyS0"].join("/"));
            // await this.root.bind("#web/console", [this.path, "fd/2"].join("/"));
        }
        
    }

    async start() {
        const ctlcmd = [
            "start",
            "-m", this.mem,
            "-append", `'rw root=host9p rootfstype=9p rootflags=trans=virtio,version=9p2000.L,aname=${this.fsys},cache=none,msize=131072 ${this.append}'`,
        ].join(" ");
        await this.system.root.writeFile([this.vmNS, this.rid, "ctl"].join("/"), ctlcmd);
        console.log('vm start', this, this.rid, this.id);
    }
}

if (typeof window !== "undefined") {
    customElements.define("wanix-vm", VMElement);
}


