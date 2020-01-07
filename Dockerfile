FROM golang:1.13.5-stretch
MAINTAINER Textile <contact@textile.io>

# This is (in large part) copied (with love) from
# https://hub.docker.com/r/ipfs/go-ipfs/dockerfile

# Get source
ENV SRC_DIR /textile

# Download packages first so they can be cached.
COPY go.mod go.sum $SRC_DIR/
RUN cd $SRC_DIR \
  && go mod download

COPY . $SRC_DIR

# build source
RUN cd $SRC_DIR \
  && go install github.com/textileio/textile/cmd/textiled

# Get su-exec, a very minimal tool for dropping privileges,
# and tini, a very minimal init daemon for containers
ENV SUEXEC_VERSION v0.2
ENV TINI_VERSION v0.16.1
RUN set -x \
  && cd /tmp \
  && git clone https://github.com/ncopa/su-exec.git \
  && cd su-exec \
  && git checkout -q $SUEXEC_VERSION \
  && make \
  && cd /tmp \
  && wget -q -O tini https://github.com/krallin/tini/releases/download/$TINI_VERSION/tini \
  && chmod +x tini

# Get the TLS CA certificates, they're not provided by busybox.
RUN apt-get update && apt-get install -y ca-certificates

# Now comes the actual target image, which aims to be as small as possible.
FROM busybox:1-glibc
MAINTAINER Textile <contact@textile.io>

# Get the textile binary, entrypoint script, and TLS CAs from the build container.
ENV SRC_DIR /textile
COPY --from=0 /go/bin/textiled /usr/local/bin/textiled
COPY --from=0 $SRC_DIR/bin/container_daemon /usr/local/bin/start_textiled
COPY --from=0 /tmp/su-exec/su-exec /sbin/su-exec
COPY --from=0 /tmp/tini /sbin/tini
COPY --from=0 /etc/ssl/certs /etc/ssl/certs

# This shared lib (part of glibc) doesn't seem to be included with busybox.
# COPY --from=0 /lib/x86_64-linux-gnu/libdl-2.24.so /lib/libdl.so.2

# Create the fs-repo directory
ENV REPO /data/textile
RUN mkdir -p $REPO \
  && adduser -D -h $REPO -u 1000 -G users textile \
  && chown -R textile:users $REPO

# Switch to a non-privileged user
USER textile

# Expose the fs-repo as a volume.
# start_textiled initializes an fs-repo if none is mounted.
# Important this happens after the USER directive so permission are correct.
VOLUME $REPO

EXPOSE 3006
EXPOSE 4006
EXPOSE 5006
EXPOSE 6006
EXPOSE 7006
EXPOSE 8006

# This just makes sure that:
# 1. There's an fs-repo, and initializes one if there isn't.
# 2. The API and Gateway are accessible from outside the container.
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/start_textiled"]

CMD ["textiled", "--repo=/data/textile", "--addrApi=/ip4/0.0.0.0/tcp/3006", "--addrThreadsApi=/ip4/0.0.0.0/tcp/6006", "--addrThreadsApiProxy=/ip4/0.0.0.0/tcp/7006", "--addrGateway=/ip4/0.0.0.0/tcp/8006"]
