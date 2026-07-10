import { register, cacheURL, templateFetch, cacheFetch, cacheTemplate } from '../wanix-sw.js';
import {
  clientConfig,
  isInternalPathname,
  parseUrl,
} from './site-url.js';

const LOCAL_CACHE_NAME = "local";
const REMOTE_CACHE_NAME = "remote";
const HANKO_STORAGE_KEY = "hanko";
const SKIP_SW_HEADER = "x-skip-wanix-sw";
const OVERLAY_STYLE_ID = "wanix-site-overlay-styles";

function siteConfig() {
  return clientConfig();
}

function currentSite() {
  return parseUrl(location.href, siteConfig());
}

function siteIdFromLocation() {
  return currentSite().xid;
}

function sitePagePath(path = "/index.html") {
  return path.startsWith("/") ? path : `/${path}`;
}

const OVERLAY_STYLES = `
  body.lock {
    overflow: hidden;
  }

  #edit-mode {
    --wanix-split: 50vw;
    --wanix-t: 0.38s cubic-bezier(0.4, 0, 0.2, 1);
    position: fixed;
    inset: 0;
    z-index: 999;
    display: flex;
    flex-direction: row;
    min-width: 0;
    min-height: 0;
  }

  #edit-mode[hidden] {
    display: flex !important;
    visibility: hidden;
    pointer-events: none;
  }

  #edit-mode.ready {
    visibility: visible;
    pointer-events: auto;
  }

  #site-editor-panel {
    flex: 0 0 auto;
    width: 0;
    overflow: hidden;
    background: #1e1e1e;
    border-right: 1px solid transparent;
    pointer-events: none;
    transition:
      width var(--wanix-t),
      border-color var(--wanix-t),
      box-shadow var(--wanix-t);
  }

  #edit-mode.editor #site-editor-panel {
    width: var(--wanix-split);
    pointer-events: auto;
    border-right-color: #ccd;
    box-shadow: 4px 0 16px rgba(0, 0, 0, 0.06);
  }

  #site-editor-panel iframe {
    width: var(--wanix-split);
    height: 100%;
    border: 0;
    background: #1e1e1e;
    visibility: hidden;
  }

  #edit-mode.editor-ready #site-editor-panel iframe {
    visibility: visible;
  }

  #site-preview {
    flex: 1;
    min-width: 0;
    border: 0;
  }

  #edit-mode-divider {
    flex: 0 0 0;
    width: 0;
    position: relative;
    z-index: 2;
    display: none;
    align-items: center;
    justify-content: center;
    cursor: col-resize;
    touch-action: none;
    user-select: none;
  }

  #edit-mode.editor #edit-mode-divider {
    display: flex;
    flex: 0 0 7px;
    width: 7px;
    margin: 0 -3px;
  }

  #edit-mode-divider::after {
    content: "";
    width: 4px;
    height: 28px;
    border-radius: 2px;
    background: #99a;
    opacity: 0.55;
    transition: opacity 0.15s, background 0.15s;
  }

  #edit-mode-divider:hover::after,
  #edit-mode.resizing #edit-mode-divider::after {
    opacity: 1;
    background: #667;
  }

  #edit-mode.resizing,
  #edit-mode.resizing #site-editor-panel {
    transition: none;
  }

  #edit-mode.resizing iframe {
    pointer-events: none;
  }

  @media (prefers-reduced-motion: reduce) {
    #edit-mode {
      --wanix-t: 0.01ms;
    }
  }
`;

function injectOverlayStyles() {
  if (document.getElementById(OVERLAY_STYLE_ID)) {
    return;
  }
  const style = document.createElement("style");
  style.id = OVERLAY_STYLE_ID;
  style.textContent = OVERLAY_STYLES;
  document.head.append(style);
}

let swReady;
if (window === window.top) {
  swReady = register({
    onfetch: async (req) => {
      const url = new URL(req.url);
      if (
        url.origin !== location.origin ||
        isInternalPathname(url.pathname)
      ) {
        return null;
      }

      const config = siteConfig();
      const page = parseUrl(location.href, config);
      const requestSite = parseUrl(req.url, config);
      if (!page.xid && !requestSite.xid) {
        return null;
      }
      
      // console.log("checking local cache", requestSite);
      const localcache = await cacheFetch(req, "local");
      if (localcache) {
        return localcache;
      }

      // console.log("checking main cache", requestSite);
      const cached = await cacheFetch(req, REMOTE_CACHE_NAME);
      if (cached) {
        (async () => {
          const cache = await caches.open(REMOTE_CACHE_NAME);
          const res = await siteNetworkFetch(req.url);
          if (res.ok) {
            await cache.put(cacheKey(req.url), res);
          }
        })();
        return cached;
      }
    },
    oninstall: () => siteEnsureCache(REMOTE_CACHE_NAME, siteCachePrefetchPaths()),
  });
}

