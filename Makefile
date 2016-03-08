repo = knollit/http_frontend

all: build

build: flatbuffers
	CGO_ENABLED=0 GOOS=linux go build -a --installsuffix cgo --ldflags="-s" -o dest/http_frontend .
	docker build -t $(repo):latest .

flatbuffers:
	flatc -g -o $${GOPATH##*:}/src/github.com/$(repo) *.fbs

clean:
	rm -rf dest

publish: build
	docker tag $(repo):latest $(repo):$$CIRCLE_SHA1
	docker push $(repo):$$CIRCLE_SHA1
	docker push $(repo):latest
