DOCKER_CMD		?= $(shell command -v podman || command -v docker)

bundle.tgz: bundle
	tar -C bundle -czf bundle.tgz .

bundle:
	$(DOCKER_CMD) build -t wanix-shell-build --platform linux/386 -f Dockerfile .
	$(DOCKER_CMD) rm wanix-shell-export
	$(DOCKER_CMD) create --name wanix-shell-export --platform linux/386 wanix-shell-build
	mkdir -p bundle
	$(DOCKER_CMD) export wanix-shell-export | tar -C bundle -xf -
	

clean:
	rm -f bundle.tgz
	rm -rf bundle
.PHONY: clean
