#!/bin/sh

flatc -g *.fbs
go get
CGO_ENABLED=0 GOOS=linux go build -a --installsuffix cgo --ldflags="-s" -o http_frontend .
docker build -t http_frontend:latest .
rm http_frontend
