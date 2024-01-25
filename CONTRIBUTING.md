# Contributing

## Development

### Editors

**VSCODE**

- [Go for Visual Studio (Recommended)](https://marketplace.visualstudio.com/items?itemName=golang.go)

- Workspace Settings:

  - Since we are using `syscall/js`, we need to set the `GOOS` and `GOARCH` environment variables to `js` and `wasm` respectively.

  ```json
  {
    "go.toolsEnvVars": {
      "GOOS": "js",
      "GOARCH": "wasm"
    }
  }
  ```

---
