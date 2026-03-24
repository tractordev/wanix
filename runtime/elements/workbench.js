/**
 * Web component that loads and mounts the VS Code workbench.
 * Call mount() when the host is ready (e.g. after any other scripts that must run first).
 *
 * Attribute `base` (optional): path on the current origin (e.g. "/vscode" or "/local/vscode") or absolute URL.
 * Workbench assets load from `{base}/out/...` (a trailing slash is not required). Defaults to the current URL.
 * Host pages should point workbench CSS at the same `{base}/out/...` path.
 *
 * Cascade: workbench CSS is loaded before app styles. App styles are in @layer app so
 * unlayered workbench rules (e.g. .monaco-tl-twistie padding) are not overridden.
 */

// const ORIG_DEFAULT_PROFILE = {
//   DEFAULT_BASE_PATH
//   const DEFAULT_PROFILE = {
//     contents: JSON.stringify({
//       globalState: JSON.stringify({
//           "workbench.explorer.views.state.hidden":
//             '[{"id":"outline","isHidden":true},{"id":"timeline","isHidden":true},{"id":"workbench.explorer.openEditorsView","isHidden":true},{"id":"workbench.explorer.emptyView","isHidden":false},{"id":"npm","isHidden":true}]',
//           "workbench.panel.pinnedPanels":
//             '[{"id":"workbench.panel.markers","pinned":false,"visible":false,"order":0},{"id":"workbench.panel.output","pinned":false,"visible":false,"order":1},{"id":"workbench.panel.repl","pinned":true,"visible":false,"order":2},{"id":"terminal","pinned":true,"visible":false,"order":3},{"id":"workbench.panel.testResults","pinned":true,"visible":false,"order":3},{"id":"refactorPreview","pinned":true,"visible":false}]',
//           "workbench.activity.pinnedViewlets2": JSON.stringify([
//             { id: "workbench.view.explorer", pinned: true, visible: true, order: 0 },
//             { id: "workbench.view.search", pinned: true, visible: true, order: 1 },
//             { id: "workbench.view.scm", pinned: false, visible: false, order: 2 },
//             { id: "workbench.view.debug", pinned: false, visible: false, order: 3 },
//             { id: "workbench.view.extensions", pinned: false, visible: false, order: 4 },
//           ]),
//         },
//       }),
//     }),

/**
 * Converts a plain profile object into the wire format VS Code expects.
 *
 * Stringification layers (see DEFAULT_PROFILE_RAW):
 * - `contents` is JSON.stringify of `{ globalState: <string> }`
 * - `globalState` is JSON.stringify of `{ storage: { ... } }` (and any sibling keys)
 * - each `storage[key]` value is JSON.stringify of the actual array/object/primitive
 *
 * @param {{ name: string, contents: { globalState: Record<string, unknown> & { storage?: Record<string, unknown> } } }} profile
 * @returns {{ name: string, contents: string }}
 */
