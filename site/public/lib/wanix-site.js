import { register, templateFetch, cacheFetch, ensureCache, cacheTemplate } from '../wanix-sw.js';

const CACHE_NAME = "wanix-v1";
const CACHE_SENTINEL = "/index.html";
const CACHE_POLL_INTERVAL = 500;
const OVERLAY_STYLE_ID = "wanix-site-overlay-styles";

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
  }

  #site-preview {
    flex: 1;
    min-width: 0;
    border: 0;
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
      const cached = await cacheFetch(req, CACHE_NAME);
      if (cached) return cached;
  
      const template = await templateFetch(req);
      if (template) return template;
    },
    oninstall: async () => {
      const resources = performance.getEntriesByType('resource').map(r => r.name);
      await ensureCache(CACHE_NAME, [
        "/",
        "/index.html", 
        "/lib/wanix.min.js",
        "/lib/wanix.debug.wasm",
        ...resources,
      ]);
      await cacheTemplate(CACHE_NAME, "/_editor.html");
    },
  });
}


class WanixSite extends HTMLElement {
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
        <button type="button" data-action="preview" aria-label="Open editor">Edit</button>
      </div>
    `;
    this.revertButton = root.querySelector('[data-action="revert"]');
    this.previewButton = root.querySelector('[data-action="preview"]');
    this.overlay = null;
    this.panel = null;
    this.previewFrame = null;
    this.editorFrame = null;
    this._cacheBaseline = null;
    this._cacheWatchTimer = null;
    this._lastEventFingerprint = null;
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

    panel.append(editor);
    overlay.append(panel, preview);
    document.body.append(overlay);

    this.overlay = overlay;
    this.panel = panel;
    this.editorFrame = editor;
    this.previewFrame = preview;
  }

  connectedCallback() {
    if (window !== window.top) {
      this.style.display = "none";
      return;
    }
    this.createOverlay();
    this.previewButton.addEventListener("click", () => {
      void this.onPreviewClick();
    });
    this.revertButton.addEventListener("click", () => {
      void this.onRevertClick();
    });
    if (document.readyState === "complete") {
      void this.startBootstrap();
    } else {
      window.addEventListener("load", () => void this.startBootstrap(), {
        once: true,
      });
    }
  }

  disconnectedCallback() {
    this.stopCacheWatch();
  }

  async waitForCache(
    name = CACHE_NAME,
    path = CACHE_SENTINEL,
    { interval = 50, timeout = 60_000 } = {}
  ) {
    const deadline = Date.now() + timeout;
    while (Date.now() < deadline) {
      if ("caches" in globalThis) {
        const cache = await caches.open(name);
        if (await cache.match(path)) {
          return cache;
        }
      }
      await new Promise((resolve) => setTimeout(resolve, interval));
    }
    throw new Error(`cache ${name} did not contain ${path} within ${timeout}ms`);
  }

  async startBootstrap() {
    if (swReady) {
      await swReady;
    }
    try {
      await this.waitForCache();
      await this.captureCacheBaseline();
      this.startCacheWatch();
    } catch (err) {
      console.warn("wanix-site: cache not ready, bootstrapping anyway", err);
    }
    this.bootstrapFrames();
  }

  async captureCacheBaseline(cacheName = CACHE_NAME) {
    this._cacheBaseline = await snapshotCache(cacheName);
    this._lastEventFingerprint = snapshotFingerprint(this._cacheBaseline);
    return this._cacheBaseline;
  }

  async resetCacheBaseline(cacheName = CACHE_NAME) {
    return this.captureCacheBaseline(cacheName);
  }

  async hasCacheChanges(cacheName = CACHE_NAME) {
    if (!this._cacheBaseline) {
      return false;
    }
    const current = await snapshotCache(cacheName);
    return diffCacheSnapshots(this._cacheBaseline, current).changed;
  }

  async getCacheChanges(cacheName = CACHE_NAME) {
    if (!this._cacheBaseline) {
      return { added: [], removed: [], modified: [], changed: false };
    }
    const current = await snapshotCache(cacheName);
    return diffCacheSnapshots(this._cacheBaseline, current);
  }

  startCacheWatch({ interval = CACHE_POLL_INTERVAL, cacheName = CACHE_NAME } = {}) {
    this.stopCacheWatch();
    this._cacheWatchTimer = setInterval(() => {
      void this._pollCacheChanges(cacheName);
    }, interval);
  }

  stopCacheWatch() {
    if (this._cacheWatchTimer !== null) {
      clearInterval(this._cacheWatchTimer);
      this._cacheWatchTimer = null;
    }
  }

  async _pollCacheChanges(cacheName) {
    if (window !== window.top || !this._cacheBaseline) {
      return;
    }

    const current = await snapshotCache(cacheName);
    const diff = diffCacheSnapshots(this._cacheBaseline, current);
    if (!diff.changed) {
      this._lastEventFingerprint = snapshotFingerprint(current);
      if (this.editMode) {
        void this.updateRevertButton(false);
      }
      return;
    }

    const fingerprint = snapshotFingerprint(current);
    if (fingerprint === this._lastEventFingerprint) {
      if (this.editMode) {
        void this.updateRevertButton(true);
      }
      return;
    }
    this._lastEventFingerprint = fingerprint;

    this.dispatchEvent(
      new CustomEvent("cachechange", {
        bubbles: true,
        composed: true,
        detail: diff,
      })
    );

    if (this.editMode) {
      void this.updateRevertButton(true);
    }

    if (this.editMode && this.previewFrame?.contentWindow) {
      this.previewFrame.contentWindow.location.reload();
    }
  }

  bootstrapFrames() {
    this._previewReady = this.whenFrame(this.previewFrame);
    this._editorReady = this.whenFrame(this.editorFrame);
    this.previewFrame.src = location.href;
    this.editorFrame.src = "/_editor.html";

    this._previewReady.then(() => {
      this.overlay.hidden = false;
      this.overlay.classList.add("ready");
      document.body.classList.add("lock");
    });
  }

  whenFrame(frame) {
    if (frame.dataset.loaded === "true") {
      return Promise.resolve();
    }
    return new Promise((resolve) => {
      frame.addEventListener(
        "load",
        () => {
          frame.dataset.loaded = "true";
          resolve();
        },
        { once: true }
      );
    });
  }

  async onPreviewClick() {
    this.editMode = !this.editMode;
    this.setPreviewButton(this.editMode);
    if (this.editMode) {
      await this.open();
    } else {
      await this.close();
    }
  }

  setPreviewButton(open) {
    this.previewButton.textContent = open ? "Done" : "Edit";
    this.previewButton.setAttribute(
      "aria-label",
      open ? "Done editing" : "Open editor"
    );
    this.revertButton.hidden = !open;
    if (open) {
      void this.updateRevertButton();
    } else {
      this.revertButton.disabled = true;
    }
  }

  async updateRevertButton(dirty = null) {
    if (this.revertButton.hidden) {
      return;
    }
    if (dirty === null) {
      dirty = await this.hasCacheChanges();
    }
    this.revertButton.disabled = !dirty;
  }

  setBusy(loading) {
    this.previewButton.disabled = loading;
    this.previewButton.toggleAttribute("aria-busy", loading);
    if (loading) {
      this.revertButton.disabled = true;
    } else if (this.editMode) {
      void this.updateRevertButton();
    }
  }

  async onRevertClick() {
    if (!(await this.hasCacheChanges())) {
      return;
    }
    this.setBusy(true);
    try {
      if ("caches" in globalThis) {
        await caches.delete(CACHE_NAME);
      }
      window.location.reload();
    } catch (err) {
      console.error("wanix-site: failed to revert cache", err);
      this.setBusy(false);
    }
  }

  async open() {
    this.setBusy(true);
    try {
      await Promise.all([this._previewReady, this._editorReady]);
      if (!this.editMode) {
        return;
      }
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

    window.location.reload();
  }
}

if (!customElements.get("wanix-site")) {
  customElements.define("wanix-site", WanixSite);
}

async function hashResponse(response, algorithm = "SHA-256") {
  const buffer = await response.arrayBuffer();
  const hashBuffer = await crypto.subtle.digest(algorithm, buffer);
  return [...new Uint8Array(hashBuffer)]
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

async function snapshotCache(cacheName) {
  if (!("caches" in globalThis)) {
    return new Map();
  }
  const cache = await caches.open(cacheName);
  const keys = await cache.keys();
  const entries = new Map();
  for (const req of keys) {
    const res = await cache.match(req);
    if (res) {
      entries.set(req.url, await hashResponse(res));
    }
  }
  return entries;
}

function snapshotFingerprint(entries) {
  return [...entries.entries()]
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([key, hash]) => `${key}\0${hash}`)
    .join("\n");
}

function diffCacheSnapshots(baseline, current) {
  const added = [];
  const removed = [];
  const modified = [];
  for (const [key, hash] of current) {
    if (!baseline.has(key)) {
      added.push(key);
    } else if (baseline.get(key) !== hash) {
      modified.push(key);
    }
  }
  for (const key of baseline.keys()) {
    if (!current.has(key)) {
      removed.push(key);
    }
  }
  return {
    added,
    removed,
    modified,
    changed: added.length > 0 || removed.length > 0 || modified.length > 0,
  };
}
