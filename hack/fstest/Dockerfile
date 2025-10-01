FROM debian:bookworm-slim
RUN apt update && apt install -y bats bats-assert attr sudo
RUN useradd -m -s /bin/bash testuser
RUN usermod -aG sudo testuser
RUN echo 'testuser ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers
RUN chown -R testuser:testuser /mnt
# USER testuser
WORKDIR /test
COPY fstest.bats .
COPY fstest /bin/fstest
ENTRYPOINT ["/bin/fstest"]