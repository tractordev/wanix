import mime from 'mime';

export async function handle(req, env) {
    const url = new URL(req.url);
    
    // Check if url.pathname does not end with "/" and does not include a file extension
    if (!url.pathname.endsWith("/") && !/\.[^\/]+$/.test(url.pathname)) {
        url.pathname = url.pathname + "/";
        return Response.redirect(url.toString(), 302);
    }

    
    let site = "";
    const baseHost = url.hostname.split(":")[0];
    if (baseHost.endsWith(".localhost")) {
        site = baseHost.split(".")[0];
    } else if (
        baseHost !== "localhost" &&
        !/^\d+\.\d+\.\d+\.\d+$/.test(baseHost) &&
        baseHost.split(".").length > 2
    ) {
        site = baseHost.split(".")[0];
    }

    if (!site) {
        return new Response('Site not found', { status: 404 });
    }
    
    let sitePath = `/${site}`;
    let objectKey = `${sitePath}${url.pathname}`;
    let object = await env.bucket.get(objectKey);
    if (!object || object.customMetadata["Content-Type"] === "application/x-directory") {
        objectKey = `${sitePath}${url.pathname}/index.html`.replace(/\/{2,}/g, "/");
        object = await env.bucket.get(objectKey);
        if (!object) {
            object = await env.bucket.get(`${sitePath}/404.html`);
            if (object) {
                return new Response(object.body, {
                    headers: {
                        'Content-Type': object.httpMetadata.contentType || mime.getType(url.pathname) || 'text/html',
                    },
                    status: 404,
                });
            } else {
                return new Response('Not found', { status: 404 });
            }
        }
    }

    return new Response(object.body, {
        headers: {
            'Content-Type': object.httpMetadata.contentType || mime.getType(url.pathname) || 'text/html',
        },
    });
}
