package ws_test

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andygeiss/wisp-engine/internal/ws"
)

// The RFC's own handshake example, section 1.3: this key gets this answer.
const (
	rfcKey    = "dGhlIHNhbXBsZSBub25jZQ=="
	rfcAccept = "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
)

// goodHeaders is a handshake the server accepts, minus the request line and
// Host, which the helpers write.
const goodHeaders = "Upgrade: websocket\r\n" +
	"Connection: keep-alive, Upgrade\r\n" +
	"Sec-WebSocket-Key: " + rfcKey + "\r\n" +
	"Sec-WebSocket-Version: 13\r\n" +
	"Origin: http://lab.test\r\n"

// pipeListener hands one end of a net.Pipe to an http.Server for every dial,
// so a handshake runs through the real server and its hijack without a port.
type pipeListener struct {
	conns chan net.Conn
	done  chan struct{}
	once  sync.Once
}

func (l *pipeListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *pipeListener) Close() error   { l.once.Do(func() { close(l.done) }); return nil }
func (l *pipeListener) Addr() net.Addr { return pipeAddr{} }

func (l *pipeListener) dial(t *testing.T) net.Conn {
	t.Helper()
	client, server := net.Pipe()
	select {
	case l.conns <- server:
	case <-l.done:
		t.Fatal("the server is gone")
	}
	return client
}

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }

// serve runs h on a pipe listener until the test ends.
func serve(t *testing.T, h http.Handler) *pipeListener {
	t.Helper()
	ln := &pipeListener{conns: make(chan net.Conn), done: make(chan struct{})}
	srv := &http.Server{Handler: h, ReadHeaderTimeout: time.Second}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
	return ln
}

// accepting is a handler that upgrades and hands the connection out.
func accepting(t *testing.T) (http.Handler, <-chan *ws.Conn) {
	t.Helper()
	accepted := make(chan *ws.Conn, 1)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := ws.Accept(w, r)
		if err != nil {
			return
		}
		c.ReadTimeout, c.WriteTimeout = 5*time.Second, 5*time.Second
		accepted <- c
	}), accepted
}

// pair dials through a real handshake and returns both ends, each with
// deadlines so a bug is a failure rather than a hang.
func pair(t *testing.T) (client, server *ws.Conn) {
	t.Helper()
	h, accepted := accepting(t)
	ln := serve(t, h)
	client, err := ws.Dial(ln.dial(t), "lab.test", "/ws")
	if err != nil {
		t.Fatal(err)
	}
	client.ReadTimeout, client.WriteTimeout = 5*time.Second, 5*time.Second
	server = <-accepted
	t.Cleanup(func() { client.Close(ws.CloseNormal, ""); server.Close(ws.CloseNormal, "") })
	return client, server
}

// raw performs the handshake by hand and returns the bare client side, so a
// test can write frames the real client never would. The reader is the one
// the response was read through, in case it holds bytes past it.
func raw(t *testing.T) (nc net.Conn, br *bufio.Reader, server *ws.Conn) {
	t.Helper()
	h, accepted := accepting(t)
	ln := serve(t, h)
	nc = ln.dial(t)
	nc.SetDeadline(time.Now().Add(5 * time.Second))
	res, br := handshake(t, nc, goodHeaders)
	if res.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("handshake answered %s", res.Status)
	}
	server = <-accepted
	t.Cleanup(func() { nc.Close(); server.Close(ws.CloseNormal, "") })
	return nc, br, server
}

// handshake writes a request with the given headers and reads the answer.
func handshake(t *testing.T, nc net.Conn, headers string) (*http.Response, *bufio.Reader) {
	t.Helper()
	go io.WriteString(nc, "GET /ws HTTP/1.1\r\nHost: lab.test\r\n"+headers+"\r\n")
	br := bufio.NewReader(nc)
	res, err := http.ReadResponse(br, &http.Request{Method: http.MethodGet})
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res, br
}

// frame builds one frame the way a client or a server would.
func frame(fin bool, op byte, payload []byte, mask bool) []byte {
	b := []byte{op}
	if fin {
		b[0] |= 0x80
	}
	m := byte(0)
	if mask {
		m = 0x80
	}
	switch n := len(payload); {
	case n <= 125:
		b = append(b, m|byte(n))
	case n < 1<<16:
		b = append(b, m|126, byte(n>>8), byte(n))
	default:
		b = append(b, m|127)
		b = binary.BigEndian.AppendUint64(b, uint64(n))
	}
	if !mask {
		return append(b, payload...)
	}
	key := [4]byte{1, 2, 3, 4}
	b = append(b, key[:]...)
	for i, p := range payload {
		b = append(b, p^key[i&3])
	}
	return b
}

