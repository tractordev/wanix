FROM debian:bullseye
WORKDIR /build
RUN apt-get update && apt-get install -y \
    build-essential \
    wget \
    bc \
    flex \
    bison \
    libelf-dev \
    gcc-i686-linux-gnu \
    binutils-i686-linux-gnu \
    && rm -rf /var/lib/apt/lists/*
RUN wget https://cdn.kernel.org/pub/linux/kernel/v5.x/linux-5.15.137.tar.xz && \
    tar xf linux-5.15.137.tar.xz && \
    rm linux-5.15.137.tar.xz
COPY kernel.config /build/linux-5.15.137/.config
RUN cd /build/linux-5.15.137 && \
    make ARCH=i386 CROSS_COMPILE=i686-linux-gnu- oldconfig < /dev/null && \
    make ARCH=i386 CROSS_COMPILE=i686-linux-gnu- bzImage -j$(nproc) && \
    cp arch/x86/boot/bzImage /build/
CMD ["/bin/sh", "-c", "cp -f /build/bzImage /output/"]