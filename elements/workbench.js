
const DEFAULT_ASSETS = new URL('workbench/', import.meta.url).href;
  
export class WorkbenchElement extends HTMLElement {

    constructor() {
      super();
      this._loaded = false;
      this._parsed = false;
      this.port = undefined;
      this.extension = this.getAttribute("extension") || "wanix.workbench";
      this.for = this.getAttribute('for');
      this.term = this.hasAttribute('term');
      this.debug = this.hasAttribute('debug');

      this.workdir = this.getAttribute("workdir");
      if (this.workdir === "." || this.workdir === "/" || !this.workdir) {
        this.workdir = "";
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

    get assets() {
      const raw = this.getAttribute('assets');
      if (!raw) return DEFAULT_ASSETS;
      return new URL((raw+"/").replace(/\/+$/, '/'), this.baseURI).href;
    }
  
    connectedCallback() {
      this.style.flex = "1";
      this.style.minHeight = "100%";
      this.style.display = "flex";
      this.style.flexDirection = "column";

      if (this.for) {
        this.system = document.getElementById(this.for);
      } else {
        this.system = this.closest('wanix-system');
      }

      if (this._parsed) {
        return;
      }
      if (document.readyState === 'loading') {
          document.addEventListener('DOMContentLoaded', () => this.parseChildren(), { once: true });
      } else {
          this.parseChildren();
      }
    }

    parseChildren() {
      this._parsed = true;
      this.shellTask = this.querySelector(':scope > wanix-task[role="shell"]');
      
      if (this.system) {
        const sendPort = async () => {
          const port = await this.system.openPort();
          this.load(() => ({wanix: port}));
        }
        if (this.system.isReady) {
          sendPort();
        } else {
          this.system.addEventListener('ready', async (e) => {
            sendPort();
          });
        }
      }
    }
  
    /** Load and mount the workbench. Idempotent. */
    load(portCb) {
      if (this._loaded) return;
      this._loaded = true;
  
      const codeDir = new URL("code/", this.assets);
      const outDir = new URL("out/", codeDir);
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
        // override the default background color of the terminal to match workbench theme
        const style = document.createElement('style');
        style.textContent = `.xterm .xterm-viewport { background-color: rgba(0, 0, 0, 0) !important; }`;
        document.head.appendChild(style);
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
        const obj = await portCb();
        const transfer = [...Object.values(obj)];
        obj["config"] = {
          term: this.term,
          taskNS: this.taskNS,
          termNS: this.termNS,
          taskCmd: this.shellTask?.cmd,
          taskType: this.shellTask?.type,
          workdir: this.shellTask?.workdir || this.workdir,
        };
        event.data.port.postMessage(obj, transfer);
      };
  
      const pageUrl = new URL(window.location.href);
      const scheme = pageUrl.protocol.replace(":", "");
      const hostParts = pageUrl.host.split(".");
      if (hostParts.length > 2) hostParts.shift();
      const hostJoin = hostParts.join(".");
  
      const codeDir = new URL("code/", this.assets);
      const outDir = new URL("out/", codeDir);
      const webviewPre = new URL(
        "vs/workbench/contrib/webview/browser/pre/",
        outDir,
      );
      let webviewContentExternalBaseUrlTemplate;
      if (this.assets.origin === pageUrl.origin) {
        const pathPrefix = this.assets.pathname.replace(/\/?$/, "");
        webviewContentExternalBaseUrlTemplate = `${scheme}://{{uuid}}.${hostJoin}${pathPrefix}/out/vs/workbench/contrib/webview/browser/pre/`;
      } else {
        webviewContentExternalBaseUrlTemplate = webviewPre.href;
      }
  
      const defaultConfig = {
        configurationDefaults: {
          "window.commandCenter": false,  
          "workbench.statusBar.visible": false,
          "workbench.layoutControl.enabled": false,
          "workbench.activityBar.location": "hidden",
          "workbench.tips.enabled": false,
          "workbench.welcomePage.walkthroughs.openOnInstall": false,
          "editor.minimap.enabled": false,
        //   "workbench.tree.indent": 12,
        //   "workbench.secondarySideBar.defaultVisibility": "visible", //"hidden",
        //   "problems.visibility": false,
        //   "workbench.startupEditor": "none",  
        //   "terminal.integrated.tabs.showActions": false,
        //   "workbench.panel.opensMaximized": "always",
        },
        developmentOptions: { logLevel: this.debug ? 2 : 0 },
        profile: DEFAULT_PROFILE,
      };
  
      require(["vs/workbench/workbench.web.main"], (wb) => {
        const config = mergeDeep(defaultConfig, {
          additionalBuiltinExtensions: [wb.URI.parse(this.assets)],
          productConfiguration: {
            extensionEnabledApiProposals: { [this.extension]: ["ipc"] },
            webviewContentExternalBaseUrlTemplate,
          },
          workspaceProvider: {
            trusted: true,
            workspace: { folderUri: wb.URI.parse(`wanix:/${this.workdir}`) },
            open(workspace, options) {
              console.log("todo: handle openFolder", workspace, options);
              return Promise.resolve(true);
            },
          }
        });
        if (!config.messagePorts) {
          config.messagePorts = new Map();
        }
        config.messagePorts.set(this.extension, ch.port1);

        wb.create(this, config);
        // console.log("workbench ready?");
        // this.dispatchEvent(new CustomEvent("workbench-ready"));
      });
    }
  }
  
  customElements.define("wanix-workbench", WorkbenchElement);
  
    
  /**
   * Resolves the workbench asset base: attribute `base` (path on current origin, or absolute URL).
   * Bundled scripts live under `{base}/out/...`.
   *
   * When `base` is set explicitly, the URL must be treated as a directory: if the path has no
   * trailing `/`, relative joins like `out/` would otherwise replace the last segment (e.g.
   * `/local/vscode` + `out/` → `/local/out/` instead of `/local/vscode/out/`).
   */
  function resolveAssetBase(el) {
    const raw = el.getAttribute("assets");
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

const DEFAULT_PROFILE = serializeWorkbenchProfile({
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
});

function mergeDeep(a, b) {
  if (Array.isArray(a) && Array.isArray(b)) return [...a, ...b];
  if (isPlainObject(a) && isPlainObject(b)) {
    const out = { ...a };
    for (const key of Object.keys(b)) {
      out[key] = key in a ? mergeDeep(a[key], b[key]) : b[key];
    }
    return out;
  }
  return b; // primitives / type mismatch: b wins
}

function isPlainObject(v) {
  return v !== null && typeof v === 'object' && v.constructor === Object;
}