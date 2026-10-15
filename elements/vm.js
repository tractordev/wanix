import { WanixElement, parseNsAttribute } from "./base.js";
import { TaskElement } from "./task.js";

export class VMElement extends WanixElement {
    constructor() {
        super();
        this.rid = null;
        this.task = new TaskElement();
        this.etherNS = null;
        this.etherChan = "0";
    }

    get path() {
        if (!this.rid) {
            throw new Error("VM not allocated");
        }
        return [this._vmpath, this.rid].join("/");
    }

    connectedCallback() {
        super.connectedCallback();

        this.style.display = "contents";

        this.alias = this.getAttribute("alias") || this.getAttribute("id") || null;
        this.type = this.getAttribute("type") || "v86";
        this.ns = parseNsAttribute(this);
        this._term = this.hasAttribute("term");
        this._autostart = this.hasAttribute("start");

        const args = [];
        // other than export, these all map to qemu cli flags.
        // append is put in env because of shell quoting limitations atm
        [
            "export", "mem", "vga-mem", "hda", "hdb", "fda", "fdb", "cdrom",
            "boot", "bios", "acpi", "fastboot", "kernel", "initrd", "netdev", "virtfs",
        ].forEach((attr) => {
            if (this.hasAttribute(attr)) {
                let val = this.getAttribute(attr);
                if (attr === "netdev" && val.startsWith("user,")) {
                    val.split(",").forEach(part => {
                        const [key, value] = part.split("=");
                        if (key === "ether") {
                            this.etherNS = value.startsWith("/") ? value.slice(1) : value;
                            val += ",relay_url=inbrowser";
                        }
                        if (key === "id") {
                            this.etherChan = value;
                        }
                    });
                }
                args.push(`-${attr} ${val}`);
            }
        });

        this.task.type = "gojs";
        if (this.hasAttribute("append")) {
            this.task.env = `VM_APPEND=${this.getAttribute("append")}\n`;
        } else {
            this.task.env = "";
        }
        this.task.cmd = `#vm/${this.type}/${this.type}-vm.wasm ${args.join(" ")}`;
    }

    async _awake() {
        await this.allocate();
        if (this.etherNS) {
            let datafd = undefined;
            const root = this._kernel.root;
            // bug: not sure etherChan gets to v86 configuration correctly
            const vnic = new BroadcastChannel(`v86-inbrowser-${this.etherChan}`);
            vnic.onmessage = async (event) => {
                if (!datafd) {
                    const fd = await root.open(`${this.etherNS}/clone`);
                    const id = (new TextDecoder()).decode(await root.read(fd, 8)).trim();
                    datafd = await root.openFile(`${this.etherNS}/${id}/data`, 2, 0);
                    (async () => {
                        const stream = await root.readable(datafd);
                        for await (const chunk of stream) {
                            vnic.postMessage(chunk);
                        }
                    })();
                }
                await root.write(datafd, event.data);
            };
        }
        if (this._autostart) {
            await this.start();
        }
    }

    async allocate() {
        if (this.rid) {
            throw new Error("VM already allocated");
        }
        this.rid = (await this._kernel.root.readText([this._vmpath, "new", this.type].join("/"))).trim();
        if (this.id) {
            await this._kernel.root.writeFile([this.path, "alias"].join("/"), this.id);
        }

        // Backing task is not in the DOM; wire kernel and ns manually.
        this.task._kernel = this._kernel;
        this.task._taskpath = this._taskpath;
        this.task._termpath = this._termpath;
        this.task.ns = this.ns;
        this.task.env += `vm=${this.rid}\n`;
        await this.task.allocate(this._childBinds());

        if (this._term) {
            const termID = (await this._kernel.root.readText([this._termpath, "new"].join("/"))).trim();
            this.term = [this._termpath, termID].join("/");
            await this._kernel.root.bind(this.term, [this.path, "term"].join("/"));
            if (this.alias) {
                await this._kernel.root.bind(this.term, [this._vmpath, this.alias, "term"].join("/"));
            }

            await this.task.taskRoot.bind(this.term, `${this._taskpath}/self/term`);

            const program = [this.term, "program"].join("/");
            await this.task.taskRoot.bind(program, [this.task.path, "fd/0"].join("/"));
            await this.task.taskRoot.bind(program, [this.task.path, "fd/1"].join("/"));
            await this.task.taskRoot.bind(program, [this.task.path, "fd/2"].join("/"));
        }
    }

    async start() {
        await this.task.start();
        console.log("vm started", this.rid, this.id);
    }
}

if (typeof window !== "undefined") {
    customElements.define("wanix-vm", VMElement);
}
