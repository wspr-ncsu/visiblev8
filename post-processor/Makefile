PHONY: rust-build go-build fetch-easy-privacy build_entity_hashmap

docker:
	./build/docker.sh

rust-build:
	mkdir -p ./artifacts
	cd ./adblock/adblock-oracle-rs && cargo build --release
	cp ./adblock/adblock-oracle-rs/target/release/adblock-oracle-rs ./artifacts/adblock

go-build:
	mkdir -p ./artifacts
	go build .
	cp ./vv8-post-processor ./artifacts/vv8-post-processor

publish:
	./build/publish.sh

fetch-easy-privacy:
	./build/fetch_easy_privacy.sh

build_entity_hashmap:
	./build/entity_hashmap.sh

clean:
	rm -rf ./artifacts
	rm -rf ./adblock/adblock-oracle-rs/target
	rm -rf ./vv8-postprocessor