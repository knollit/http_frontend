#!/bin/sh

if [ "$CIRCLECI" = true ]
then
  flatc -g -o ~/.go_workspace/src/github.com/knollit/$CIRCLE_PROJECT_REPONAME/ *.fbs
else
  flatc -g *.fbs
fi
go get
CGO_ENABLED=0 GOOS=linux go build -a --installsuffix cgo --ldflags="-s" -o http_frontend .
docker build -t http_frontend:latest .
rm http_frontend
