const HANKO_COOKIE = "hanko";

// Cookie name matches what Hanko clients expect; we store the token as-is.
// TODO: make configurable
const SIGN_IN_URL = "https://io.wanix.site/ident/login";

function safeRedirect(value) {
  if (!value || !value.startsWith("/") || value.startsWith("//")) {
    return "/";
  }
  return value;
}

function hankoCookie(request) {
  const header = request.headers.get("Cookie");
  if (!header) {
    return null;
  }
  for (const part of header.split(";")) {
    const trimmed = part.trim();
    const eq = trimmed.indexOf("=");
    if (eq === -1) {
      continue;
    }
    const name = trimmed.slice(0, eq);
    const value = trimmed.slice(eq + 1);
    if (name === HANKO_COOKIE && value) {
      return decodeURIComponent(value);
    }
  }
  return null;
}

function redirectResponse(location, { setCookie } = {}) {
  const headers = { Location: location };
  if (setCookie) {
    headers["Set-Cookie"] = setCookie;
  }
  return new Response(null, { status: 302, headers });
}

function hankoSetCookie(token) {
  return `${HANKO_COOKIE}=${encodeURIComponent(token)}; Path=/; HttpOnly; SameSite=Lax`;
}

export async function handleSso(request) {
  const url = new URL(request.url);
  const redirect = safeRedirect(url.searchParams.get("redirect"));
  const token = url.searchParams.get("token");

  if (token) {
    return redirectResponse(redirect, {
      setCookie: hankoSetCookie(token),
    });
  }

  const existing = hankoCookie(request);
  if (existing) {
    return redirectResponse(redirect);
  }

  const callback = new URL(request.url);
  callback.searchParams.delete("token");
  if (!callback.searchParams.has("redirect")) {
    callback.searchParams.set("redirect", redirect);
  }

  const signIn = new URL(SIGN_IN_URL);
  signIn.searchParams.set("redirect", callback.toString());
  return redirectResponse(signIn.toString());
}
