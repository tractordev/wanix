import { HANKO_STORAGE_KEY } from "./sso.js";

export function sessionTokenFromRequest(request) {
  const url = new URL(request.url);
  const fromQuery = url.searchParams.get("token");
  if (fromQuery) {
    return fromQuery;
  }

  const auth = request.headers.get("Authorization");
  if (auth) {
    const prefix = "Bearer ";
    if (auth.startsWith(prefix)) {
      const token = auth.slice(prefix.length).trim();
      if (token) {
        return token;
      }
    }
  }

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
    if (name === HANKO_STORAGE_KEY && value) {
      return decodeURIComponent(value);
    }
  }
  return null;
}

export function decodeJwtPayload(token) {
  const parts = token.split(".");
  if (parts.length !== 3) {
    return null;
  }
  try {
    const base64 = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    const padded = base64 + "=".repeat((4 - (base64.length % 4)) % 4);
    return JSON.parse(atob(padded));
  } catch {
    return null;
  }
}

export function usernameFromRequest(request) {
  const token = sessionTokenFromRequest(request);
  if (!token) {
    return null;
  }
  // TODO: validate JWT with Hanko before trusting claims
  const payload = decodeJwtPayload(token);
  return payload?.username ?? null;
}
