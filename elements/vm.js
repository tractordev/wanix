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
        
        this.type = this.getAttribute('type') || "v86";
        this.fsys = this.getAttribute('fsys');
        this._term = this.hasAttribute('term');
        this._autostart = this.hasAttribute('start');

        this.mem = this.getAttribute('mem') || "512M";
        this.vgaMem = this.getAttribute('vga-mem') || "8M";

        this.hda = this.getAttribute('hda');
        this.hdb = this.getAttribute('hdb');
        this.fda = this.getAttribute('fda');
        this.fdb = this.getAttribute('fdb');
        this.cdrom = this.getAttribute('cdrom');

        this.boot = this.getAttribute('boot');
        this.bios = this.getAttribute('bios');
        this.acpi = this.hasAttribute('acpi');
        this.fastboot = this.hasAttribute('fastboot');

        this.kernel = this.getAttribute('kernel');
        this.initrd = this.getAttribute('initrd');
        this.append = this.getAttribute('append');

        this.netdev = this.getAttribute('netdev');
        this.virtfs = this.getAttribute('virtfs');

        this.task._system = this._system;
        this.task.type = "gojs";
        this.task.cmd = `#vm/${this.type}/${this.type}-vm.wasm`;
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

        await this.task.allocate(this.querySelectorAll(':scope > wanix-bind'));

        if (this._term) {
            const termID = (await this._system.root.readText([this._termpath, "new"].join("/"))).trim();
            this.term = [this._termpath, termID].join("/");
            // should this binding be done in task vfs?
            await this._system.root.bind(this.term, [this.path, "term"].join("/"));
            // this is def a hack, but it works for now.
            // this is in addition to the above since aliased path needs its own binding.
            if (this.id) {
                await this._system.root.bind(this.term, [this._vmpath, this.id, "term"].join("/"));
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


