#!/bin/sh
set -eu

protoc \
  --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  proto/santaizi.proto

cp proto/santaizi.proto ../santaizi-agent/proto/santaizi.proto
cp proto/santaizi.pb.go ../santaizi-agent/proto/santaizi.pb.go
cp proto/santaizi_grpc.pb.go ../santaizi-agent/proto/santaizi_grpc.pb.go

cmp proto/santaizi.proto ../santaizi-agent/proto/santaizi.proto
cmp proto/santaizi.pb.go ../santaizi-agent/proto/santaizi.pb.go
cmp proto/santaizi_grpc.pb.go ../santaizi-agent/proto/santaizi_grpc.pb.go
