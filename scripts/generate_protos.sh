#!/usr/bin/env bash

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROTO_DIR=$DIR/../src/cred-alert/revokpb

if ! hash protoc 2>/dev/null; then
  echo "protoc missing, cannot continue"
  echo "download protoc from https://github.com/google/protobuf/releases"
  exit 1
fi

protoc \
  --proto_path=$PROTO_DIR \
  --go_out=plugins=grpc:$PROTO_DIR \
  $PROTO_DIR/*.proto
