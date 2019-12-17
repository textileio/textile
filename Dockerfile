FROM golang:1.13.1-stretch
MAINTAINER Andrew Hill <andrew@textile.io>

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
  && go install ./cmd/textiled

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
MAINTAINER Sander Pick <sander@textile.io>

# Get the threads binary, entrypoint script, and TLS CAs from the build container.
ENV SRC_DIR /textile
COPY --from=0 /go/bin/textiled /usr/local/bin/textiled
COPY --from=0 $SRC_DIR/bin/container_daemon /usr/local/bin/start_textile
COPY --from=0 /tmp/su-exec/su-exec /sbin/su-exec
COPY --from=0 /tmp/tini /sbin/tini
COPY --from=0 /etc/ssl/certs /etc/ssl/certs

# Create the fs-repo directory
ENV TEXTILE_REPO /data/textiled
RUN mkdir -p $TEXTILE_REPO \
  && adduser -D -h $TEXTILE_REPO -u 1000 -G users textile \
  && chown -R textile:users $TEXTILE_REPO

# Switch to a non-privileged user
USER textile

# Important this happens after the USER directive so permission are correct.
VOLUME $TEXTILE_REPO

EXPOSE 3006

ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/start_textile"]

CMD ["--addrApi=/ip4/0.0.0.0/tcp/3006"]
