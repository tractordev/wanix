import mime from "mime";
import { parseRequest } from "./site.js";

export async function handle(req, env) {
  const parsed = parseRequest(req);
  const site = parsed.xid;
  const url = new URL(req.url);
  const sitePathname = parsed.path;

  if (!sitePathname.endsWith("/") && !/\.[^/]+$/.test(sitePathname)) {
    url.pathname = `${sitePathname}/`;
    return Response.redirect(url.toString(), 302);
  }

  if (!site) {
    return new Response("Site not found", { status: 404 });
  }

  const cacheControl = {  }; //"Cache-Control": "no-store"

  const bucketPath = `/${site}`;
  let objectKey = `${bucketPath}${sitePathname}`;
  let object = await env.bucket.get(objectKey);
  if (
    !object ||
    object.customMetadata?.["Content-Type"] === "application/x-directory"
  ) {
    objectKey = `${bucketPath}${sitePathname}/index.html`.replace(/\/{2,}/g, "/");
    object = await env.bucket.get(objectKey);
    if (!object) {
      object = await env.bucket.get(`${bucketPath}/404.html`);
      if (object) {
        return new Response(object.body, {
          headers: {
            "Content-Type":
              object.httpMetadata.contentType ||
              mime.getType(sitePathname) ||
              "text/html",
            ...cacheControl,
          },
          status: 404,
        });
      }
      return new Response("Not found", { status: 404 });
    }
  }

  return new Response(object.body, {
    headers: {
      "Content-Type":
        object.httpMetadata.contentType ||
        mime.getType(sitePathname) ||
        "text/html",
      ...cacheControl,
    },
  });
}
