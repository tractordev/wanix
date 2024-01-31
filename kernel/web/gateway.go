package web

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"syscall/js"

	"tractor.dev/wanix/internal/jsutil"
	"tractor.dev/wanix/kernel/web/gwutil"
)

type Gateway struct{}

func (s *Gateway) InitializeJS() {
	// expose to host
	js.Global().Get("sys").Call("handle", "web.request", map[string]any{
		"respondRPC": js.FuncOf(s.request),
	})
}

func (s *Gateway) request(this js.Value, args []js.Value) any {
	var (
		responder = args[0]
		call      = args[1]
	)
	return jsutil.Promise(func() (any, error) {
		params := jsutil.Await(call.Call("receive"))
		var (
			method  = params.Index(0).String()
			fullURL = params.Index(1).String()
		)

		u, _ := url.Parse(fullURL)
		req, err := http.NewRequest(method, u.Path, nil)
		if err != nil {
			fmt.Println("err:", err)
			return nil, err
		}
		rr := httptest.NewRecorder()

		//fmt.Println("SW", fullURL)
		s.ServeHTTP(rr, req)

		headers := make(map[string]any)
		for k, v := range rr.Result().Header {
			headers[k] = v[0]
		}
		headers["Wanix-Status-Code"] = strconv.Itoa(rr.Code)
		headers["Wanix-Status-Text"] = rr.Result().Status

		ch := jsutil.Await(responder.Call("continue", headers))
		chw := &jsutil.Writer{ch}

		if _, err := rr.Body.WriteTo(chw); err != nil {
			fmt.Println("gateway:", err)
		}

		ch.Call("close")
		return nil, nil
	})
}

func (s *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	gwutil.FileTransformer(os.DirFS("."), func(f fs.FS) http.Handler {
		return http.FileServer(http.FS(f))
	}).ServeHTTP(w, r)
}
