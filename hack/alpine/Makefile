
alpine.tgz:
	go run ./hubpull/main.go i386/alpine latest 386
	go run ./fixlinks/main.go -input i386/alpine-latest-386/69aa61ccf55e5bf8e7a069b89e8afb42b4f3443b3785868795af8046d810d608.tar.gz -output alpine.tar
	rm -rf i386
	tar -rf alpine.tar bin/
	gzip -c alpine.tar > alpine.tgz
	rm alpine.tar