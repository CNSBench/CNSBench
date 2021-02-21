FROM debian:buster

RUN echo "nameserver 8.8.8.8" > /etc/resolv.conf && cat /etc/resolv.conf && ping -c1 deb.debian.org && apt update && apt install -y jq && \
    apt install -y curl && \
    curl -LO "https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl" && \
    curl -LO "https://github.com/stedolan/jq/releases/download/jq-1.6/jq-linux64" && \
    chmod +x ./jq-linux64 && \
    mv ./jq-linux64 /usr/local/bin/jq && \
    chmod +x ./kubectl && \
    mv ./kubectl /usr/local/bin/kubectl

ENTRYPOINT /bin/bash -c
