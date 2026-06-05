import { WanixElement } from "./base.js";
import { TaskElement } from "./task.js";

export class VMElement extends WanixElement {
    constructor() {
        super();
        this.rid = null;
        this.task = new TaskElement();
    }

    get path() {
        if (!this.rid) {
            throw new Error('VM not allocated');
        }
        return [this._vmpath, this.rid].join("/");
    }

    connectedCallback() {
        super.connectedCallback();

        this.alias = this.getAttribute('alias') || this.getAttribute('id') || null;
        this.type = this.getAttribute('type') || "v86";
        this.fsys = this.getAttribute('fsys');
        this._term = this.hasAttribute('term');
        this._autostart = this.hasAttribute('start');

        const args = [];
        // other than export, these all map to qemu cli flags.
        // append is put in env because of shell quoting limitations atm
        ['export', 'mem', 'vga-mem', 'hda', 'hdb', 'fda', 'fdb', 'cdrom', 'boot', 'bios', 'acpi', 'fastboot', 'kernel', 'initrd', 'netdev', 'virtfs'].forEach(attr => {
            if (this.hasAttribute(attr)) {
                args.push(`-${attr} ${this.getAttribute(attr)}`);
            }
        });

        this.task._system = this._system;
        this.task.type = "gojs";
        if (this.hasAttribute('append')) {
            this.task.env = `VM_APPEND=${this.getAttribute('append')}\n`;
        } else {
            this.task.env = "";
        }
        this.task.cmd = `#vm/${this.type}/${this.type}-vm.wasm ${args.join(" ")}`;
    }

    async _awake() {
        await this.allocate();
        if (this._autostart) {
            this.start();
        }
    }

    async allocate() {
        if (this.rid) {
            throw new Error('VM already allocated');
        }
        this.rid = (await this._system.root.readText([this._vmpath, "new", this.type].join("/"))).trim();
        if (this.id) {
            await this._system.root.writeFile([this.path, "alias"].join("/"), this.id);
        }

        this.task.env += `vm=${this.rid}\n`;
        await this.task.allocate(this.querySelectorAll(':scope > wanix-bind'));

        if (this._term) {
            const termID = (await this._system.root.readText([this._termpath, "new"].join("/"))).trim();
            this.term = [this._termpath, termID].join("/");
            // should this binding be done in task vfs?
            await this._system.root.bind(this.term, [this.path, "term"].join("/"));
            // this is def a hack, but it works for now.
            // this is in addition to the above since aliased path needs its own binding.
            if (this.alias) {
                await this._system.root.bind(this.term, [this._vmpath, this.alias, "term"].join("/"));
            }

            // otherwise it'll point to task 1 being cloned from root
            await this.task.root.bind(this.term, `${this._taskpath}/self/term`);

            const program = [this.term, "program"].join("/");
            await this.task.root.bind(program, [this.task.path, "fd/0"].join("/"));
            await this.task.root.bind(program, [this.task.path, "fd/1"].join("/"));
            await this.task.root.bind(program, [this.task.path, "fd/2"].join("/"));
            
        }

    }

    async start() {
        await this.task.start();
        console.log('vm started', this.rid, this.id);
    }
}

if (typeof window !== "undefined") {
    customElements.define("wanix-vm", VMElement);
}


