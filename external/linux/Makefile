.PHONY: build serve qemu

build:
	docker build -t l4w-i386-builder .
	docker run --rm -v "$(PWD):/output-mount" l4w-i386-builder
	@ls -lh ./bzImage ./initramfs.gz

qemu:
	qemu-system-i386 \
		-kernel ./bzImage \
		-initrd ./initramfs.gz \
		-netdev user,id=net0 \
		-device virtio-net-pci,netdev=net0