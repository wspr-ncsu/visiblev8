.PHONY: build patch-debug clean

DEFAULT ?= 1
DEBUG ?= 0
PUBLISH_ASSETS ?= 0
ANDROID ?= 0
ARM ?= 0
WEBVIEW ?= 0
IDLDATA ?= 1

build:
	# to build a specific version, run:
	# make build VERSION=104.0.5112.79 
	./run.sh "$(VERSION)" "$(DEFAULT)" "$(DEBUG)" "$(PUBLISH_ASSETS)" "$(TESTS)" "$(ANDROID)" "$(ARM)" "$(WEBVIEW)" "$(IDLDATA)"

debug:
	@$(MAKE) build DEFAULT=1 DEBUG=1

android:
	@$(MAKE) build ANDROID=1 DEFAULT=0

arm:
	@$(MAKE) build ARM=1 DEFAULT=0

webview:
	@$(MAKE) build WEBVIEW=1 DEFAULT=0

patch-debug:
	docker commit $$(docker ps -q -l) patch-fail
	docker run -it -v $$(pwd)/artifacts:/artifacts -v $$(pwd)/build:/build -v $$(dirname `pwd`):/build/visiblev8 --entrypoint=/bin/bash patch-fail

clean:
	# cleanup any old builds (like build/103.0.5060.134)
	sudo find build/ -type d -name '[0-9]*\.[0-9]*\.[0-9]*\.[0-9]*' | sudo xargs rm -rf