const GET_RESOURCE_INITIATORS = new Set([
  "navigation",
  "script",
  "link",
  "css",
  "iframe",
  "img",
  "audio",
  "video",
  "other",
  "worker",
]);

function isPrefetchableUrl(pathOrUrl) {
  try {
    const url = new URL(pathOrUrl, location.origin);
    if (url.origin !== location.origin) {
      return false;
    }
    if (isInternalPathname(url.pathname)) {
      return false;
    }
    if (url.pathname.split("/").pop()?.startsWith("_")) {
      return false;
    }
    return true;
  } catch {
    return false;
  }
}

function siteCachePrefetchPaths() {
  const { xid } = currentSite();
  const resources = performance
    .getEntriesByType("resource")
    .filter(
      (entry) =>
        GET_RESOURCE_INITIATORS.has(entry.initiatorType) &&
        isPrefetchableUrl(entry.name)
    )
    .map((entry) => entry.name);
  return [
    ...new Set([
      sitePagePath("/index.html"),
      "/editor.html",
      "/favicon.ico",
      "/style.css",
      // "/.lib/wanix-site.js",
      // "/.lib/site-url.js",
      // "/.lib/wanix.min.js",
      // "/.lib/wanix.debug.wasm",
      // "/wanix-sw.js",
      ...resources,
    ]),
  ];
}

function normalizeCachePath(path) {
  const url = new URL(path, location.origin);
  if (url.pathname.endsWith("/")) {
    return new URL(`${url.pathname}index.html`, url.origin).pathname;
  }
  return url.pathname;
}

function cacheKey(path) {
  return new Request(new URL(normalizeCachePath(path), location.origin));
}

async function siteNetworkFetch(path) {
  const url = new URL(path, location.origin);
  return fetch(url, {
    headers: { [SKIP_SW_HEADER]: "1" },
    cache: "no-store",
    credentials: "same-origin",
  });
}

let siteCachePopulate = null;

async function siteEnsureCache(cacheName, prefetch = []) {
  const cache = await caches.open(cacheName);
  if (prefetch.length > 0 && (await cache.match(cacheKey(prefetch[0])))) {
    return cache;
  }
  if (!siteCachePopulate) {
    siteCachePopulate = Promise.all(
      prefetch.filter(isPrefetchableUrl).map(async (path) => {
        const res = await siteNetworkFetch(path);
        if (res.ok) {
          await cache.put(cacheKey(path), res);
        }
      })
    ).finally(() => {
      siteCachePopulate = null;
    });
  }
  await siteCachePopulate;
  return cache;
}

// async function siteRepopulateCache(cacheName, prefetch = []) {
//   siteCachePopulate = null;
//   let paths = [...prefetch, ...siteCachePrefetchPaths()];
//   if ("caches" in globalThis) {
//     const existing = await caches.open(cacheName);
//     paths.push(...(await existing.keys()).map((key) => key.url));
//     await caches.delete(cacheName);
//   }
//   paths = [...new Set(paths)].filter(isPrefetchableUrl);
//   const cache = await caches.open(cacheName);
//   await Promise.all(
//     paths.map(async (path) => {
//       const res = await siteNetworkFetch(path);
//       if (res.ok) {
//         await cache.put(cacheKey(path), res);
//       }
//     })
//   );
//   return cache;
// }


class WanixSite extends HTMLElement {
  static get observedAttributes() {
    return ["mode", "prefetch"];
  }

  get mode() {
    const value = (this.getAttribute("mode") || "view").toLowerCase();
    return value === "edit" ? "edit" : "view";
  }

  get prefetch() {
    return this.hasAttribute("prefetch");
  }

