#!/bin/sh

./build.sh
docker run -p 6080:80 --link organizations:orgsvc -e TLS_CA_PATH=/ca.crt -e TLS_CERT_PATH=/apiproj-client4.crt -e TLS_KEY_PATH=/apiproj-client4.key --name json_api --rm json_api:latest
