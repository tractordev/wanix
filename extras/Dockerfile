ARG BUILDPLATFORM
ARG TARGETPLATFORM
ARG LINUX_386=linux/386
ARG LINUX_AMD64=linux/amd64

ARG GO_VERSION=1.25.0
ARG TINYGO_VERSION=0.40.1
ARG ALPINE_VERSION=3.22

##
# Build wexec with tinygo for i386
FROM --platform=$BUILDPLATFORM tinygo/tinygo:$TINYGO_VERSION AS wexec
COPY wexec/main.go .
RUN GOOS=linux GOARCH=386 tinygo build -o wexec main.go

##
# Build kernel for alpine-linux
FROM --platform=$BUILDPLATFORM ghcr.io/progrium/linux-build:latest AS kernel
COPY alpine-linux/boot/kernel.config .config
RUN make ARCH=i386 CROSS_COMPILE=i686-linux-gnu- oldconfig < /dev/null && \
    make ARCH=i386 CROSS_COMPILE=i686-linux-gnu- bzImage -j$(nproc)

##
# Build alpine-linux with kernel and wexec
FROM --platform=$LINUX_386 docker.io/i386/alpine:$ALPINE_VERSION AS alpine-linux

FROM alpine:latest AS alpine-files
COPY --from=alpine-linux / /root/
COPY --from=kernel /build/arch/x86/boot/bzImage /root/boot/bzImage
COPY --from=wexec /home/tinygo/wexec /root/bin/
COPY ./alpine-linux/bin/* /root/bin/
COPY ./alpine-linux/etc/* /root/etc/
COPY ./alpine-linux/boot/* /root/boot/
RUN tar -C /root -czf /alpine-linux.tgz .

##
# v86 files
FROM --platform=$LINUX_AMD64 ghcr.io/progrium/v86:latest AS v86

FROM alpine:latest AS v86-files
COPY --from=v86 /libv86.js /dist/libv86.js
COPY --from=v86 /v86.wasm /dist/v86.wasm
COPY --from=v86 /bios/vgabios.bin /dist/vgabios.bin
COPY --from=v86 /bios/seabios.bin /dist/seabios.bin
RUN tar -C /dist -czf /v86.tgz .

##
# Build extras bundle
FROM scratch
COPY --from=alpine-files /alpine-linux.tgz /dist/alpine-linux.tgz
COPY --from=v86-files /v86.tgz /dist/v86.tgz
CMD ["true"]