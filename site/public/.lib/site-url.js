export const SITE_ROOT = "wanix.site";

const XID_RE = /^[0-9a-v]{12,24}$/;

export function isXid(value) {
  return typeof value === "string" && XID_RE.test(value);
}

export function siteUrlConfig(siteRoot = SITE_ROOT) {
  return { siteRoot };
}

export function xidFromHost(hostname, siteRoot = SITE_ROOT) {
  const host = hostname.split(":")[0];
  if (host === siteRoot || host === "localhost") {
    return "";
  }
  if (host.endsWith(`.${siteRoot}`)) {
    const xid = host.slice(0, -(siteRoot.length + 1));
    return isXid(xid) ? xid : "";
  }
  const dot = host.indexOf(".");
  if (dot > 0) {
    const xid = host.slice(0, dot);
    const suffix = host.slice(dot + 1);
    if (isXid(xid) && suffix === "localhost") {
      return xid;
    }
  }
  return "";
}

export function parseUrl(url, config) {
  const base =
    typeof url === "string"
      ? url.startsWith("http")
        ? new URL(url)
        : new URL(url, `https://${config.siteRoot}`)
      : new URL(url);

  return {
    xid: xidFromHost(base.hostname, config.siteRoot),
    path: base.pathname || "/",
    search: base.search,
    hash: base.hash,
    origin: base.origin,
  };
}

function siteHostSuffix(hostname, config) {
  const host = hostname.split(":")[0];
  if (host === "localhost" || host.endsWith(".localhost")) {
    return "localhost";
  }
  return config.siteRoot;
}

export function siteOrigin(xid, config, fallbackOrigin = "") {
  if (!xid) {
    if (fallbackOrigin) {
      return new URL(fallbackOrigin).origin;
    }
    return `https://${config.siteRoot}`;
  }
  const base = fallbackOrigin
    ? new URL(fallbackOrigin)
    : new URL(`https://${config.siteRoot}`);
  const suffix = siteHostSuffix(base.hostname, config);
  const port = base.port ? `:${base.port}` : "";
  return `${base.protocol}//${xid}.${suffix}${port}`;
}

export function sitePath(_xid, path, _config) {
  return path.startsWith("/") ? path : `/${path}`;
}

export function siteUrl(xid, path, config, fallbackOrigin = "") {
  const normalized = sitePath(xid, path, config);
  return new URL(normalized, siteOrigin(xid, config, fallbackOrigin)).toString();
}

export function withSiteContext(target, source, config) {
  const ctx = parseUrl(source, config);
  const to =
    target instanceof URL
      ? new URL(target)
      : new URL(target, typeof source === "string" ? source : source.href || source);

  if (!ctx.xid) {
    return to;
  }

  const sourceUrl =
    typeof source === "string" ? new URL(source) : new URL(source.href);
  const suffix = siteHostSuffix(sourceUrl.hostname, config);
  to.protocol = sourceUrl.protocol;
  to.hostname = `${ctx.xid}.${suffix}`;
  to.port = sourceUrl.port;
  return to;
}

export function workbenchWd(location, _config) {
  const parsed = parseUrl(location.href, _config);
  const dir = parsed.path.replace(/\/[^/]*$/, "") || "";
  return `site/${location.host}${dir}`;
}

export function clientConfig() {
  return siteUrlConfig();
}

export function isInternalPathname(pathname) {
  const path = pathname || "/";
  return (
    path.startsWith("/.lib/") ||
    path === "/wanix-sw.js"
  );
}
