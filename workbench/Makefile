VSCODE_DOWNLOAD	?= https://github.com/progrium/vscode-web/releases/download/v1/vscode-web-1.108.2.zip

build: dist code
.PHONY: build 

clean:
	rm -rf code
	rm -rf dist
	rm -rf node_modules
.PHONY: clean

code:
	curl -sLo code.zip $(VSCODE_DOWNLOAD)
	unzip code.zip
	mv dist/vscode code
	rm dist/index.html
	rm code.zip

dist: node_modules
	go run ./build.go

node_modules: package.json
	npm install