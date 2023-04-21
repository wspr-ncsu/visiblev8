PHONY: rust-build go-build fetch-easy-privacy

rust-build:
	mkdir -p ./artifacts
	cd ./adblock/adblock-oracle-rs && cargo build --release
	cp ./adblock/adblock-oracle-rs/target/release/adblock-oracle-rs ./artifacts/adblock

go-build:
	mkdir -p ./artifacts
	go build .
	cp ./vv8-post-processor ./artifacts/vv8-post-processor

fetch-easy-privacy:
	./fetch_easy_privacy.sh

run:
	./artifacts/vv8-post-processor

clean:
	rm -rf ./artifacts
	rm -rf ./adblock/adblock-oracle-rs/target
	rm -rf ./vv8-postprocessor