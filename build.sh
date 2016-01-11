#!/bin/sh

if [ "$CIRCLECI" = true ]
then
  export PROJNAME=$CIRCLE_PROJECT_REPONAME
  flatc -g -o ~/.go_workspace/src/github.com/knollit/$PROJNAME/ *.fbs
else
  flatc -g *.fbs
fi
go get
CGO_ENABLED=0 GOOS=linux go build -a --installsuffix cgo --ldflags="-s" -o $PROJNAME .
docker build -t knollit/$PROJNAME:latest .
rm $PROJNAME
