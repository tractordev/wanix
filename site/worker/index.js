import { handleNew } from "./new.js";
import { handlePublish } from "./publish.js";
import { handle } from "./public.js";
import { handleSso, handleSsoLogout } from "./sso.js";
import { parseRequest, isInternalPathname } from "./site.js";

function withCors(response, note) {
  const headers = new Headers(response.headers);
  headers.set("Access-Control-Allow-Origin", "*");
  if (note) {
    headers.set("X-Note", note);
  }
  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers,
  });
}

export default {
  async fetch(request, env) {
    const parsed = parseRequest(request);
    const { path: routePath, xid } = parsed;

    if (routePath === "/" && !xid) {
      return Response.redirect("https://github.com/tractordev/wanix", 302);
    }

    if (routePath === "/.clone" || routePath === "/.clone/") {
      return withCors(await handleNew(request, env));
    }
    if (routePath === "/.push" || routePath === "/.push/") {
      return withCors(await handlePublish(request, env));
    }
    if (routePath === "/.logout" || routePath === "/.logout/") {
      return withCors(await handleSsoLogout(request));
    }
    if (routePath === "/.sso" || routePath === "/.sso/") {
      return withCors(await handleSso(request));
    }

    if (isInternalPathname(routePath)) {
      const assetUrl = new URL(request.url);
      const assets = await env.ASSETS.fetch(new Request(assetUrl, request));
      if (assets.status !== 404) {
        return withCors(assets, "assets");
      }
    }

    if (!xid) {
      const assets = await env.ASSETS.fetch(request);
      if (assets.status !== 404) {
        return withCors(assets, "no-xid");
      }
    }

    const response = await handle(request, env);
    if (response.status !== 404) {
      return withCors(response, "r2");
    }

    return withCors(response, "r2-404");
  },
};
