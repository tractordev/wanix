FROM alpine:latest
WORKDIR /build
RUN apk add --no-cache nodejs npm esbuild
COPY ./package.json .
RUN npm install
COPY . .
RUN esbuild index.ts --bundle --outfile=wasi.bundle.js --format=esm
CMD ["/bin/sh", "-c", "cp -f /build/wasi.bundle.js /output/"]