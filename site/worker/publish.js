import mime from "mime";
import { usernameFromRequest } from "./auth.js";
import { siteIdFromRequest } from "./site.js";

function normalizePublishPath(path) {
  if (!path || typeof path !== "string") {
    return null;
  }
  let normalized = path.trim();
  if (!normalized.startsWith("/")) {
    normalized = `/${normalized}`;
  }
  if (normalized.includes("..") || !/^\/[a-zA-Z0-9/._-]+$/.test(normalized)) {
    return null;
  }
  return normalized;
}

function publishContentType(path, valueType) {
  if (path.endsWith(".html")) {
    return "text/html";
  }
  return valueType || mime.getType(path) || "application/octet-stream";
}

export async function handlePublish(request, env) {
  if (request.method !== "POST") {
    return new Response("method not allowed", { status: 405 });
  }

  const username = usernameFromRequest(request);
  if (!username) {
    console.log("no username", request.url);
    return new Response("unauthorized", { status: 401 });
  }

  const siteId = siteIdFromRequest(request);
  if (!siteId) {
    console.log("site not found", siteId, request.url);
    return new Response("forbidden", { status: 403 });
  }

  const indexKey = `/${siteId}/index.html`;
  const indexObject = await env.bucket.head(indexKey);
  if (!indexObject) {
    console.log("site index not found", siteId, indexKey);
    return new Response("forbidden", { status: 403 });
  }
  const owner = indexObject.customMetadata?.username;
  if (owner !== username) {
    console.log("unauthorized publish attempt", username, owner, request.url);
    return new Response("forbidden", { status: 403 });
  }

  const form = await request.formData();
  let wrote = 0;
  for (const [name, value] of form.entries()) {
    if (typeof value === "string") {
      continue;
    }
    const path = normalizePublishPath(name);
    if (!path) {
      continue;
    }
    const contentType = publishContentType(path, value.type);
    await env.bucket.put(`/${siteId}${path}`, value, {
      httpMetadata: { contentType },
      customMetadata: { username },
    });
    wrote += 1;
  }

  return Response.json({ ok: true, wrote });
}
