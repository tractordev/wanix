
export class WanixElement extends HTMLElement {
    constructor() {
        super();
        this._taskpath = "#task";
        this._termpath = "#term";
        this._vmpath = "#vm";
    }

    connectedCallback() {
        if (this.hasAttribute('task-ns')) {
            this._taskpath = this.getAttribute('task-ns');
        }
        if (this.hasAttribute('term-ns')) {
            this._termpath = this.getAttribute('term-ns');
        }
        if (this.hasAttribute('vm-ns')) {
            this._vmpath = this.getAttribute('vm-ns');
        }

        if (this.tagName === "WANIX-SYSTEM") {
            return;
        }

        this._system = this.closest('wanix-system');
        if (this.hasAttribute('for')) {
            this._system = document.getElementById(this.getAttribute('for'));
            if (this._system && this._system.tagName !== "WANIX-SYSTEM") {
                throw new Error('Component element must be a child of a wanix-system element');
            }
        }
        if (this._system) {
            this._system.addEventListener('ready', () => this._awake());
        }
    }

    _awake() { throw new Error('Not implemented'); }

}
