#!/bin/sh

./build.sh
docker run -p 6080:80 --link organizations:orgsvc --name json_api --rm json_api:latest
