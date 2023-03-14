.PHONY: build patch-debug clean

build:
	# to build a specific version, run:
	# make build VERSION=104.0.5112.79
	./run.sh $(VERSION)

patch-debug:
	docker commit $$(docker ps -q -l) patch-fail
	docker run -it -v $$(pwd)/artifacts:/artifacts -v $$(pwd)/build:/build -v $$(dirname `pwd`):/build/visiblev8 --entrypoint=/bin/bash patch-fail

clean:
	# cleanup any old builds (like build/103.0.5060.134)
	sudo find build/ -type d -name '[0-9]*\.[0-9]*\.[0-9]*\.[0-9]*' | sudo xargs rm -rf