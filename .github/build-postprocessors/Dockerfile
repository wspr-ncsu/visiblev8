FROM golang:1.23 AS build

COPY post-processor /post-processor

RUN apt update
RUN apt install -y --no-install-recommends git python3 python3-pip curl make
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | bash -s -- -y
ENV PATH="${PATH}:/root/.cargo/bin"
WORKDIR /
RUN make -C post-processor

FROM ubuntu:latest
COPY --from=build /post-processor/artifacts /artifacts
