#!/usr/bin/env sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
PATH="$(go env GOPATH)/bin:$PATH"

rm -rf "$ROOT/gen/go"
mkdir -p "$ROOT/gen/go"

protoc -I "$ROOT/proto" -I "$ROOT/../contracts/sms/proto" \
  --go_out="$ROOT/gen/go" \
  --go_opt=paths=source_relative \
  --go-grpc_out="$ROOT/gen/go" \
  --go-grpc_opt=paths=source_relative \
  "$ROOT/proto/byte/v/forge/sms/internal/v1/sms_internal.proto" \
  "$ROOT/proto/byte/v/forge/sms/providers/fivesim/v1/fivesim.proto" \
  "$ROOT/proto/byte/v/forge/sms/providers/herosms/v1/herosms.proto" \
  "$ROOT/proto/byte/v/forge/sms/providers/smsbower/v1/smsbower.proto"
