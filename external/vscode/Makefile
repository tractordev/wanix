.PHONY: build

VSCODE_ARTIFACT_URL="https://github.com/progrium/vscode-web/releases/download/v1/vscode-web-1.92.1-patched.zip"

build: assets/vscode
	GOOS=wasip1 GOARCH=wasm go build -o vscode.wasm .

assets/vscode:
	mkdir -p assets
	curl -L $(VSCODE_ARTIFACT_URL) -o vscode.zip
	unzip vscode.zip
	mv dist/vscode assets/vscode
	rm -rf dist vscode.zip
