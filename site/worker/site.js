import { env } from "cloudflare:workers";
import {
  clientConfig,
  isInternalPathname,
  parseUrl,
  sitePath,
  siteUrl,
  siteUrlConfig,
  withSiteContext,
} from "../public/.lib/site-url.js";

export function config() {
  return siteUrlConfig(env.SITE_ROOT || undefined);
}

export function parseRequest(request) {
  return parseUrl(request.url, config());
}

export function siteIdFromRequest(request) {
  return parseRequest(request).xid;
}

export function siteLocation(requestUrl, xid) {
  return new URL(siteUrl(xid, "/", config(), new URL(requestUrl).origin));
}

export {
  parseUrl,
  siteUrl,
  sitePath,
  withSiteContext,
  clientConfig,
  siteUrlConfig,
  isInternalPathname as isInternalPathname,
};