// readFrame reads one unmasked frame the way a client reads a server's.
func readFrame(t *testing.T, br *bufio.Reader) (op byte, payload []byte) {
	t.Helper()
	var hdr [2]byte
	if _, err := io.ReadFull(br, hdr[:]); err != nil {
		t.Fatalf("reading a frame header: %v", err)
	}
	if hdr[1]&0x80 != 0 {
		t.Fatal("the server masked a frame")
	}
	n := int(hdr[1] & 0x7F)
	switch n {
	case 126:
		var ext [2]byte
		io.ReadFull(br, ext[:])
		n = int(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		io.ReadFull(br, ext[:])
		n = int(binary.BigEndian.Uint64(ext[:]))
	}
	payload = make([]byte, n)
	if _, err := io.ReadFull(br, payload); err != nil {
		t.Fatalf("reading a frame payload: %v", err)
	}
	return hdr[0] & 0x0F, payload
}

// closeCode reads a close frame and returns its code.
func closeCode(t *testing.T, br *bufio.Reader) uint16 {
	t.Helper()
	op, payload := readFrame(t, br)
	if op != 0x8 {
		t.Fatalf("opcode %#x, want a close frame", op)
	}
	if len(payload) < 2 {
		t.Fatalf("a close frame with %d bytes of payload", len(payload))
	}
	return binary.BigEndian.Uint16(payload)
}

// read runs Read on its own goroutine, because a pipe hands bytes over only
// while somebody is reading them.
func read(c *ws.Conn) <-chan result {
	ch := make(chan result, 1)
	go func() {
		msg, err := c.Read()
		ch <- result{msg, err}
	}()
	return ch
}

type result struct {
	msg []byte
	err error
}

// wantClose asserts a Read ended with the given close code.
func wantClose(t *testing.T, r result, code uint16) {
	t.Helper()
	ce, ok := errors.AsType[*ws.CloseError](r.err)
	if !ok {
		t.Fatalf("Read = %q, %v; want a *CloseError", r.msg, r.err)
	}
	if ce.Code != code {
		t.Errorf("closed with %d (%s), want %d", ce.Code, ce.Reason, code)
	}
}

func TestHandshakeAnswersTheRFCsVector(t *testing.T) {
	t.Parallel()
	h, accepted := accepting(t)
	ln := serve(t, h)
	nc := ln.dial(t)
	defer nc.Close()
	res, _ := handshake(t, nc, goodHeaders)

	if res.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("status = %s, want 101", res.Status)
	}
	if got := res.Header.Get("Sec-WebSocket-Accept"); got != rfcAccept {
		t.Errorf("Sec-WebSocket-Accept = %q, want %q", got, rfcAccept)
	}
	if !strings.EqualFold(res.Header.Get("Upgrade"), "websocket") || !strings.EqualFold(res.Header.Get("Connection"), "Upgrade") {
		t.Errorf("the answer is missing Upgrade or Connection: %v", res.Header)
	}
	(<-accepted).Close(ws.CloseNormal, "")
}

func TestAcceptRefuses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		headers string
		status  int
	}{
		{"a plain GET", "", http.StatusBadRequest},
		{"a key that is not 16 bytes", strings.Replace(goodHeaders, rfcKey, "c2hvcnQ=", 1), http.StatusBadRequest},
		{"a version that is not 13", strings.Replace(goodHeaders, "Version: 13", "Version: 8", 1), http.StatusUpgradeRequired},
		{"an origin that is another host", strings.Replace(goodHeaders, "http://lab.test", "http://evil.test", 1), http.StatusForbidden},
		{"no origin at all", strings.Replace(goodHeaders, "Origin: http://lab.test\r\n", "", 1), http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h, _ := accepting(t)
			ln := serve(t, h)
			nc := ln.dial(t)
			defer nc.Close()
			res, _ := handshake(t, nc, tt.headers)
			if res.StatusCode != tt.status {
				t.Errorf("status = %d, want %d", res.StatusCode, tt.status)
			}
			if tt.status == http.StatusUpgradeRequired && res.Header.Get("Sec-WebSocket-Version") != "13" {
				t.Error("a 426 has to say which version it does speak")
			}
		})
	}
}

