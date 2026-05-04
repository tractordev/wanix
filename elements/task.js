import { WanixElement } from "./base.js";

export class TaskElement extends WanixElement {
    constructor() {
        super();
        this.rid = null;   
    }

    get path() {
        if (!this.rid) {
            throw new Error('Task not allocated');
        }
        return [this._taskpath, this.rid].join("/");
    }

    connectedCallback() {
        super.connectedCallback();

        this.type = this.getAttribute('type') || "auto";
        this.role = this.getAttribute('role');
        this.cmd = this.getAttribute('cmd');
        this.env = this.getAttribute('env');
        this.stdout = this.getAttribute('stdout');
        this.stderr = this.getAttribute('stderr');
        this.stdin = this.getAttribute('stdin');
        this.fsys = this.getAttribute('fsys');
        this._term = this.hasAttribute('term');
        this._autostart = this.hasAttribute('start');
        if (this.hasAttribute('wd')) {
            this.wd = this.getAttribute('wd');
        }

    }

    async _awake() {
        await this.allocate();
        if (this._autostart) {
            this.start();
        }
    }

    async allocate(bindElements=null) {
        if (this.rid) {
            throw new Error('Task already allocated');
        }
        this.rid = (await this._system.root.readText([this._taskpath, "new", this.type].join("/"))).trim();
        this.root = this._system.openHandle(this.rid);

        await this._system.root.writeFile([this.path, "cmd"].join("/"), this.cmd);
        if (this.env) {
            await this._system.root.writeFile([this.path, "env"].join("/"), spaceToNewline(this.env));
        }
        if (this.wd) {
            await this._system.root.writeFile([this.path, "dir"].join("/"), this.wd);
        }
        if (this.id) {
            await this._system.root.writeFile([this.path, "alias"].join("/"), this.id);
        }

        // otherwise it'll point to task 1 being cloned from root
        await this.root.bind(this.path, `${this._taskpath}/self`);

        if (this._term) {
            const termID = (await this._system.root.readText([this._termpath, "new"].join("/"))).trim();
            this.term = [this._termpath, termID].join("/");
            // should this binding be done in task vfs?
            await this._system.root.bind(this.term, [this.path, "term"].join("/"));
            // this is def a hack, but it works for now.
            // this is in addition to the above since aliased path needs its own binding.
            if (this.id) {
                await this._system.root.bind(this.term, [this._taskpath, this.id, "term"].join("/"));
            }

            // otherwise it'll point to task 1 being cloned from root
            await this.root.bind(this.term, `${this._taskpath}/self/term`);

            const program = [this.term, "program"].join("/");
            await this.root.bind(program, [this.path, "fd/0"].join("/"));
            await this.root.bind(program, [this.path, "fd/1"].join("/"));
            await this.root.bind(program, [this.path, "fd/2"].join("/"));
            
        } else {
            // await this.root.bind("#web/console", [this.path, "fd/1"].join("/"));
            // await this.root.bind("#web/console", [this.path, "fd/2"].join("/"));
        }
        
        if (!bindElements) {
            bindElements = this.querySelectorAll(':scope > wanix-bind');
        }
        this._system._setupNamespace(this.rid, this.fsys, bindElements);
    }

    async start() {
        await this._system.root.writeFile([this._taskpath, this.rid, "ctl"].join("/"), "start");
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