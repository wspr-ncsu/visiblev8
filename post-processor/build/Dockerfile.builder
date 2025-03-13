FROM golang:1.23 AS build

WORKDIR /visiblev8
RUN apt update
RUN apt install -y --no-install-recommends git python3 python3-pip curl
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | bash -s -- -y
ENV PATH="${PATH}:/root/.cargo/bin"
