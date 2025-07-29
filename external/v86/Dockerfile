FROM alpine:3.18
WORKDIR /v86
RUN mkdir -p /out

# build v86
RUN apk add --update curl clang make openjdk8-jre-base npm python3 git openssh
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y && PATH="${HOME}/.cargo/bin:${PATH}" rustup target add wasm32-unknown-unknown
RUN git clone --shallow-since="2025-06-11" https://github.com/copy/v86.git . 
RUN git checkout 0eec29652b6fd38624fe28c9c8fe07a519f96820


COPY ./patch/9p.js ./lib/9p.js
COPY ./patch/starter.js ./src/browser/starter.js


RUN PATH="${HOME}/.cargo/bin:${PATH}" make all && rm -rf closure-compiler gen lib src .cargo cargo.toml Makefile

RUN cp ./build/libv86.js /out
RUN cp ./build/v86.wasm /out
RUN cp ./bios/seabios.bin /out
RUN cp ./bios/vgabios.bin /out
# ensure process is undefined since used to detect nodejs, but wasm_exec.js defines it
RUN cd /out && { echo "window.process = undefined;"; cat libv86.js; } > libv86.js.tmp && mv libv86.js.tmp libv86.js

CMD ["sh", "-c", "cp -r /out/* /dst"]