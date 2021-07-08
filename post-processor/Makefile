# Target [private] Docker image repository (for 'up' target)
DOCKER_REPO = localhost:5000

# Base image name (sans version hash)
TAG_ROOT = vv8-post-processor

# Git-commit-hash-derived tag version (from the commit hash for our sub-tree only, not the entire repository)
TAG_HASH = $(shell git rev-parse --verify --short=0 HEAD:post-processor)

# Combined (root+version) image name
IMAGE_TAG = $(TAG_ROOT):$(TAG_HASH)




# default target: build the binary and embed the version hash
binary:
	go build -ldflags "-X main.Version=$(TAG_HASH)"

# cleanup default target: remove build artifacts
clean:
	rm -f vv8-post-processor

# Docker-build target: build container image and tag with the appropriate version hash
image:
	@test -f idldata.json || (echo "cannot build Docker image without 'idldata.json' file from a VV8 build"; exit 1)
	docker build -t $(IMAGE_TAG) .
	docker tag $(IMAGE_TAG) $(TAG_ROOT):latest

# Docker-publish target: upload the current-version container image to the target [private] repository
up:
	docker tag $(IMAGE_TAG) $(DOCKER_REPO)/$(IMAGE_TAG)
	docker push $(DOCKER_REPO)/$(IMAGE_TAG)


