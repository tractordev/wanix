# wanix shell

This is a small (<5MB) Wanix bundle that uses v86, BusyBox, and a custom kernel 
to give you a minimal Wanix shell. It does not have a package manager or 
networking (see hack/alpine for that). 

## Build

With Podman/Docker installed you can run:

```sh
make
```

This will produce `bundle.tgz`.