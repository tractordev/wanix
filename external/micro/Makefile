build: update
	cd ./repo && make wasm

repo: 
	git clone -q https://github.com/zyedidia/micro ./repo
	cd ./repo && git checkout -q d8e9d61a955edf6f58c1533012d8a25728ba65be

update: wanix_vendor.patch repo
	cd ./repo && git reset --hard HEAD
	cd ./repo && go mod vendor && git apply ../wanix_vendor.patch

clean:
	rm -rf ./repo