export function serializeWorkbenchProfile(profile) {
    const { name, contents } = profile;
    const gs = contents.globalState;
    const storageIn = gs.storage ?? {};
    const storageOut = {};
    for (const [key, value] of Object.entries(storageIn)) {
      storageOut[key] =
        typeof value === "string" ? value : JSON.stringify(value);
    }
    const globalStatePayload = { ...gs, storage: storageOut };
    const globalStateString = JSON.stringify(globalStatePayload);
    const contentsString = JSON.stringify({ globalState: globalStateString });
    return { name, contents: contentsString };
  }
  
  /** Plain object shape; serializeWorkbenchProfile() produces the nested stringified form. */
  const DEFAULT_PROFILE_RAW = {
    name: "Default",
    contents: {
      globalState: {
        storage: {
          "workbench.explorer.views.state.hidden": [
            { id: "outline", isHidden: true },
            { id: "timeline", isHidden: true },
            { id: "workbench.explorer.openEditorsView", isHidden: true },
            { id: "workbench.explorer.emptyView", isHidden: false },
            { id: "npm", isHidden: true },
          ],
          "workbench.panel.pinnedPanels": [
            { id: "workbench.panel.markers", pinned: false, visible: false, order: 0 },
            { id: "workbench.panel.output", pinned: false, visible: false, order: 1 },
            { id: "workbench.panel.repl", pinned: true, visible: false, order: 2 },
            { id: "terminal", pinned: true, visible: false, order: 3 },
            { id: "workbench.panel.testResults", pinned: true, visible: false, order: 3 },
            { id: "refactorPreview", pinned: true, visible: false },
          ],
          "workbench.activity.pinnedViewlets2": [
            { id: "workbench.view.explorer", pinned: true, visible: true, order: 0 },
            { id: "workbench.view.search", pinned: true, visible: true, order: 1 },
            { id: "workbench.view.scm", pinned: false, visible: false, order: 2 },
            { id: "workbench.view.debug", pinned: false, visible: false, order: 3 },
            { id: "workbench.view.extensions", pinned: false, visible: false, order: 4 },
          ],
        },
      },
    },
  };
  
  const DEFAULT_PROFILE = serializeWorkbenchProfile(DEFAULT_PROFILE_RAW);
  
  /**
   * Resolves the workbench asset base: attribute `base` (path on current origin, or absolute URL).
   * Bundled scripts live under `{base}/out/...`.
   *
   * When `base` is set explicitly, the URL must be treated as a directory: if the path has no
   * trailing `/`, relative joins like `out/` would otherwise replace the last segment (e.g.
   * `/local/vscode` + `out/` → `/local/out/` instead of `/local/vscode/out/`).
   */
  function resolveWorkbenchBase(el) {
    const raw = el.getAttribute("base");
    const explicit = raw != null && raw.trim() !== "";
    const value = explicit ? raw.trim() : window.location.toString();
    try {
      const url = /^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(value)
        ? new URL(value)
        : new URL(value, window.location.origin);
      if (explicit && !url.pathname.endsWith("/")) {
        url.pathname += "/";
      }
      return url;
    } catch {
      const fallback = new URL(window.location.pathname, window.location.origin);
      if (explicit && !fallback.pathname.endsWith("/")) {
        fallback.pathname += "/";
      }
      return fallback;
    }
  }
  
  export class Workbench extends HTMLElement {
    constructor() {
      super();
      this._loaded = false;
      this.port = undefined;
    }
  
    connectedCallback() {
      this.style.flex = "1";
      this.style.minHeight = "100%";
      this.style.display = "flex";
      this.style.flexDirection = "column";
    }
  
    /** Load and mount the workbench. Idempotent. */
    load(portCb) {
      if (this._loaded) return;
      this._loaded = true;
  
      const outDir = new URL("out/", resolveWorkbenchBase(this));
      const outRoot = outDir.href.replace(/\/?$/, "");
  
      const nls = document.createElement("script");
      nls.src = new URL("nls.messages.js", outDir).href;
      const loader = document.createElement("script");
      loader.src = new URL("vs/loader.js", outDir).href;

      // Check if a <link> to workbench.web.main.css already exists in <head>
      const cssAlreadyLoaded = Array.from(document.head.querySelectorAll('link[rel="stylesheet"]'))
        .some(link => link.href.endsWith("workbench.web.main.css"));
      if (!cssAlreadyLoaded) {
        const cssLink = document.createElement("link");
        cssLink.rel = "stylesheet";
        cssLink.href = new URL("vs/workbench/workbench.web.main.css", outDir).href;
        document.head.appendChild(cssLink);
      }
  
      const runBootstrap = () => {
        const go = () => {
          if (typeof require === "undefined") {
            setTimeout(go, 0);
            return;
          }
          require.config({ baseUrl: outRoot });
          this._createWorkbench(portCb);
        };
        go();
      };
  
      loader.onload = runBootstrap;
      loader.onerror = () => this.dispatchEvent(new CustomEvent("error", { detail: new Error("Failed to load VS Code loader") }));
      document.head.appendChild(loader);
      document.head.appendChild(nls);
    }
  
    _createWorkbench(portCb) {
      const ch = new MessageChannel();
      this.port = ch.port2;
      this.port.onmessage = async (event) => {
        const port = await portCb();
        event.data.port.postMessage({ wanix: port }, [port]);
    };
  
      const pageUrl = new URL(window.location.href);
      const scheme = pageUrl.protocol.replace(":", "");
      const hostParts = pageUrl.host.split(".");
      if (hostParts.length > 2) hostParts.shift();
      const hostJoin = hostParts.join(".");
  
      const assetBase = resolveWorkbenchBase(this);
      const outDir = new URL("out/", assetBase);
      const webviewPre = new URL(
        "vs/workbench/contrib/webview/browser/pre/",
        outDir,
      );
      let webviewContentExternalBaseUrlTemplate;
      if (assetBase.origin === pageUrl.origin) {
        const pathPrefix = assetBase.pathname.replace(/\/?$/, "");
        webviewContentExternalBaseUrlTemplate = `${scheme}://{{uuid}}.${hostJoin}${pathPrefix}/out/vs/workbench/contrib/webview/browser/pre/`;
      } else {
        webviewContentExternalBaseUrlTemplate = webviewPre.href;
      }
  
      const config = {
        messagePorts: new Map([["progrium.apptron-system", ch.port1]]),
        productConfiguration: {
          extensionEnabledApiProposals: { "progrium.apptron-system": ["ipc"] },
          webviewContentExternalBaseUrlTemplate,
        },
        configurationDefaults: {
          "workbench.colorTheme": "Tractor Dark",
          "workbench.secondarySideBar.defaultVisibility": "hidden",
          "workbench.statusBar.visible": false,
          "workbench.layoutControl.enabled": false,
          "window.commandCenter": false,
          "workbench.startupEditor": "none",
          "workbench.activityBar.location": "hidden",
          "workbench.tips.enabled": false,
          "workbench.welcomePage.walkthroughs.openOnInstall": false,
          "problems.visibility": false,
          "editor.minimap.enabled": false,
          "terminal.integrated.tabs.showActions": false,
        },
        developmentOptions: { logLevel: 0 },
        additionalBuiltinExtensions: [
          { scheme, authority: pageUrl.host, path: "/local/system" },
        //   { scheme, authority: pageUrl.host, path: "/preview" },
        ],
        profile: DEFAULT_PROFILE,
        folderUri: { scheme: "wanix", path: "/project" },
      };
  
      require(["vs/workbench/workbench.web.main"], (wb) => {
        wb.create(this, {
          ...config,
          workspaceProvider: {
            trusted: true,
            workspace: { folderUri: wb.URI.revive(config.folderUri) },
            open(workspace, options) {
              console.log("openFolder requested", workspace);
              return Promise.resolve(true);
            },
          },
        });
        this.dispatchEvent(new CustomEvent("vscode-ready"));
      });
    }
  }
  
  customElements.define("wanix-workbench", Workbench);
  