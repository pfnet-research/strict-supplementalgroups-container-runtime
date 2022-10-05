FROM ubuntu:22.04
RUN groupadd -g 50000 bypassed-group \
    && useradd -m -u 1000 alice \
    && gpasswd -a alice bypassed-group
