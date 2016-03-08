all: build

build: flatbuffers
	CGO_ENABLED=0 GOOS=linux go build -a --installsuffix cgo --ldflags="-s" -o dest/$$PROJECT_NAME .
	docker build -t knollit/$$PROJECT_NAME:latest .

flatbuffers:
	flatc -g -o $${GOPATH##*:}/src/github.com/knollit/$$PROJECT_NAME *.fbs

clean:
	rm -rf dest

publish: build
	docker tag knollit/$$CIRCLE_PROJECT_REPONAME:latest knollit/$$CIRCLE_PROJECT_REPONAME:$$CIRCLE_SHA1
	docker push knollit/$$CIRCLE_PROJECT_REPONAME:$$CIRCLE_SHA1
	docker push knollit/$$CIRCLE_PROJECT_REPONAME:latest
