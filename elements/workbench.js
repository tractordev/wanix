import { WanixElement } from "./base.js";

const DEFAULT_ASSETS = new URL('workbench/', import.meta.url).href;
  
export class WorkbenchElement extends WanixElement {
    constructor() {
      super();
      this._loaded = false;
      this._parsed = false;
      this.port = undefined;
      this.tasks = {}; // task role -> task element
    }

    get assets() {
      const raw = this.getAttribute('assets');
      if (!raw) return DEFAULT_ASSETS;
      return new URL((raw+"/").replace(/\/+$/, '/'), this.baseURI).href;
    }
  
    connectedCallback() {
      super.connectedCallback();

      this.style.flex = "1";
      this.style.height = "100%";
      this.style.width = "100%";
      this.style.display = "flex";
      this.style.flexDirection = "column";

      this.extension = this.getAttribute("extension") || "wanix.workbench";

      this.debug = this.hasAttribute('debug');
      this._term = this.hasAttribute('term');
      this._sidebar = parseSidebarMode(this.getAttribute("sidebar"));
      this._welcome = this.hasAttribute("welcome");
      this._openPaths = parseOpenPaths(this.getAttribute("open"));
      this.raw = this.hasAttribute('raw');

      this.wd = this.getAttribute("wd");
      if (this.wd === "." || this.wd === "/" || !this.wd) {
        this.wd = "";
      }
    }

