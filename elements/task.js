import { WanixElement, parseNsAttribute } from "./base.js";

export class TaskElement extends WanixElement {
    constructor() {
        super();
        this.rid = null;
    }

    get path() {
        if (!this.rid) {
            throw new Error("Task not allocated");
        }
        return [this._taskpath, this.rid].join("/");
    }

    connectedCallback() {
        super.connectedCallback();

        this.alias = this.getAttribute("alias") || this.getAttribute("id") || null;
        this.type = this.getAttribute("type") || "auto";
        this.role = this.getAttribute("role");
        this.cmd = this.getAttribute("cmd");
        this.env = spaceToNewline(this.getAttribute("env"));
        this.stdout = this.getAttribute("stdout");
        this.stderr = this.getAttribute("stderr");
        this.stdin = this.getAttribute("stdin");
        this.ns = parseNsAttribute(this);
        this._term = this.hasAttribute("term");
        this._autostart = this.hasAttribute("start");
        if (this.hasAttribute("wd")) {
            this.wd = this.getAttribute("wd");
        }
    }

    async _awake() {
        await this.allocate();
        if (this._autostart) {
            await this.start();
        }
    }

    async allocate(bindElements = null) {
        if (this.rid) {
            throw new Error("Task already allocated");
        }
        this.rid = (await this._kernel.root.readText([this._taskpath, "new", this.type].join("/"))).trim();
        this.taskRoot = this._kernel.openHandle(this.rid);

        await this._kernel.root.writeFile([this.path, "cmd"].join("/"), this.cmd);
        if (this.env) {
            await this._kernel.root.writeFile([this.path, "env"].join("/"), this.env);
        }
        if (this.wd) {
            await this._kernel.root.writeFile([this.path, "dir"].join("/"), this.wd);
        }
        if (this.alias) {
            await this._kernel.root.writeFile([this.path, "alias"].join("/"), this.alias);
        }

        // otherwise it'll point to task 1 being cloned from root
        await this.taskRoot.bind(this.path, `${this._taskpath}/self`);

        if (this._term) {
            const termID = (await this._kernel.root.readText([this._termpath, "new"].join("/"))).trim();
            this.term = [this._termpath, termID].join("/");
            await this._kernel.root.bind(this.term, [this.path, "term"].join("/"));
            if (this.id) {
                await this._kernel.root.bind(this.term, [this._taskpath, this.id, "term"].join("/"));
            }

            await this.taskRoot.bind(this.term, `${this._taskpath}/self/term`);

            const program = [this.term, "program"].join("/");
            await this.taskRoot.bind(program, [this.path, "fd/0"].join("/"));
            await this.taskRoot.bind(program, [this.path, "fd/1"].join("/"));
            await this.taskRoot.bind(program, [this.path, "fd/2"].join("/"));
        } else {
            // torn on if this is the right default, but importantly it makes
            // it easier to redirect output because bind to #web/console works better
            // than bind to #task/<id>/fd/1 for whatever reason right now
            await this.taskRoot.bind("#web/console", [this.path, "fd/0"].join("/"));
            await this.taskRoot.bind("#web/console", [this.path, "fd/1"].join("/"));
            await this.taskRoot.bind("#web/console", [this.path, "fd/2"].join("/"));
        }

        if (!bindElements) {
            bindElements = this._childBinds();
        }
        // Model A: binds configure this task's namespace.
        await this._kernel._setupNamespace(this.rid, this.ns, bindElements);
    }

    async start() {
        await this._kernel.root.writeFile([this._taskpath, this.rid, "ctl"].join("/"), "start");
    }
}

if (typeof window !== "undefined") {
    customElements.define("wanix-task", TaskElement);
}

function spaceToNewline(input) {
    if (!input) return null;
    const tokens = [];
    let current = "";
    let inQuotes = false;

    for (const char of input) {
        if (char === "'") {
            inQuotes = !inQuotes;
        } else if (char === " " && !inQuotes) {
            if (current) {
                tokens.push(current);
                current = "";
            }
        } else {
            current += char;
        }
    }
    if (current) tokens.push(current);

    return tokens.join("\n");
}