// Every size that changes the header's shape, in both directions.
func TestMessagesOfEverySizeRoundTrip(t *testing.T) {
	t.Parallel()
	client, server := pair(t)
	for _, n := range []int{0, 1, 125, 126, 127, 65535, 65536, 70000} {
		msg := make([]byte, n)
		for i := range msg {
			msg[i] = byte(i)
		}
		for _, dir := range []struct {
			name     string
			from, to *ws.Conn
		}{{"client to server", client, server}, {"server to client", server, client}} {
			got := read(dir.to)
			if err := dir.from.Write(msg); err != nil {
				t.Fatalf("%d bytes %s: Write: %v", n, dir.name, err)
			}
			r := <-got
			if r.err != nil || string(r.msg) != string(msg) {
				t.Fatalf("%d bytes %s: Read = %d bytes, %v", n, dir.name, len(r.msg), r.err)
			}
		}
	}
}

func TestAnUnmaskedClientFrameIsRefused(t *testing.T) {
	t.Parallel()
	nc, br, server := raw(t)
	got := read(server)
	nc.Write(frame(true, 0x2, []byte("x"), false))

	if code := closeCode(t, br); code != ws.CloseProtocolError {
		t.Errorf("the server closed with %d, want 1002", code)
	}
	wantClose(t, <-got, ws.CloseProtocolError)
}

func TestAFragmentedMessageIsReassembled(t *testing.T) {
	t.Parallel()
	nc, br, server := raw(t)
	got := read(server)
	nc.Write(frame(false, 0x2, []byte("hel"), true))
	// A control frame may land between two fragments, and is answered there.
	nc.Write(frame(true, 0x9, []byte("ping"), true))
	if op, p := readFrame(t, br); op != 0xA || string(p) != "ping" {
		t.Errorf("the ping between fragments got opcode %#x %q, want a pong", op, p)
	}
	nc.Write(frame(false, 0x0, []byte("l"), true))
	nc.Write(frame(true, 0x0, []byte("o"), true))

	r := <-got
	if r.err != nil || string(r.msg) != "hello" {
		t.Errorf("Read = %q, %v; want hello", r.msg, r.err)
	}
}

func TestTheCloseHandshake(t *testing.T) {
	t.Parallel()
	t.Run("the server closes", func(t *testing.T) {
		t.Parallel()
		client, server := pair(t)
		got := read(client)
		if err := server.Close(ws.CloseGoingAway, "shutting down"); err != nil {
			t.Fatal(err)
		}
		r := <-got
		wantClose(t, r, ws.CloseGoingAway)
		if ce, _ := errors.AsType[*ws.CloseError](r.err); ce != nil && ce.Reason != "shutting down" {
			t.Errorf("reason = %q", ce.Reason)
		}
		if err := server.Write([]byte("late")); !errors.Is(err, ws.ErrClosed) {
			t.Errorf("Write after Close = %v, want ErrClosed", err)
		}
	})
	t.Run("the client closes", func(t *testing.T) {
		t.Parallel()
		client, server := pair(t)
		got := read(server)
		client.Close(ws.CloseNormal, "")
		wantClose(t, <-got, ws.CloseNormal)
		if _, err := server.Read(); !errors.Is(err, ws.ErrClosed) {
			t.Errorf("Read after the peer closed = %v, want ErrClosed", err)
		}
	})
	t.Run("a raw close is echoed", func(t *testing.T) {
		t.Parallel()
		nc, br, server := raw(t)
		got := read(server)
		nc.Write(frame(true, 0x8, []byte{0x0F, 0xA0, 'b', 'y', 'e'}, true)) // 4000
		if code := closeCode(t, br); code != 4000 {
			t.Errorf("the echo carried %d, want the peer's own 4000", code)
		}
		wantClose(t, <-got, 4000)
	})
}

func TestAFrameThatLiesAboutItsLength(t *testing.T) {
	t.Parallel()
	t.Run("more than the cap is refused before it is read", func(t *testing.T) {
		t.Parallel()
		nc, br, server := raw(t)
		server.MaxMessage = 1024
		got := read(server)
		// The header of a 2000-byte frame, and not one byte of the 2000.
		nc.Write([]byte{0x82, 0xFE, 0x07, 0xD0, 1, 2, 3, 4})
		if code := closeCode(t, br); code != ws.CloseTooLarge {
			t.Errorf("the server closed with %d, want 1009", code)
		}
		wantClose(t, <-got, ws.CloseTooLarge)
	})
	t.Run("fragments are counted together", func(t *testing.T) {
		t.Parallel()
		nc, br, server := raw(t)
		server.MaxMessage = 8
		got := read(server)
		nc.Write(frame(false, 0x2, []byte("12345"), true))
		nc.Write(frame(true, 0x0, []byte("6789"), true))
		if code := closeCode(t, br); code != ws.CloseTooLarge {
			t.Errorf("the server closed with %d, want 1009", code)
		}
		wantClose(t, <-got, ws.CloseTooLarge)
	})
	t.Run("fewer bytes than promised is an error, not a panic", func(t *testing.T) {
		t.Parallel()
		nc, _, server := raw(t)
		got := read(server)
		nc.Write([]byte{0x82, 0x8A, 1, 2, 3, 4, 'a', 'b', 'c'}) // says 10, sends 3
		nc.Close()
		if r := <-got; !errors.Is(r.err, io.ErrUnexpectedEOF) {
			t.Errorf("Read = %q, %v; want ErrUnexpectedEOF", r.msg, r.err)
		}
	})
}

