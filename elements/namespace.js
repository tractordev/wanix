import { WanixElement, parseNsAttribute } from "./base.js";

/**
 * Explicit root namespace + kernel host.
 * Child wanix-bind elements configure the root namespace.
 * The `ns` attribute is accepted for consistency; root always starts empty.
 */
export class NamespaceElement extends WanixElement {
    connectedCallback() {
        super.connectedCallback();

        this.ns = parseNsAttribute(this);

        this.style.display = "flex";
        this.style.flexDirection = "column";
        this.style.height = "100%";
        this.style.minHeight = "0";
    }

    async _awake() {
        // Root binds are applied during kernel bootstrap.
    }
}

if (typeof window !== "undefined") {
    customElements.define("wanix-namespace", NamespaceElement);
    // deprecated
    class SystemElement extends NamespaceElement {}
    customElements.define("wanix-system", SystemElement);
}
