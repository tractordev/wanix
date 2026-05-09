ARG GO_VERSION=1.25.0
ARG TINYGO_VERSION=0.41.1
ARG ALPINE_VERSION=3.22

ARG BUILDPLATFORM
ARG TARGETPLATFORM
ARG LINUX_386=linux/386
ARG LINUX_AMD64=linux/amd64
ARG GOOS
ARG GOARCH
ARG HOSTEXPORT=hostexport-go

FROM --platform=$BUILDPLATFORM tinygo/tinygo:${TINYGO_VERSION} AS tinygo-buildbase
WORKDIR /build
USER root
ENV GOFLAGS="-buildvcs=false"
RUN git config --global --add safe.directory /build
COPY ./misc/cbor ./misc/cbor
COPY go.mod go.sum ./
RUN go mod download
COPY . .

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS go-buildbase
WORKDIR /build

## WANIX CORE

FROM tinygo-buildbase AS wasm-tinygo
RUN make wasm-tinygo

FROM go-buildbase AS build
RUN apk add --no-cache \
    nodejs \
    npm \
    git \
    esbuild \
    make
COPY ./package.json .
RUN npm install
COPY ./misc/cbor ./misc/cbor
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make js wasm
ARG GOOS
ARG GOARCH
RUN make cmd

## WANIX EXTRAS

##
# Pull kernel and alpine images
FROM --platform=$LINUX_AMD64 ghcr.io/tractordev/apptron:kernel AS kernel
# FROM --platform=$BUILDPLATFORM ghcr.io/progrium/linux-build:latest AS kernel
# COPY alpine-linux/boot/kernel.config .config
# RUN make ARCH=i386 CROSS_COMPILE=i686-linux-gnu- oldconfig < /dev/null && \
#     make ARCH=i386 CROSS_COMPILE=i686-linux-gnu- bzImage -j$(nproc)
FROM --platform=$LINUX_386 docker.io/i386/alpine:$ALPINE_VERSION AS alpine-root

FROM tinygo-buildbase AS wexec
RUN GOOS=linux GOARCH=386 tinygo build -o wexec ./extras/wexec/main.go

FROM tinygo-buildbase AS hostexport-tinygo
RUN GOOS=linux GOARCH=386 tinygo build -o hostexport ./extras/hostexport/main.go

FROM go-buildbase AS hostexport-go
COPY ./misc/cbor ./misc/cbor
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN GOOS=linux GOARCH=386 go build -o hostexport ./extras/hostexport/main.go

FROM ${HOSTEXPORT} AS hostexport

FROM alpine:latest AS extras-alpine
COPY --from=alpine-root / /root/
COPY --from=kernel /bzImage /root/boot/bzImage
COPY --from=wexec /build/wexec /root/bin/
COPY --from=hostexport /build/hostexport /root/bin/
COPY ./extras/alpine-linux/bin/* /root/bin/
COPY ./extras/alpine-linux/etc/* /root/etc/
COPY ./extras/alpine-linux/boot/* /root/boot/
RUN tar -C /root -czf /alpine-linux.tgz .


## FINAL IMAGE

FROM scratch AS dist
WORKDIR /dist
COPY --from=build /build/dist/* .
COPY --from=build /build/.local/bin/wanix ./wanix
COPY --from=wasm-tinygo /build/dist/wanix.wasm ./wanix.wasm
CMD ["true"]