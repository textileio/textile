FROM golang:1.13.5-buster
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

# Install the daemon
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
FROM busybox:1.31.0-glibc
LABEL maintainer="Textile <contact@textile.io>"

# Get the textile binary, entrypoint script, and TLS CAs from the build container.
ENV SRC_DIR /textile
COPY --from=0 /go/bin/textiled /usr/local/bin/textiled
COPY --from=0 /tmp/su-exec/su-exec /sbin/su-exec
COPY --from=0 /tmp/tini /sbin/tini
COPY --from=0 /etc/ssl/certs /etc/ssl/certs

# This shared lib (part of glibc) doesn't seem to be included with busybox.
COPY --from=0 /lib/x86_64-linux-gnu/libdl.so.2 /lib/libdl.so.2

# addrApi; should be exposed to the public
EXPOSE 3006
# addrThreadsHost; should be exposed to the public
EXPOSE 4006
# addrThreadsHostProxy; should be exposed to the public
EXPOSE 5006
# addrThreadsApi; should be exposed to the public
EXPOSE 6006
# addrThreadsApiProxy; should be exposed to the public
EXPOSE 7006
# addrGatewayHost; should be exposed to the public
EXPOSE 8006

# Create the repo directory and switch to a non-privileged user.
ENV TEXTILE_PATH /data/textile
RUN mkdir -p $TEXTILE_PATH \
  && adduser -D -h $TEXTILE_PATH -u 1000 -G users textile \
  && chown -R textile:users $TEXTILE_PATH

# Switch to a non-privileged user
USER textile

# Expose the repo as a volume.
# Important this happens after the USER directive so permission are correct.
VOLUME $TEXTILE_PATH

ENTRYPOINT ["/sbin/tini", "--", "textiled"]

CMD ["--repo=/data/textile"]
