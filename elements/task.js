import { NamespaceElement } from "./namespace.js";

export class TaskElement extends NamespaceElement {
    constructor() {
        super();
        this.cmd = this.getAttribute('cmd');
        this.type = this.getAttribute('type') || "auto";
        
        this.role = this.getAttribute('role');
        this.env = this.getAttribute('env');
        
        if (this.hasAttribute('workdir')) {
            this.workdir = this.getAttribute('workdir');
        }

        this.stdout = this.getAttribute('stdout');
        this.stderr = this.getAttribute('stderr');
        this.stdin = this.getAttribute('stdin');

        this.for = this.getAttribute('for');
        this.fsys = this.getAttribute('fsys');
        this._term = this.hasAttribute('term');
        this.autostart = this.hasAttribute('start');

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
        return [this.taskNS, this.rid].join("/");
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
            throw new Error('Task already allocated');
        }
        this.rid = (await this.system.root.readText([this.taskNS, "new", this.type].join("/"))).trim();
        this.root = this.system.openHandle(this.rid);

        await this.system.root.writeFile([this.taskNS, this.rid, "cmd"].join("/"), this.cmd);
        if (this.env) {
            await this.system.root.writeFile([this.taskNS, this.rid, "env"].join("/"), spaceToNewline(this.env));
        }
        if (this.workdir) {
            await this.system.root.writeFile([this.taskNS, this.rid, "dir"].join("/"), this.workdir);
        }
        if (this.id) {
            await this.system.root.writeFile([this.taskNS, this.rid, "alias"].join("/"), this.id);
        }
        if (this._term) {
            const termID = (await this.system.root.readText([this.termNS, "new"].join("/"))).trim();
            this.term = [this.termNS, termID].join("/");
            // should this binding be done in task vfs?
            await this.system.root.bind(this.term, [this.path, "term"].join("/"));
            // this is def a hack, but it works for now
            if (this.id) {
                await this.system.root.bind(this.term, [this.taskNS, this.id, "term"].join("/"));
            }

            const program = [this.term, "program"].join("/");
            await this.root.bind(program, [this.path, "fd/0"].join("/"));
            await this.root.bind(program, [this.path, "fd/1"].join("/"));
            await this.root.bind(program, [this.path, "fd/2"].join("/"));
        } else {
            await this.root.bind("#web/console", [this.path, "fd/1"].join("/"));
            await this.root.bind("#web/console", [this.path, "fd/2"].join("/"));
        }
        
    }

    async start() {
        await this.system.root.writeFile([this.taskNS, this.rid, "ctl"].join("/"), "start");
        // console.log('task start', this, this.rid, this.id);
    }
}

if (typeof window !== "undefined") {
    customElements.define("wanix-task", TaskElement);
}

function spaceToNewline(input) {
    const tokens = [];
    let current = '';
    let inQuotes = false;
  
    for (const char of input) {
      if (char === "'") {
        inQuotes = !inQuotes;
      } else if (char === ' ' && !inQuotes) {
        if (current) {
          tokens.push(current);
          current = '';
        }
      } else {
        current += char;
      }
    }
    if (current) tokens.push(current);
  
    return tokens.join('\n');
  }