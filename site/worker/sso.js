import { config, withSiteContext } from "./site.js";

export const HANKO_STORAGE_KEY = "hanko";

// TODO: make configurable
const SIGN_IN_URL = "https://io.wanix.site/ident/login";

function redirectPath(value, requestUrl) {
  if (!value || !value.startsWith("/") || value.startsWith("//")) {
    value = "/";
  }
  const target = withSiteContext(new URL(value, requestUrl), requestUrl, config());
  return `${target.pathname}${target.search}`;
}

function htmlPage(script) {
  return new Response(
    `<!DOCTYPE html><html><head><meta charset="utf-8"></head><body><script>${script}</script></body></html>`,
    { headers: { "Content-Type": "text/html; charset=utf-8" } }
  );
}

function signInUrl(callback) {
  const signIn = new URL(SIGN_IN_URL);
  signIn.searchParams.set("redirect", callback.toString());
  return signIn.toString();
}

const hankoSessionScript = `
const __hankoKey = ${JSON.stringify(HANKO_STORAGE_KEY)};
function __setHankoSession(token) {
  localStorage.setItem(__hankoKey, token);
  document.cookie = __hankoKey + "=" + encodeURIComponent(token) + "; path=/; SameSite=Lax";
}
function __hasHankoSession() {
  if (localStorage.getItem(__hankoKey)) {
    return true;
  }
  for (const part of document.cookie.split(";")) {
    const trimmed = part.trim();
    const eq = trimmed.indexOf("=");
    if (eq === -1) {
      continue;
    }
    if (trimmed.slice(0, eq) === __hankoKey && trimmed.slice(eq + 1)) {
      return true;
    }
  }
  return false;
}
function __clearHankoSession() {
  localStorage.removeItem(__hankoKey);
  document.cookie = __hankoKey + "=; path=/; Max-Age=0; SameSite=Lax";
}
`.trim();

export async function handleSso(request) {
  const url = new URL(request.url);
  const redirect = redirectPath(url.searchParams.get("redirect"), request.url);
  const token = url.searchParams.get("token");

  if (token) {
    return htmlPage(`
${hankoSessionScript}
__setHankoSession(${JSON.stringify(token)});
location.replace(${JSON.stringify(redirect)});
    `.trim());
  }

  const callback = withSiteContext(new URL(request.url), request.url, config());
  callback.searchParams.delete("token");
  if (!callback.searchParams.has("redirect")) {
    callback.searchParams.set("redirect", redirect);
  }

  return htmlPage(`
${hankoSessionScript}
if (__hasHankoSession()) {
  location.replace(${JSON.stringify(redirect)});
} else {
  location.replace(${JSON.stringify(signInUrl(callback))});
}
  `.trim());
}

export async function handleSsoLogout(request) {
  const url = new URL(request.url);
  const redirect = redirectPath(url.searchParams.get("redirect"), request.url);

  return htmlPage(`
${hankoSessionScript}
__clearHankoSession();
location.replace(${JSON.stringify(redirect)});
  `.trim());
}
