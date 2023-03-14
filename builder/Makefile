all:
	./run.sh

patch-debug:
	docker commit $(docker ps -q -l) patch-fail
	docker run -it -v $(pwd)/artifacts:/artifacts -v $(pwd)/build:/build -v $VV8_DIR:/build/visiblev8 --entrypoint=/bin/bash build-fail

clean:
	# cleanup any old builds (like build/103.0.5060.134)
	sudo find build/ -type d -name '[0-9]*\.[0-9]*\.[0-9]*\.[0-9]*' | sudo xargs rm -rf