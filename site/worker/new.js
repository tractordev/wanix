import mime from "mime";
import { usernameFromRequest } from "./auth.js";
import { siteLocation } from "./site.js";
import { xid } from "./xid.js";

const ASSET_PATHS = [
  "/index.html",
  "/style.css",
  "/editor.html",
  "/favicon.ico",
  // "/wanix-sw.js",
  // "/.lib/wanix-site.js",
  // "/.lib/site-url.js",
  // "/.lib/wanix.min.js",
  // "/.lib/wanix.debug.wasm",
];

async function copyAssets(env, xid, username) {
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
        customMetadata: { username },
      });
    }),
  );
}

export async function handleNew(request, env) {
  const username = usernameFromRequest(request);
  if (!username) {
    return new Response("unauthorized", { status: 401 });
  }

  const id = xid();
  await copyAssets(env, id, username);

  const url = siteLocation(request.url, id);
  return Response.redirect(url.toString(), 302);
}
