FROM golang:1.22 AS build

RUN apt update
RUN apt install -y --no-install-recommends git python3 python3-pip curl make
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | bash -s -- -y
ENV PATH="${PATH}:/root/.cargo/bin"
ENTRYPOINT [ "make", "-C", "post-processor" ]