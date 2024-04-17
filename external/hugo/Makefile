.PHONY: build update clean

build: update
	cd ./repo && GOOS=js GOARCH=wasm go build -tags nodeploy -o ./../../../boot/initfs/cmd/hugo.wasm

repo:
	git clone -q https://github.com/gohugoio/hugo.git ./repo
	cd ./repo && git checkout -q v0.123.6

update: wanix_vendor.patch repo
	cd ./repo && git reset --hard HEAD
	cd ./repo && git apply ../wanix_vendor.patch

clean:
	rm -rf ./repo
