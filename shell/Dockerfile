ARG BUILDPLATFORM
ARG TARGETPLATFORM
ARG X86_64=linux/x86_64

##
# Build wexec with tinygo for i386
FROM --platform=$BUILDPLATFORM tinygo/tinygo:0.36.0 AS wexec
COPY wexec/main.go .
RUN GOOS=linux GOARCH=386 tinygo build -o wexec main.go

##
# Build preload.so with gcc for i386
FROM --platform=$X86_64 ubuntu:noble AS preload
RUN apt-get update && apt-get install -y \
    gcc-10-multilib \
    g++-10-multilib \
    make \
    && rm -rf /var/lib/apt/lists/*
ENV CFLAGS="-m32"
ENV LDFLAGS="-m32"
WORKDIR /build
COPY preload/main.c .
RUN gcc -m32 -shared -fPIC -o preload.so main.c -ldl

##
# Build busybox with gcc for i386
FROM --platform=$BUILDPLATFORM debian:bullseye AS busybox
ARG BUSYBOX_VERSION=1.35.0
RUN apt-get update && apt-get install -y \
    build-essential \
    wget \
    gcc-i686-linux-gnu \
    binutils-i686-linux-gnu \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /build
RUN wget https://busybox.net/downloads/busybox-${BUSYBOX_VERSION}.tar.bz2 && \
    tar xf busybox-${BUSYBOX_VERSION}.tar.bz2 && \
    rm busybox-${BUSYBOX_VERSION}.tar.bz2
RUN cd busybox-${BUSYBOX_VERSION} && \
    make defconfig ARCH=i386 CROSS_COMPILE=i686-linux-gnu- && \
    sed -i 's/^CONFIG_FEATURE_MOUNT_NFS=y/CONFIG_FEATURE_MOUNT_NFS=n/' .config && \
    sed -i 's/^CONFIG_FEATURE_INETD_RPC=y/CONFIG_FEATURE_INETD_RPC=n/' .config && \
    # sed -i 's/^# CONFIG_STATIC is not set/CONFIG_STATIC=y/' .config && \
    # LDFLAGS='-static'
    make ARCH=i386 CROSS_COMPILE=i686-linux-gnu- -j$(nproc) && \
    i686-linux-gnu-strip busybox && \
    chmod +x busybox && \
    mv busybox /build

FROM --platform=$BUILDPLATFORM ghcr.io/progrium/linux-build:latest AS kernel
COPY kernel.config .config
RUN make ARCH=i386 CROSS_COMPILE=i686-linux-gnu- oldconfig < /dev/null && \
    make ARCH=i386 CROSS_COMPILE=i686-linux-gnu- bzImage -j$(nproc)

FROM ghcr.io/progrium/v86:latest AS v86

##
# Grab i386 libraries
FROM --platform=$TARGETPLATFORM i386/debian:12-slim AS lib32
RUN mkdir -p /empty

##
# Build shell bundle
FROM scratch
COPY --from=lib32 /empty /rootfs/bin
COPY --from=lib32 /empty /rootfs/dev
COPY --from=lib32 /empty /rootfs/etc
COPY --from=lib32 /empty /rootfs/proc
COPY --from=lib32 /empty /rootfs/lib
COPY --from=lib32 /lib/ld-linux.so.2 /rootfs/lib/
COPY --from=lib32 /lib/i386-linux-gnu/libc.so.6 /rootfs/lib/
COPY --from=lib32 /lib/i386-linux-gnu/libm.so.6 /rootfs/lib/
COPY --from=lib32 /lib/i386-linux-gnu/libresolv.so.2 /rootfs/lib/
COPY --from=lib32 /lib/i386-linux-gnu/libdl.so.2 /rootfs/lib/
COPY --from=preload /build/preload.so /rootfs/lib/
COPY --from=wexec /home/tinygo/wexec /rootfs/bin/
COPY --from=busybox /build/busybox /rootfs/bin/
COPY --from=kernel /build/arch/x86/boot/bzImage /kernel/bzImage
COPY --from=v86 /libv86.js /v86/libv86.js
COPY --from=v86 /v86.wasm /v86/v86.wasm
COPY --from=v86 /bios/seabios.bin /v86/seabios.bin
COPY --from=v86 /bios/vgabios.bin /v86/vgabios.bin
COPY bin/* /rootfs/bin
COPY etc/* /rootfs/etc
COPY init.js /init.js