func TestAPingIsAnsweredWithAPong(t *testing.T) {
	t.Parallel()
	nc, br, server := raw(t)
	got := read(server)
	nc.Write(frame(true, 0x9, []byte("still there?"), true))
	if op, p := readFrame(t, br); op != 0xA || string(p) != "still there?" {
		t.Errorf("got opcode %#x %q, want a pong carrying the ping's payload", op, p)
	}
	// A pong from the peer is swallowed, and the message after it arrives.
	nc.Write(frame(true, 0xA, nil, true))
	nc.Write(frame(true, 0x2, []byte("data"), true))
	if r := <-got; r.err != nil || string(r.msg) != "data" {
		t.Errorf("Read = %q, %v", r.msg, r.err)
	}
}

func TestTheControlFrameRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		frame []byte
		code  uint16
	}{
		{"a fragmented ping", frame(false, 0x9, nil, true), ws.CloseProtocolError},
		{"a ping over 125 bytes", frame(true, 0x9, make([]byte, 126), true), ws.CloseProtocolError},
		{"a reserved bit", append([]byte{0xC2}, frame(true, 0x2, []byte("x"), true)[1:]...), ws.CloseProtocolError},
		{"an opcode that is not one", frame(true, 0x3, nil, true), ws.CloseProtocolError},
		{"a text frame", frame(true, 0x1, []byte("hi"), true), ws.CloseUnsupported},
		{"a continuation of nothing", frame(true, 0x0, []byte("x"), true), ws.CloseProtocolError},
		{"a close with one byte of status", frame(true, 0x8, []byte{3}, true), ws.CloseProtocolError},
		{"a close with a code that is not one", frame(true, 0x8, []byte{0x03, 0xE7}, true), ws.CloseProtocolError},
		{"a close whose reason is not UTF-8", frame(true, 0x8, []byte{0x03, 0xE8, 0xFF}, true), ws.CloseBadPayload},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			nc, br, server := raw(t)
			got := read(server)
			nc.Write(tt.frame)
			if code := closeCode(t, br); code != tt.code {
				t.Errorf("the server closed with %d, want %d", code, tt.code)
			}
			wantClose(t, <-got, tt.code)
		})
	}
}

func TestAClientRefusesAMaskedServerFrame(t *testing.T) {
	t.Parallel()
	// A server written by hand, so it can mask a frame the way no server may.
	clientEnd, serverEnd := net.Pipe()
	defer serverEnd.Close()
	go func() {
		br := bufio.NewReader(serverEnd)
		req, err := http.ReadRequest(br)
		if err != nil {
			return
		}
		sum := sha1.Sum([]byte(req.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		io.WriteString(serverEnd, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n"+
			"Sec-WebSocket-Accept: "+base64.StdEncoding.EncodeToString(sum[:])+"\r\n\r\n")
		serverEnd.Write(frame(true, 0x2, []byte("masked"), true))
		io.Copy(io.Discard, serverEnd)
	}()
	client, err := ws.Dial(clientEnd, "lab.test", "/ws")
	if err != nil {
		t.Fatal(err)
	}
	client.ReadTimeout = 5 * time.Second
	_, err = client.Read()
	ce, ok := errors.AsType[*ws.CloseError](err)
	if !ok || ce.Code != ws.CloseProtocolError {
		t.Errorf("Read = %v, want a 1002 close", err)
	}
}

func TestTheDeadlines(t *testing.T) {
	t.Parallel()
	t.Run("a silent peer is a timeout, not a wait for ever", func(t *testing.T) {
		t.Parallel()
		_, server := pair(t)
		server.ReadTimeout = 50 * time.Millisecond
		_, err := server.Read()
		if ne, ok := errors.AsType[net.Error](err); !ok || !ne.Timeout() {
			t.Errorf("Read = %v, want a timeout", err)
		}
	})
	t.Run("a peer that does not read is a timeout", func(t *testing.T) {
		t.Parallel()
		_, server := pair(t)
		server.WriteTimeout = 50 * time.Millisecond
		err := server.Write([]byte("nobody is reading this"))
		if ne, ok := errors.AsType[net.Error](err); !ok || !ne.Timeout() {
			t.Errorf("Write = %v, want a timeout", err)
		}
	})
}
