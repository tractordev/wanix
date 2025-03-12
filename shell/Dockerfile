##
# Multi-arch, multi-phase builder for Wanix shell
ARG X86_64=linux/x86_64
ARG X86_32=linux/386

##
# Build wexec with tinygo for i386
FROM tinygo/tinygo:0.36.0 AS wexec
COPY wexec/main.go .
RUN GOOS=linux GOARCH=386 tinygo build -o wexec main.go

##
# Build preload.so with gcc for i386
FROM --platform=$X86_64 debian:bullseye AS preload
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
FROM debian:bullseye AS busybox
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

##
# Grab i386 libraries
FROM --platform=$X86_32 i386/debian:12-slim AS lib32

##
# Build shell rootfs tarball
FROM debian:bullseye
WORKDIR /build
RUN mkdir -p \
    /build/rootfs/bin \
    /build/rootfs/dev \
    /build/rootfs/proc \
    /build/rootfs/lib
COPY --from=lib32 /lib/ld-linux.so.2 /build/rootfs/lib
COPY --from=lib32 /lib/i386-linux-gnu/libc.so.6 /build/rootfs/lib
COPY --from=lib32 /lib/i386-linux-gnu/libm.so.6 /build/rootfs/lib
COPY --from=lib32 /lib/i386-linux-gnu/libresolv.so.2 /build/rootfs/lib
COPY --from=lib32 /lib/i386-linux-gnu/libdl.so.2 /build/rootfs/lib
COPY --from=preload /build/preload.so /build/rootfs/lib
COPY --from=wexec /home/tinygo/wexec /build/rootfs/bin
COPY --from=busybox /build/busybox /build/rootfs/bin
COPY bin/* /build/rootfs/bin
RUN chmod +x /build/rootfs/bin/*
RUN tar -zcf /build/shell.tgz -C rootfs .
CMD ["/bin/sh", "-c", "cp -f /build/shell.tgz /output/"]