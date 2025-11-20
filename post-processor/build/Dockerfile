FROM golang:1.24 AS builder

WORKDIR /visiblev8

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        git python3 python3-pip curl && \
    rm -rf /var/lib/apt/lists/*

RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs \
    | bash -s -- -y
ENV PATH="${PATH}:/root/.cargo/bin"

RUN --mount=type=cache,target=/root/.cargo/registry \
    --mount=type=cache,target=/go/pkg \
    true

COPY . .

RUN make -C /visiblev8

FROM ubuntu:latest AS runtime

WORKDIR /

COPY --from=builder /visiblev8/artifacts /artifacts
