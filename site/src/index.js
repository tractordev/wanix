import { handleNew } from "./new.js";
import { handle } from "./public.js";
import { handleSso } from "./sso.js";

function withCors(response) {
  const headers = new Headers(response.headers);
  headers.set("Access-Control-Allow-Origin", "*");
  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers,
  });
}

export default {
  async fetch(request, env) {
    const { pathname } = new URL(request.url);
    if (pathname === "/-/new" || pathname === "/-/new/") {
      return withCors(await handleNew(request, env));
    }
    if (pathname === "/-/sso" || pathname === "/-/sso/") {
      return withCors(await handleSso(request));
    }

    const response = await handle(request, env);
    if (response.status !== 404) {
      return withCors(response);
    }
    return withCors(await env.ASSETS.fetch(request));
  },
};
