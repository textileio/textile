FROM golang:1.16.0-buster
MAINTAINER Textile <contact@textile.io>

# This is (in large part) copied (with love) from
# https://hub.docker.com/r/ipfs/go-ipfs/dockerfile

# Install deps
RUN apt-get update && apt-get install -y \
  libssl-dev \
  ca-certificates

ENV SRC_DIR /textile

# Download packages first so they can be cached.
COPY go.mod go.sum $SRC_DIR/
RUN cd $SRC_DIR \
  && go mod download

COPY . $SRC_DIR

# Build the thing.
RUN cd $SRC_DIR \
  && TXTL_BUILD_FLAGS="CGO_ENABLED=0 GOOS=linux" make build-buckd

# Get su-exec, a very minimal tool for dropping privileges,
# and tini, a very minimal init daemon for containers
ENV SUEXEC_VERSION v0.2
ENV TINI_VERSION v0.19.0
RUN set -eux; \
    dpkgArch="$(dpkg --print-architecture)"; \
    case "${dpkgArch##*-}" in \
        "amd64" | "armhf" | "arm64") tiniArch="tini-static-$dpkgArch" ;;\
        *) echo >&2 "unsupported architecture: ${dpkgArch}"; exit 1 ;; \
    esac; \
  cd /tmp \
  && git clone https://github.com/ncopa/su-exec.git \
  && cd su-exec \
  && git checkout -q $SUEXEC_VERSION \
  && make su-exec-static \
  && cd /tmp \
  && wget -q -O tini https://github.com/krallin/tini/releases/download/$TINI_VERSION/$tiniArch \
  && chmod +x tini

# Now comes the actual target image, which aims to be as small as possible.
FROM busybox:1.31.1-glibc
LABEL maintainer="Textile <contact@textile.io>"

# Get the textile binary, entrypoint script, and TLS CAs from the build container.
ENV SRC_DIR /textile
COPY --from=0 $SRC_DIR/buckd /usr/local/bin/buckd
COPY --from=0 /tmp/su-exec/su-exec-static /sbin/su-exec
COPY --from=0 /tmp/tini /sbin/tini
COPY --from=0 /etc/ssl/certs /etc/ssl/certs

# This shared lib (part of glibc) doesn't seem to be included with busybox.
COPY --from=0 /lib/*-linux-gnu*/libdl.so.2 /lib/

# Copy over SSL libraries.
COPY --from=0 /usr/lib/*-linux-gnu*/libssl.so* /usr/lib/
COPY --from=0 /usr/lib/*-linux-gnu*/libcrypto.so* /usr/lib/

# addrApi; can be exposed to the public.
EXPOSE 3006
# addrApiProxy; can be exposed to the public.
EXPOSE 3007
# addrThreadsHost; must be exposed to the public.
EXPOSE 4006
# addrGatewayHost; can be exposed to the public.
EXPOSE 8006

# Create the repo directory.
ENV BUCKETS_PATH /data/buckets
RUN mkdir -p $BUCKETS_PATH \
  && adduser -D -h $BUCKETS_PATH -u 1000 -G users buckets \
  && chown buckets:users $BUCKETS_PATH

# Switch to a non-privileged user.
USER buckets

# Expose the repo as a volume.
# Important this happens after the USER directive so permission are correct.
VOLUME $BUCKETS_PATH

ENTRYPOINT ["/sbin/tini", "--", "buckd"]

CMD ["--repo=/data/buckets"]
