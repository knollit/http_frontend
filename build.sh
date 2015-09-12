#!/bin/sh

mkdir -p proto
protoc --go_out=proto *.proto
go get
CGO_ENABLED=0 GOOS=linux go build -a --installsuffix cgo --ldflags="-s" -o json_api .
docker build -t json_api:latest .
rm json_api
