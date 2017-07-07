#!/usr/bin/env bash

set -e -u

if ! hash protoc 2>/dev/null; then
  echo "protoc missing, cannot continue"
  echo "download protoc from https://github.com/google/protobuf/releases"
  exit 1
fi

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
SRC=$DIR/../src

# red
RED_PROTO_DIR=$SRC/red/redpb
protoc \
  --proto_path=$RED_PROTO_DIR \
  --go_out=plugins=grpc:$SRC \
  $RED_PROTO_DIR/*.proto

# revok
REVOK_PROTO_DIR=$SRC/cred-alert/revokpb
protoc \
  --proto_path=$RED_PROTO_DIR \
  --proto_path=$REVOK_PROTO_DIR \
  --go_out=plugins=grpc:$REVOK_PROTO_DIR \
  $REVOK_PROTO_DIR/*.proto

# rolodex
ROLODEX_PROTO_DIR=$SRC/rolodex/rolodexpb
protoc \
  --proto_path=$RED_PROTO_DIR \
  --proto_path=$ROLODEX_PROTO_DIR \
  --go_out=plugins=grpc:$ROLODEX_PROTO_DIR \
  $ROLODEX_PROTO_DIR/*.proto
