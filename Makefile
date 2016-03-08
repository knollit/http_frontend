all: build

build: flatbuffers
	CGO_ENABLED=0 GOOS=linux go build -a --installsuffix cgo --ldflags="-s" -o dest/http_frontend .
	docker build -t knollit/http_frontend:latest .

flatbuffers:
	flatc -g -o $${GOPATH##*:}/src/github.com/knollit/http_frontend *.fbs

clean:
	rm -rf dest

publish: build
	docker tag knollit/http_frontend:latest knollit/http_frontend:$$CIRCLE_SHA1
	docker push knollit/http_frontend:$$CIRCLE_SHA1
	docker push knollit/http_frontend:latest
