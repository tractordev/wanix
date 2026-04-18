FROM golang:1.25.0-alpine3.22 AS base
RUN apk add --no-cache \
    nodejs \
    npm \
    git \
    esbuild \
    make

FROM base AS js
WORKDIR /build
COPY ./package.json .
RUN npm install
COPY . .
RUN esbuild index-handle.ts \
    --outfile=dist/wanix.handle.js \
    --bundle \
    --external:util \
    --format=esm \
    --minify
RUN esbuild index.ts \
    --outfile=dist/wanix.min.js \
    --bundle \
    --external:util \
    --loader:.go.js=text \
    --loader:.tinygo.js=text \
    --format=esm \
    --minify
RUN esbuild index.ts \
    --outfile=dist/wanix.js \
    --bundle \
    --external:util \
    --loader:.go.js=text \
    --loader:.tinygo.js=text \
    --format=esm
RUN esbuild wasi/mod.ts \
    --outfile=wasi/worker/lib.js \
    --bundle \
    --external:util \
    --format=esm
RUN esbuild gojs/mod.ts \
    --outfile=gojs/worker/lib.js \
    --bundle \
    --external:util \
    --format=esm

FROM tinygo/tinygo:0.39.0 AS wasm-tinygo
WORKDIR /build
USER root
ENV GOFLAGS="-buildvcs=false"
RUN git config --global --add safe.directory /build
COPY ./hack/cbor ./hack/cbor
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=js /build/wasi/worker/lib.js /build/wasi/worker/lib.js
RUN make wasm-tinygo

FROM base AS go-base
WORKDIR /build
COPY ./hack/cbor ./hack/cbor
COPY go.mod go.sum ./
RUN go mod download

FROM go-base AS wasm-go
COPY . .
COPY --from=js /build/wasi/worker/lib.js /build/wasi/worker/lib.js
RUN make wasm-go


FROM go-base AS cmd
COPY . .
ARG GOOS=linux
ARG GOARCH=amd64
COPY --from=js /build/wasi/worker/lib.js /build/wasi/worker/lib.js
COPY --from=js /build/dist/wanix.min.js /build/dist/wanix.min.js
COPY --from=js /build/dist/wanix.handle.js /build/dist/wanix.handle.js
COPY --from=wasm-go /build/dist/wanix.debug.wasm /build/dist/wanix.debug.wasm
COPY --from=wasm-tinygo /build/dist/wanix.wasm /build/dist/wanix.wasm
RUN make cmd

FROM scratch AS runtime-dist
WORKDIR /
COPY --from=js /build/dist/wanix.min.js /wanix.min.js
COPY --from=js /build/dist/wanix.js /wanix.js
COPY --from=js /build/dist/wanix.handle.js /wanix.handle.js
COPY --from=wasm-go /build/dist/wanix.debug.wasm /wanix.debug.wasm
COPY --from=wasm-tinygo /build/dist/wanix.wasm /wanix.wasm
CMD ["true"]

FROM scratch AS dist
WORKDIR /
COPY --from=runtime-dist /* .
COPY --from=cmd /build/.local/bin/wanix /wanix
CMD ["true"]