  constructor() {
    super();
    this.editMode = false;
    this._previewReady = null;
    this._editorReady = null;
    const root = this.attachShadow({ mode: "open" });
    root.innerHTML = `
      <style>
        :host {
          position: fixed;
          right: 1rem;
          bottom: 1rem;
          z-index: 2000;
        }
        .toolbar {
          display: flex;
          align-items: stretch;
          border: 1px solid #ddd;
          border-radius: 999px;
          background: #fff;
          box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
          overflow: hidden;
        }
        button {
          margin: 0;
          padding: 0.65rem 0.9rem;
          border: 0;
          border-radius: 0;
          background: transparent;
          box-shadow: none;
          font: 0.8125rem system-ui, sans-serif;
          cursor: pointer;
        }
        button + button {
          border-left: 1px solid #ddd;
        }
        button:disabled {
          opacity: 0.45;
          cursor: not-allowed;
        }
        button[aria-busy="true"] {
          opacity: 0.55;
          cursor: wait;
        }
      </style>
      <div class="toolbar">
        <button type="button" data-action="revert" hidden disabled aria-label="Revert changes">
          Revert
        </button>
        <button type="button" data-action="publish" hidden disabled aria-label="Publish changes">
          Publish
        </button>
        <button type="button" data-action="preview" aria-label="Open editor">Edit</button>
      </div>
    `;
    this.revertButton = root.querySelector('[data-action="revert"]');
    this.publishButton = root.querySelector('[data-action="publish"]');
    this.previewButton = root.querySelector('[data-action="preview"]');
    this.overlay = null;
    this.panel = null;
    this.previewFrame = null;
    this.editorFrame = null;
    this._cacheBaseline = null;
    this._publishedBaseline = null;
    this._cacheWatchTimer = null;
    this._lastEventFingerprint = null;
    this._bootstrapping = null;
    this._hankoUsername = null;
  }

  createOverlay() {
    injectOverlayStyles();

    const overlay = document.createElement("aside");
    overlay.id = "edit-mode";
    overlay.hidden = true;

    const panel = document.createElement("div");
    panel.id = "site-editor-panel";

    const editor = document.createElement("iframe");
    editor.id = "site-editor";
    editor.title = "Site editor";

    const preview = document.createElement("iframe");
    preview.id = "site-preview";
    preview.title = "Site preview";

    const divider = document.createElement("div");
    divider.id = "edit-mode-divider";
    divider.title = "Drag to resize";
    divider.setAttribute("role", "separator");
    divider.setAttribute("aria-orientation", "vertical");

    panel.append(editor);
    overlay.append(panel, divider, preview);
    document.body.append(overlay);

    this.overlay = overlay;
    this.panel = panel;
    this.editorFrame = editor;
    this.previewFrame = preview;
    this.bindSplitResize(divider);
  }

  bindSplitResize(divider) {
    divider.addEventListener("pointerdown", (e) => {
      if (e.button !== 0 || !this.overlay.classList.contains("editor")) {
        return;
      }
      e.preventDefault();
      this.overlay.classList.add("resizing");
      divider.setPointerCapture(e.pointerId);

      const onMove = (ev) => {
        const rect = this.overlay.getBoundingClientRect();
        const min = 160;
        const max = Math.max(min, rect.width - 160);
        const x = Math.min(max, Math.max(min, ev.clientX - rect.left));
        this.overlay.style.setProperty("--wanix-split", `${x}px`);
      };
      const onUp = (ev) => {
        this.overlay.classList.remove("resizing");
        divider.releasePointerCapture(ev.pointerId);
        divider.removeEventListener("pointermove", onMove);
        divider.removeEventListener("pointerup", onUp);
        divider.removeEventListener("pointercancel", onUp);
      };
      divider.addEventListener("pointermove", onMove);
      divider.addEventListener("pointerup", onUp);
      divider.addEventListener("pointercancel", onUp);
    });
  }

  connectedCallback() {
    if (window !== window.top) {
      this.style.display = "none";
      return;
    }
    this.createOverlay();
    this.initHankoUser();
    this.previewButton.addEventListener("click", () => {
      void this.onPreviewClick();
    });
    this.revertButton.addEventListener("click", () => {
      void this.onRevertClick();
    });
    this.publishButton.addEventListener("click", () => {
      void this.onPublishClick();
    });
    const onLoad = () => {
      void this.startBootstrap().then(() => this.maybeAutoEnterEdit());
    };
    if (document.readyState === "complete") {
      onLoad();
    } else {
      window.addEventListener("load", onLoad, { once: true });
    }
  }

  disconnectedCallback() {
    this.stopCacheWatch();
  }

