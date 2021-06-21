binary:
	go build -ldflags "-X main.Version=`git rev-parse --verify HEAD`"

image:
	docker build -t vv8-post-processor:`git rev-parse --verify --short=0 HEAD` .
	docker tag vv8-post-processor:`git rev-parse --verify --short=0 HEAD` vv8-post-processor:latest

up:
	docker tag vv8-post-processor:`git rev-parse --verify --short=0 HEAD` localhost:5000/vv8-post-processor:`git rev-parse --verify --short=0 HEAD`
	docker push localhost:5000/vv8-post-processor:`git rev-parse --verify --short=0 HEAD`