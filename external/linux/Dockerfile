FROM ubuntu:noble
ARG KERNEL_VERSION=6.12.18
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
RUN wget https://cdn.kernel.org/pub/linux/kernel/v6.x/linux-${KERNEL_VERSION}.tar.xz && \
    tar xf linux-${KERNEL_VERSION}.tar.xz && \
    rm linux-${KERNEL_VERSION}.tar.xz
COPY kernel.config /build/linux-${KERNEL_VERSION}/.config
RUN cd /build/linux-${KERNEL_VERSION} && \
    make ARCH=i386 CROSS_COMPILE=i686-linux-gnu- oldconfig < /dev/null && \
    make ARCH=i386 CROSS_COMPILE=i686-linux-gnu- bzImage -j$(nproc) && \
    cp arch/x86/boot/bzImage /build/
CMD ["/bin/sh", "-c", "cp -f /build/bzImage /output/"]