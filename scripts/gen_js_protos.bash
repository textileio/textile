#!/bin/bash
set -eo pipefail

wd="$(pwd -P)"

js_paths=()
while IFS=  read -r -d $'\0'; do
  js_paths+=("$REPLY")
done < <(find . -path "*/pb/javascript" ! -path "*/node_modules/*" -print0)

echo installing dependencies
for path in "${js_paths[@]}"; do
  cd "${path}" && npm install >/dev/null 2>&1 && cd "${wd}"
done

echo generating js-protos in api/hubd/pb/javascript
./buildtools/protoc/bin/protoc \
  --proto_path=. \
  --plugin=protoc-gen-ts=api/hubd/pb/javascript/node_modules/.bin/protoc-gen-ts \
  --js_out=import_style=commonjs,binary:api/hubd/pb/javascript \
  --ts_out=service=grpc-web:api/hubd/pb/javascript \
  api/billingd/pb/billingd.proto api/hubd/pb/hubd.proto

echo generating js-protos in api/usersd/pb/javascript
./buildtools/protoc/bin/protoc \
  --proto_path=. \
  --plugin=protoc-gen-ts=api/usersd/pb/javascript/node_modules/.bin/protoc-gen-ts \
  --js_out=import_style=commonjs,binary:api/usersd/pb/javascript \
  --ts_out=service=grpc-web:api/usersd/pb/javascript \
  api/billingd/pb/billingd.proto api/usersd/pb/usersd.proto

echo generating js-protos in api/bucketsd/pb/javascript
./buildtools/protoc/bin/protoc \
  --proto_path=. \
  --plugin=protoc-gen-ts=api/bucketsd/pb/javascript/node_modules/.bin/protoc-gen-ts \
  --js_out=import_style=commonjs,binary:api/bucketsd/pb/javascript \
  --ts_out=service=grpc-web:api/bucketsd/pb/javascript \
  api/bucketsd/pb/bucketsd.proto
