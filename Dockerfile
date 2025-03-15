FROM alpine:latest
ARG GOOS=linux
ARG GOARCH=amd64
ARG GOARGS="-tags noconsole"

WORKDIR /build
RUN apk add --no-cache \
    bash \
    curl \
    git \
    go \
    make

COPY . .

RUN make wasm-go wanix

