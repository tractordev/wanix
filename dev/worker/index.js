
export default {
    async fetch(request, env) {
        const url = new URL(request.url);

        if (url.hostname === 'wanix.sh') {
            let redirectUrl = `https://wanix.dev${url.pathname}${url.search}`;
            return Response.redirect(redirectUrl, 301);
        }
   
        if (url.pathname.startsWith('/lib/')) {
            // /lib/... or /lib/{version}/...
            let match = url.pathname.match(/^\/lib(?:\/([^\/]+))?\/(.+)$/);
            if (match) {
                let version = match[1];
                let rest = match[2];
                let redirectUrl = version
                    ? `https://cdn.jsdelivr.net/npm/wanix@${encodeURIComponent(version)}/dist/${rest}`
                    : `https://cdn.jsdelivr.net/npm/wanix/dist/${rest}`;
                return Response.redirect(redirectUrl, 302);
            }
        } else if (url.pathname.startsWith('/extras/')) {
            // /extras/... or /extras/{version}/...
            let match = url.pathname.match(/^\/extras(?:\/([^\/]+))?\/(.+)$/);
            if (match) {
                let version = match[1];
                let rest = match[2];
                let redirectUrl = version
                    ? `https://cdn.jsdelivr.net/npm/wanix-extras@${encodeURIComponent(version)}/dist/${rest}`
                    : `https://cdn.jsdelivr.net/npm/wanix-extras/dist/${rest}`;
                return Response.redirect(redirectUrl, 302);
            }
        }
        return await env.ASSETS.fetch(request);
    }
}
