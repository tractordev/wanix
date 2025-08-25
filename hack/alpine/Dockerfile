FROM i386/alpine:latest
RUN apk add -u e2fsprogs podman runc buildah netavark umoci
COPY ./bin/init /bin/init
COPY ./bin/post-dhcp /bin/post-dhcp
COPY ./etc/containers.conf /etc/containers/containers.conf
