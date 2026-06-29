import mime from "mime";
import { xid } from "./xid.js";

const ASSET_PATHS = [
  "/index.html",
  "/wanix-sw.js",
  "/lib/wanix-site.js",
  "/lib/wanix.min.js",
  "/lib/wanix.debug.wasm",
];

function siteRoot(hostname) {
  const host = hostname.split(":")[0];
  if (host === "localhost" || /^\d+\.\d+\.\d+\.\d+$/.test(host)) {
    return host;
  }
  if (host.endsWith(".localhost")) {
    return "localhost";
  }
  const parts = host.split(".");
  if (parts.length <= 2) {
    return host;
  }
  return parts.slice(1).join(".");
}

async function copyAssets(env, xid) {
  const base = `/${xid}`;

  await Promise.all(
    ASSET_PATHS.map(async (path) => {
      const res = await env.ASSETS.fetch(
        new Request(`https://assets.internal${path}`),
      );
      if (!res.ok) {
        return;
      }

      const contentType =
        res.headers.get("content-type") ||
        mime.getType(path) ||
        "application/octet-stream";

      await env.bucket.put(`${base}${path}`, res.body, {
        httpMetadata: { contentType },
      });
    }),
  );
}

export async function handleNew(request, env) {
  const id = xid();
  await copyAssets(env, id);

  const url = new URL(request.url);
  url.hostname = `${id}.${siteRoot(url.hostname)}`;
  url.pathname = "/";
  url.search = "";
  return Response.redirect(url.toString(), 302);
}
