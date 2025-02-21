FROM alpine:3.20 AS wexec
WORKDIR /build
RUN apk add -u go
COPY wexec/main.go .
RUN GOOS=linux GOARCH=386 go build -o wexec main.go

FROM --platform=linux/386 i386/debian:12-slim AS lib32

FROM --platform=linux/x86_64 debian:bullseye AS preload
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


FROM debian:bullseye
# Install build dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    wget \
    bc \
    bzip2 \
    cpio \
    flex \
    bison \
    libelf-dev \
    gcc-i686-linux-gnu \
    binutils-i686-linux-gnu \
    && rm -rf /var/lib/apt/lists/*

# Create rootfs
WORKDIR /build
RUN mkdir -p /build/rootfs/bin \
    /build/rootfs/dev \
    /build/rootfs/proc \
    /build/rootfs/sys \
    /build/rootfs/etc \
    /build/rootfs/lib

# Get and build busybox
RUN wget https://busybox.net/downloads/busybox-1.35.0.tar.bz2 && \
    tar xf busybox-1.35.0.tar.bz2 && \
    rm busybox-1.35.0.tar.bz2

RUN cd /build/busybox-1.35.0 && \
    make defconfig ARCH=i386 CROSS_COMPILE=i686-linux-gnu- && \
    # sed -i 's/^# CONFIG_STATIC is not set/CONFIG_STATIC=y/' .config && \
    sed -i 's/^CONFIG_FEATURE_MOUNT_NFS=y/CONFIG_FEATURE_MOUNT_NFS=n/' .config && \
    sed -i 's/^CONFIG_FEATURE_INETD_RPC=y/CONFIG_FEATURE_INETD_RPC=n/' .config && \
    # LDFLAGS='-static'
    make ARCH=i386 CROSS_COMPILE=i686-linux-gnu- -j$(nproc) && \
    i686-linux-gnu-strip busybox && \
    cp busybox /build/rootfs/bin/ && \
    cd /build/rootfs/bin && \
    chmod +x busybox && \
    ln -s busybox sh

# Get and build kernel
RUN wget https://cdn.kernel.org/pub/linux/kernel/v5.x/linux-5.15.137.tar.xz && \
    tar xf linux-5.15.137.tar.xz && \
    rm linux-5.15.137.tar.xz

COPY kernel.config /build/linux-5.15.137/.config
RUN cd /build/linux-5.15.137 && \
    make ARCH=i386 CROSS_COMPILE=i686-linux-gnu- oldconfig < /dev/null && \
    make ARCH=i386 CROSS_COMPILE=i686-linux-gnu- bzImage -j$(nproc) && \
    cp arch/x86/boot/bzImage /build/

# Create network config
RUN echo "nameserver 8.8.8.8" > /build/rootfs/etc/resolv.conf

# Create init
COPY init /build/rootfs/init
RUN chmod +x /build/rootfs/init

# wexec 
COPY --from=wexec /build/wexec /build/rootfs/bin/wexec
RUN chmod +x /build/rootfs/bin/wexec

# Copy required shared libraries for i386
COPY --from=lib32 /lib/ld-linux.so.2 /build/rootfs/lib
COPY --from=lib32 /lib/i386-linux-gnu/libc.so.6 /build/rootfs/lib
COPY --from=lib32 /lib/i386-linux-gnu/libm.so.6 /build/rootfs/lib
COPY --from=lib32 /lib/i386-linux-gnu/libresolv.so.2 /build/rootfs/lib
COPY --from=lib32 /lib/i386-linux-gnu/libdl.so.2 /build/rootfs/lib
COPY --from=preload /build/preload.so /build/rootfs/lib

# Create initramfs
WORKDIR /build/rootfs
RUN find . | cpio -H newc -o | gzip > /build/initramfs.gz

# Copy outputs
RUN mkdir -p /output && \
    mv /build/bzImage /output/ && \
    mv /build/initramfs.gz /output/ && \
    chmod -R 777 /output

CMD ["/bin/sh", "-c", "cp -f /output/* /output-mount/"]