#!/bin/sh

./build.sh
docker run -p 6080:80 --link organizations:orgsvc -e TLS_CERT_PATH=/test-client.crt -e TLS_KEY_PATH=/test-client.key --name json_api --rm json_api:latest
