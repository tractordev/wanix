
alpine.tgz:
	go run ./hubpull/main.go i386/alpine latest 386
	go run ./fixlinks/main.go -input i386/alpine-latest-386/a55f2fb89da4caae0d783c0045a67446dee9bbd977fecb44db9e1231550fa888.tar.gz -output alpine.tar
	rm -rf i386
	tar -rf alpine.tar bin/
	gzip -c alpine.tar > alpine.tgz
	rm alpine.tar