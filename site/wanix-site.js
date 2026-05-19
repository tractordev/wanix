import { register, templateFetch, cacheFetch, ensureCache } from './wanix-sw.js';

const CACHE_NAME = "wanix-v1";
const CACHE_SENTINEL = "/index.html";
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

if (window === window.top) {
  register({
    onfetch: async (req) => {
      const cached = await cacheFetch(req, CACHE_NAME);
      if (cached) return cached;
  
      const template = await templateFetch(req);
      if (template) return template;
    },
    oninstall: async () => {
      await ensureCache(CACHE_NAME, [
        "/index.html", 
        ...performance.getEntriesByType('resource').map(r => r.name)
      ]);
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
        button {
          margin: 0;
          padding: 0.65rem 0.9rem;
          border: 1px solid #ddd;
          border-radius: 999px;
          background: #fff;
          box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
          font: 0.8125rem system-ui, sans-serif;
          cursor: pointer;
        }
        button:disabled {
          opacity: 0.55;
          cursor: wait;
        }
      </style>
      <button type="button" data-action="preview" aria-label="Open preview">Edit</button>
    `;
    this.previewButton = root.querySelector('[data-action="preview"]');
    this.overlay = null;
    this.panel = null;
    this.previewFrame = null;
    this.editorFrame = null;
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
    if (document.readyState === "complete") {
      void this.startBootstrap();
    } else {
      window.addEventListener("load", () => void this.startBootstrap(), {
        once: true,
      });
    }
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
    try {
      await this.waitForCache();
    } catch (err) {
      console.warn("wanix-site: cache not ready, bootstrapping anyway", err);
    }
    this.bootstrapFrames();
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
    this.previewButton.textContent = open ? "Close" : "Edit";
    this.previewButton.setAttribute(
      "aria-label",
      open ? "Close preview" : "Open preview"
    );
  }

  setBusy(loading) {
    this.previewButton.disabled = loading;
    this.previewButton.toggleAttribute("aria-busy", loading);
  }

  async open() {
    this.setBusy(true);
    try {
      await Promise.all([this._previewReady, this._editorReady]);
      if (!this.editMode) {
        return;
      }
      this.overlay.classList.add("editor");

      let lastHash = null;
      setInterval(async () => {
        if (window !== window.top) return;
        const hash = await hashCachedBody(CACHE_NAME, CACHE_SENTINEL);
        if (hash !== lastHash) {
          if (lastHash !== null) {
            this.previewFrame.contentWindow.location.reload();
            // // this is supposed to fix safari rendering issues but it doesnt work
            // this.previewFrame.addEventListener('load', () => nudge(this.previewFrame));
          }
          lastHash = hash;
        }
      }, 500);
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

async function hashCachedBody(cacheName, path, algorithm = 'SHA-256') {
  const cache = await caches.open(cacheName);
  const response = await cache.match(path);
  if (!response) return null;

  const buffer = await response.arrayBuffer();
  const hashBuffer = await crypto.subtle.digest(algorithm, buffer);

  return [...new Uint8Array(hashBuffer)]
    .map(b => b.toString(16).padStart(2, '0'))
    .join('');
}

function nudge(iframe) {
  console.log("nudging", iframe);
  iframe.style.transform = 'translateZ(0)';
  iframe.offsetHeight; // force reflow
  iframe.style.transform = '';
}