  async waitForCache(
    name = REMOTE_CACHE_NAME,
    path = sitePagePath("/index.html"),
    { interval = 50, timeout = 60_000 } = {}
  ) {
    const deadline = Date.now() + timeout;
    while (Date.now() < deadline) {
      if ("caches" in globalThis) {
        const cache = await caches.open(name);
        if (await cache.match(cacheKey(path))) {
          return cache;
        }
      }
      await new Promise((resolve) => setTimeout(resolve, interval));
    }
    throw new Error(`cache ${name} did not contain ${path} within ${timeout}ms`);
  }

  async startBootstrap() {
    if (this._bootstrapping) {
      return this._bootstrapping;
    }
    this._bootstrapping = this._runBootstrap();
    return this._bootstrapping;
  }

  async _runBootstrap() {
    if (swReady) {
      await swReady;
    }
    try {
      await siteEnsureCache(REMOTE_CACHE_NAME, siteCachePrefetchPaths());
      await this.waitForCache();
      // this.startCacheWatch();
    } catch (err) {
      console.warn("wanix-site: cache not ready, bootstrapping anyway", err);
    }
    if (!this._previewReady) {
      this.bootstrapFrames();
    }
  }

  showOverlay() {
    this.overlay.hidden = false;
    this.overlay.classList.add("ready");
    document.body.classList.add("lock");
  }

  async ensureEditorReady() {
    await this.startBootstrap();
    await this._previewReady;
    if (!this._editorReady) {
      this.bootstrapEditor();
    }
    await this._editorReady;
  }

  bootstrapFrames() {
    delete this.previewFrame.dataset.loaded;
    this._previewReady = this.whenFrame(this.previewFrame);
    this.previewFrame.src = location.href;

    this._previewReady.then(() => {
      this.showOverlay();
    });

    if (this.prefetch) {
      this.bootstrapEditor();
    }
  }

  bootstrapEditor() {
    delete this.editorFrame.dataset.loaded;
    this._editorReady = this.whenEditorAppReady(this.editorFrame);
    this.editorFrame.src = "/editor.html";
  }

  whenEditorAppReady(frame) {
    return this.whenFrame(frame, { path: "/editor.html" }).then(() => {
      this.waitForEditorWorkbench(frame);
      frame.contentDocument.querySelector("wanix-workbench")
        .addEventListener("workbench:documentSaved", (e) => {
          this.previewFrame.contentWindow.location.reload();
        });
    });
  }

  waitForEditorWorkbench(frame) {
    return new Promise((resolve, reject) => {
      const deadline = Date.now() + 120_000;
      let system = null;

      const cleanup = () => {
        if (system) {
          system.removeEventListener("ready", onSystemReady);
          system = null;
        }
      };

      const onSystemReady = () => {
        system = null;
        tick();
      };

      const tick = () => {
        if (Date.now() > deadline) {
          cleanup();
          reject(new Error("editor workbench did not load in time"));
          return;
        }

        let doc;
        try {
          doc = frame.contentDocument;
        } catch (err) {
          cleanup();
          reject(err);
          return;
        }

        if (!doc) {
          requestAnimationFrame(tick);
          return;
        }

        const bench = doc.querySelector("wanix-workbench");
        if (bench?.childElementCount > 0) {
          cleanup();
          resolve();
          return;
        }

        const nextSystem = doc.querySelector("wanix-workbench");
        if (nextSystem && !nextSystem.isReady) {
          if (system !== nextSystem) {
            cleanup();
            system = nextSystem;
            system.addEventListener("ready", onSystemReady, { once: true });
          }
          return;
        }

        requestAnimationFrame(tick);
      };

      tick();
    });
  }

  whenFrame(frame, { path = null } = {}) {
    if (frame.dataset.loaded === "true") {
      return Promise.resolve();
    }
    const matches = () => {
      if (!path) {
        return true;
      }
      try {
        return new URL(frame.src, location.origin).pathname.endsWith(path);
      } catch {
        return false;
      }
    };
    return new Promise((resolve) => {
      const done = () => {
        if (!matches()) {
          return;
        }
        frame.removeEventListener("load", done);
        frame.dataset.loaded = "true";
        resolve();
      };
      frame.addEventListener("load", done);
      if (path && frame.src && matches()) {
        try {
          if (frame.contentDocument?.readyState === "complete") {
            done();
          }
        } catch {
          // cross-origin; wait for load event
        }
      }
    });
  }

