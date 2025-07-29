FROM alpine:latest AS js
WORKDIR /build
RUN apk add --no-cache nodejs npm esbuild
COPY ./package.json .
RUN npm install
COPY . .
RUN esbuild index.ts \
            --bundle \
            # --minify \
            --loader:.go.js=text \
            --loader:.tinygo.js=text \
            --outfile=wanix.min.js
CMD ["/bin/sh", "-c", "cp -f /build/wanix.min.js /output/"]