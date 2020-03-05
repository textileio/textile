PB = $(wildcard *.proto)
GO = $(PB:.proto=.pb.go)
PROTOC_GEN_TS_PATH = "./javascript/node_modules/.bin/protoc-gen-ts"

all: $(GO)

%.pb.go: %.proto
	protoc -I=. -I=$(GOPATH)/src \
	--plugin="protoc-gen-ts=${PROTOC_GEN_TS_PATH}" --ts_out="service=grpc-web:javascript/src/." \
	--go_out=\
	plugins=grpc:\
	. $<

clean:
	rm -f *.pb.go
	rm -fr ./javascript/dist

.PHONY: clean