  async onPreviewClick() {
    if (this.editMode) {
      await this.close();
    } else {
      await this.enterEditMode();
    }
  }

  async enterEditMode() {
    this.editMode = true;
    this.setPreviewButton(true);
    await this.open();
  }

  maybeAutoEnterEdit() {
    if (this.mode !== "edit" || this.editMode) {
      return;
    }
    const params = new URLSearchParams(window.location.search);
    if (params.get("edit") === "0") {
      return;
    }
    void this.enterEditMode();
  }

  initHankoUser() {
    const token = readHankoToken();
    if (!token) {
      return;
    }
    const payload = decodeJwtPayload(token);
    const username = payload?.username;
    if (!username) {
      return;
    }
    this._hankoUsername = username;
    console.log(username);
  }

  setPreviewButton(open) {
    this.previewButton.textContent = open ? "Done" : "Edit";
    this.previewButton.setAttribute(
      "aria-label",
      open ? "Done editing" : "Open editor"
    );
    this.revertButton.hidden = !open;
    this.publishButton.hidden = !open || !this._hankoUsername || !siteIdFromLocation();
    if (open) {
      void this.updateRevertButton();
      void this.updatePublishButton();
    } else {
      this.revertButton.disabled = true;
      this.publishButton.disabled = true;
    }
  }

  async updatePublishButton() {
    if (this.publishButton.hidden) {
      return;
    }
    const dirty = await hasCacheItems(LOCAL_CACHE_NAME);
    if (!dirty) {
      this.publishButton.disabled = true;
      return;
    }
    this.publishButton.disabled = !(await hasCacheItems(LOCAL_CACHE_NAME));
  }

  async updateRevertButton(dirty = null) {
    if (this.revertButton.hidden) {
      return;
    }
    if (dirty === null) {
      dirty = await hasCacheItems(LOCAL_CACHE_NAME);
    }
    this.revertButton.disabled = !dirty;
  }

  setBusy(loading) {
    this.previewButton.disabled = loading;
    this.previewButton.toggleAttribute("aria-busy", loading);
    if (loading) {
      this.revertButton.disabled = true;
      this.publishButton.disabled = true;
    } else if (this.editMode) {
      void this.updatePublishButton();
      void this.updateRevertButton();
    }
  }

  async revertCache() {
    if (swReady) {
      await swReady;
    }
    
    // Delete the local cache
    if ("caches" in globalThis) {
      await caches.delete(LOCAL_CACHE_NAME);
    }

    if (this.previewFrame) {
      delete this.previewFrame.dataset.loaded;
      this._previewReady = this.whenFrame(this.previewFrame);
      this.previewFrame.src = location.href;
      await this._previewReady;
    }
  }

  async onPublishClick() {
    const token = readHankoToken();
    if (!token || !siteIdFromLocation()) {
      return;
    }
    const files = await cacheFiles(LOCAL_CACHE_NAME);
    if (files.length === 0) {
      return;
    }

    this.setBusy(true);
    try {
      const form = new FormData();
      for (const file of files) {
        form.append(file.path, new Blob([file.blob], { type: mimeTypeFromFilename(file.path) }), file.path.split("/").pop());
   
      }

      const res = await fetch(siteApiUrl("/.push"), {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
        credentials: "include",
        body: form,
      });
      if (!res.ok) {
        throw new Error(await res.text());
      }
      void this.updatePublishButton();
      void this.updateRevertButton();
    } catch (err) {
      console.error("wanix-site: publish failed", err);
    } finally {
      this.setBusy(false);
    }
  }

  async onRevertClick() {
    if (!(await hasCacheItems(LOCAL_CACHE_NAME))) {
      return;
    }
    this.setBusy(true);
    try {
      await this.revertCache();
    } catch (err) {
      console.error("wanix-site: failed to revert cache", err);
    } finally {
      this.setBusy(false);
    }
  }

  async open() {
    this.setBusy(true);
    try {
      await this.ensureEditorReady();
      if (!this.editMode) {
        return;
      }
      this.showOverlay();
      this.overlay.classList.add("editor-ready");
      this.overlay.classList.add("editor");
    } finally {
      this.setBusy(false);
    }
  }

  transitionMs(el) {
    const raw = getComputedStyle(el).transitionDuration;
    return Math.max(
      0,
      ...raw.split(",").map((part) => {
        const n = parseFloat(part);
        return part.trim().endsWith("ms") ? n : n * 1000;
      })
    );
  }

