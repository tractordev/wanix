FROM debian:bookworm-slim
RUN apt update && apt install -y bats bats-assert attr sudo tree
RUN useradd -m -s /bin/bash testuser
RUN usermod -aG sudo testuser
RUN echo 'testuser ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers
RUN chown -R testuser:testuser /mnt
# USER testuser
COPY fstest /bin/fstest
COPY init /bin/init
COPY testfs /bin/testfs
COPY junk /bin/junk
ENTRYPOINT ["/bin/init"]