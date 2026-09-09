//go:build js && wasm

// The one syscall/js file outside the engine: a WebSocket whose callbacks do
// nothing but hand bytes to Go, and the two facts about the page the client
// needs — its query string, and the socket's URL on the page's own origin.
//
// There is no goroutine and no channel here. The callbacks run on the frame
// loop's goroutine, the same way the engine's key events do, so what they
// append is read by the next frame without a lock.

package main

import "syscall/js"

// socket is one WebSocket to the server.
type socket struct {
	ws    js.Value
	funcs []js.Func

	// open is true between onopen and onclose; closed is true after onclose,
	// with the code and reason the browser reported.
	open   bool
	closed bool
	code   int
	reason string
}

// dialSocket opens a binary WebSocket to url. onOpen runs once the socket is
// ready to send; onMessage runs with each message's bytes, in order.
func dialSocket(url string, onOpen func(), onMessage func([]byte)) *socket {
	s := &socket{}
	s.ws = js.Global().Get("WebSocket").New(url)
	s.ws.Set("binaryType", "arraybuffer")
	s.ws.Set("onopen", s.keep(func(js.Value, []js.Value) any {
		s.open = true
		onOpen()
		return nil
	}))
	s.ws.Set("onmessage", s.keep(func(_ js.Value, args []js.Value) any {
		// The ArrayBuffer is copied out once, into a slice Go owns, and the
		// browser's buffer is never touched again.
		u8 := js.Global().Get("Uint8Array").New(args[0].Get("data"))
		buf := make([]byte, u8.Get("byteLength").Int())
		js.CopyBytesToGo(buf, u8)
		onMessage(buf)
		return nil
	}))
	s.ws.Set("onclose", s.keep(func(_ js.Value, args []js.Value) any {
		s.open, s.closed = false, true
		if len(args) > 0 && args[0].Truthy() {
			s.code = args[0].Get("code").Int()
			s.reason = args[0].Get("reason").String()
		}
		return nil
	}))
	return s
}

// send writes one binary message, and reports whether the socket was open to
// take it.
func (s *socket) send(b []byte) bool {
	if !s.open {
		return false
	}
	u8 := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(u8, b)
	s.ws.Call("send", u8)
	return true
}

// keep registers a callback and holds on to it, so the garbage collector does
// not free it while the browser still holds a reference.
func (s *socket) keep(fn func(js.Value, []js.Value) any) js.Func {
	f := js.FuncOf(fn)
	s.funcs = append(s.funcs, f)
	return f
}

// now is the page's own monotonic clock, in milliseconds.
func now() float64 { return js.Global().Get("performance").Call("now").Float() }

// pageQuery is the URL's query string, "?solo" and all.
func pageQuery() string { return js.Global().Get("location").Get("search").String() }

// socketURL is /ws on the page's own origin, over wss when the page is https.
// The same origin is what the content security policy's default-src 'self'
// allows and what the server's Origin check expects.
func socketURL() string {
	loc := js.Global().Get("location")
	scheme := "ws:"
	if loc.Get("protocol").String() == "https:" {
		scheme = "wss:"
	}
	return scheme + "//" + loc.Get("host").String() + "/ws"
}