  waitForPanelTransition(panel) {
    const duration = this.transitionMs(panel);
    if (duration <= 1) {
      return Promise.resolve();
    }
    return new Promise((resolve) => {
      const done = () => {
        panel.removeEventListener("transitionend", onEnd);
        clearTimeout(fallback);
        resolve();
      };
      const onEnd = (e) => {
        if (e.target === panel && e.propertyName === "width") {
          done();
        }
      };
      panel.addEventListener("transitionend", onEnd);
      const fallback = setTimeout(done, duration + 50);
    });
  }

  async close() {
    this.editMode = false;
    this.setPreviewButton(false);
    this.setBusy(true);

    if (this.overlay.classList.contains("editor")) {
      this.overlay.classList.remove("editor");
      await this.waitForPanelTransition(this.panel);
    }

    // If currently in "edit" mode, add edit=0 to the URL before reloading
    if (this.mode === "edit") {
      const url = new URL(window.location.href);
      url.searchParams.set("edit", "0");
      window.location.href = url.toString();
      return;
    } else {
      const url = new URL(window.location.href);
      url.searchParams.delete("edit");
      window.location.href = url.toString();
      return;
    }
  }
}

if (!customElements.get("wanix-site")) {
  customElements.define("wanix-site", WanixSite);
}


function siteApiUrl(path) {
  return path.startsWith("/") ? path : `/${path}`;
}

function readHankoToken() {
  try {
    const stored = localStorage.getItem(HANKO_STORAGE_KEY);
    if (stored) {
      return stored;
    }
  } catch {
    // ignore
  }
  for (const part of document.cookie.split(";")) {
    const trimmed = part.trim();
    const eq = trimmed.indexOf("=");
    if (eq === -1) {
      continue;
    }
    const name = trimmed.slice(0, eq);
    const value = trimmed.slice(eq + 1);
    if (name === HANKO_STORAGE_KEY && value) {
      return decodeURIComponent(value);
    }
  }
  return null;
}

function decodeJwtPayload(token) {
  const parts = token.split(".");
  if (parts.length !== 3) {
    return null;
  }
  try {
    const base64 = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    const padded = base64 + "=".repeat((4 - (base64.length % 4)) % 4);
    return JSON.parse(atob(padded));
  } catch {
    return null;
  }
}

async function cacheFiles(cacheName) {
  if (!("caches" in globalThis)) {
    return [];
  }
  const cache = await caches.open(cacheName);
  const keys = await cache.keys();
  return Promise.all(keys.map(async (key) => {
    const res = await cache.match(key);
    return {
      path: key.url
        .replace(location.origin, "")
        .replace("https://origin", "")
        .replace("http://origin", ""),
      blob: await res.blob(),
    };
  }));
}


async function hasCacheItems(cacheName) {
  if (!("caches" in globalThis)) {
    return new Map();
  }
  const cache = await caches.open(cacheName);
  return (await cache.keys()).length > 0;
}

function mimeTypeFromFilename(filename) {
  const ext = filename.split('.').pop().toLowerCase();
  switch (ext) {
    case "html":
    case "htm":
      return "text/html";
    case "js":
      return "application/javascript";
    case "mjs":
      return "application/javascript";
    case "css":
      return "text/css";
    case "json":
      return "application/json";
    case "txt":
      return "text/plain";
    case "xml":
      return "application/xml";
    case "svg":
      return "image/svg+xml";
    case "jpg":
    case "jpeg":
      return "image/jpeg";
    case "png":
      return "image/png";
    case "gif":
      return "image/gif";
    case "webp":
      return "image/webp";
    case "ico":
      return "image/x-icon";
    case "bmp":
      return "image/bmp";
    case "wasm":
      return "application/wasm";
    case "pdf":
      return "application/pdf";
    case "mp3":
      return "audio/mpeg";
    case "mp4":
      return "video/mp4";
    case "webm":
      return "video/webm";
    case "ogg":
      // Could be video or audio; default to audio
      return "audio/ogg";
    case "wav":
      return "audio/wav";
    case "csv":
      return "text/csv";
    case "zip":
      return "application/zip";
    case "tar":
      return "application/x-tar";
    case "woff":
      return "font/woff";
    case "woff2":
      return "font/woff2";
    case "ttf":
      return "font/ttf";
    case "otf":
      return "font/otf";
    default:
      return "application/octet-stream";
  }
}