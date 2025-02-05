FROM alpine:3.18
WORKDIR /v86
RUN mkdir -p /out

# build v86
RUN apk add --update curl clang make openjdk8-jre-base npm python3 git openssh
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y && PATH="${HOME}/.cargo/bin:${PATH}" rustup target add wasm32-unknown-unknown
RUN git clone --depth=1 https://github.com/copy/v86.git .


COPY ./patch/9p.js ./lib/9p.js
COPY ./patch/filesystem.js ./lib/filesystem.js
COPY ./patch/starter.js ./src/browser/starter.js


RUN PATH="${HOME}/.cargo/bin:${PATH}" make all && rm -rf closure-compiler gen lib src .cargo cargo.toml Makefile

RUN cp ./build/libv86.js /out
RUN cp ./build/v86.wasm /out
RUN cp ./bios/seabios.bin /out
RUN cp ./bios/vgabios.bin /out

CMD ["sh", "-c", "cp -r /out/* /dst"]