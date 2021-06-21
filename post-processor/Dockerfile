# Start with "compilation" base image (the Go toolchain)
FROM golang:1.13 AS gobuild

# Copy in the source directory and compile there
COPY . /build/
WORKDIR /build/
RUN go build

# Switch to target (stretch with dumb-init)
FROM debian:stretch
RUN apt update && apt install dumb-init
RUN useradd -u 1000 -d /app vv8

# Copy in build artifacts
COPY --from=gobuild /build/vv8-post-processor /app/bin/
COPY --from=gobuild /build/idldata.json /app/bin/

# Default ENV for DB/etc
ENV MONGODB_HOST localhost
ENV MONGODB_PORT 27017
ENV MONGODB_DB test

# Initialize entrypoint for safe webhook server launch
WORKDIR /app/bin
USER vv8
EXPOSE 8080
ENTRYPOINT ["/usr/bin/dumb-init", "--", "/app/bin/vv8-post-processor"]
CMD ["-webhook-server", ":8080"]