    async _awake() {
      this.tasks["shell"] = this.querySelector(':scope > wanix-task[role="shell"]');
      // Wait for child shell template to finish setup when present.
      if (this.tasks["shell"]) {
        await this.tasks["shell"]._nsReady;
      }
      const port = await this._kernel._openPort(); // todo: should workbench have a task?
      if (this.wd) {
        await this._kernel.root.waitFor(this.wd, 3000);
      }
      this.load(() => ({wanix: port}));
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
          term: this._term,
          raw: this.raw,
          sidebar: this._sidebar,
          ns: {
            task: this._taskpath,
            term: this._termpath,
          },
          shell: {
            cmd: this.tasks["shell"]?.cmd,
            type: this.tasks["shell"]?.type,
            wd: this.tasks["shell"]?.wd || this.wd,
          },
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
  
      const configurationDefaults = {
        "window.commandCenter": false,
        "workbench.statusBar.visible": false,
        "workbench.layoutControl.enabled": false,
        "workbench.activityBar.location": "hidden",
        "workbench.tips.enabled": false,
        "workbench.welcomePage.walkthroughs.openOnInstall": false,
        "workbench.startupEditor": this._welcome ? "welcomePage" : "none",
        "editor.minimap.enabled": false,
      };
      const defaultConfig = {
        configurationDefaults,
        //   "workbench.tree.indent": 12,
        //   "workbench.secondarySideBar.defaultVisibility": "visible", //"hidden",
        //   "problems.visibility": false,
        //   "workbench.startupEditor": "none",
        //   "terminal.integrated.tabs.showActions": false,
        //   "workbench.panel.opensMaximized": "always",
        developmentOptions: { logLevel: this.debug ? 2 : 0 },
        profile: buildWorkbenchProfile(this._sidebar),
      };
  
      require(["vs/workbench/workbench.web.main"], async (wb) => {
        const folderUri = wb.URI.parse(`wanix:/${this.wd}`);
        await applySidebarLayout(folderUri.toString(), this._sidebar);
        const defaultLayout = buildDefaultLayout(this._sidebar, this._openPaths, wb, this.wd);
        if (defaultLayout) {
          defaultConfig.defaultLayout = defaultLayout;
        }
        const config = mergeDeep(defaultConfig, {
          additionalBuiltinExtensions: [wb.URI.parse(this.assets)],
          productConfiguration: {
            extensionEnabledApiProposals: { [this.extension]: ["ipc"] },
            webviewContentExternalBaseUrlTemplate,
          },
          workspaceProvider: {
            trusted: true,
            workspace: { folderUri },
            open(workspace, options) {
              console.log("todo: handle openFolder", workspace, options);
              return Promise.resolve(true);
            },
          },
          commands: [
            {
              id: '__wanix.fileSaved',
              handler: (payload) => {
                this.dispatchEvent(new CustomEvent('workbench:documentSaved', { detail: payload }));
              }
            },
          ],
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
const WORKBENCH_LAYOUT_SIDEBAR_HIDDEN_KEY = "workbench.sideBar.hidden";

/** @param {string | null} value */
function parseOpenPaths(value) {
  if (!value) return [];
  return value.trim().split(/\s+/).filter(Boolean);
}

/**
 * @param {"default" | "hidden" | "never"} sidebarMode
 * @param {string[]} openPaths
 * @param {string} wd
 */
function buildDefaultLayout(sidebarMode, openPaths, wb, wd) {
  /** @type {{ views?: unknown[], editors?: { uri: unknown }[], force?: boolean }} */
  const layout = {};
  if (sidebarMode === "never") {
    layout.views = [];
  }
  if (openPaths.length) {
    layout.editors = openPaths.map((path) => ({
      uri: toWanixFileUri(wb, wd, path),
    }));
  }
  if (layout.views || layout.editors) {
    layout.force = true;
    return layout;
  }
  return undefined;
}

/** @param {string} wd workspace folder path segment (e.g. "root") */
function toWanixFileUri(wb, wd, path) {
  const normalized = path.replace(/^\/+/, "");
  const workspacePath = wd ? `${wd}/${normalized}` : normalized;
  return wb.URI.parse(`wanix:/${workspacePath}`);
}

/** @returns {"default" | "hidden" | "never"} */
function parseSidebarMode(value) {
  const mode = (value ?? "default").trim().toLowerCase();
  if (mode === "" || mode === "default") return "default";
  if (mode === "hidden" || mode === "never") return mode;
  return "default";
}

/** VS Code string hash (matches base/common/hash stringHash). */
function hashString(s) {
  let h = 149417;
  for (let i = 0; i < s.length; i++) {
    h = ((h << 5) - h + s.charCodeAt(i)) | 0;
  }
  return h;
}

function workspaceStorageId(folderUriString) {
  return hashString(folderUriString).toString(16);
}

function openWorkspaceStorageDb(workspaceId) {
  const dbName = `vscode-web-state-db-${workspaceId}`;
  const storeName = "ItemTable";
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(dbName);
    request.onerror = () => reject(request.error);
    request.onupgradeneeded = () => {
      const db = request.result;
      if (!db.objectStoreNames.contains(storeName)) {
        db.createObjectStore(storeName);
      }
    };
    request.onsuccess = () => resolve({ db: request.result, storeName });
  });
}

async function readWorkspaceStorageEntry(workspaceId, key) {
  const { db, storeName } = await openWorkspaceStorageDb(workspaceId);
  return new Promise((resolve, reject) => {
    const tx = db.transaction(storeName, "readonly");
    const req = tx.objectStore(storeName).get(key);
    req.onsuccess = () => {
      db.close();
      resolve(req.result);
    };
    req.onerror = () => {
      db.close();
      reject(req.error);
    };
  });
}

async function writeWorkspaceStorageEntry(workspaceId, key, value) {
  const { db, storeName } = await openWorkspaceStorageDb(workspaceId);
  return new Promise((resolve, reject) => {
    const tx = db.transaction(storeName, "readwrite");
    tx.objectStore(storeName).put(value, key);
    tx.oncomplete = () => {
      db.close();
      resolve();
    };
    tx.onerror = () => {
      db.close();
      reject(tx.error);
    };
  });
}

/**
 * Apply sidebar visibility from the sidebar attribute.
 * @param {"default" | "hidden" | "never"} mode
 */
async function applySidebarLayout(folderUriString, mode) {
  if (mode === "default") return;

  const workspaceId = workspaceStorageId(folderUriString);
  if (mode === "never") {
    await writeWorkspaceStorageEntry(workspaceId, WORKBENCH_LAYOUT_SIDEBAR_HIDDEN_KEY, "true");
    return;
  }
  // hidden: seed only when the workspace has no saved layout yet
  const existing = await readWorkspaceStorageEntry(workspaceId, WORKBENCH_LAYOUT_SIDEBAR_HIDDEN_KEY);
  if (existing === undefined) {
    await writeWorkspaceStorageEntry(workspaceId, WORKBENCH_LAYOUT_SIDEBAR_HIDDEN_KEY, "true");
  }
}

export function buildWorkbenchProfile(sidebarMode) {
  const showViewlets = sidebarMode === "default";
  return serializeWorkbenchProfile({
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
            { id: "workbench.view.explorer", pinned: true, visible: showViewlets, order: 0 },
            { id: "workbench.view.search", pinned: true, visible: showViewlets, order: 1 },
            { id: "workbench.view.scm", pinned: false, visible: false, order: 2 },
            { id: "workbench.view.debug", pinned: false, visible: false, order: 3 },
            { id: "workbench.view.extensions", pinned: false, visible: false, order: 4 },
          ],
        },
      },
    },
  });
}

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