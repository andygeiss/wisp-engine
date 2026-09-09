// Package ws is RFC 6455 for the lab's server: [Accept] turns an HTTP request
// into a WebSocket, [Dial] is the client half for tests and bots, and a [Conn]
// moves binary messages over either.
//
// It is written by hand because the standard library has no WebSocket and the
// dependency rule refuses one — the same move as the engine's sheet scanner,
// for the same reason. The whole protocol is a SHA-1 and a base64 for the
// handshake, a frame header of at most fourteen bytes, and a few rules about
// masks, fragments and control frames. It never compiles for the browser,
// which brings its own WebSocket, so it costs the wasm module nothing.
//
// A Conn speaks binary messages only. A text frame closes the connection with
// 1003, a broken frame with 1002, and one that claims more bytes than
// [Conn.MaxMessage] allows with 1009 — before anything is allocated for it.
// Read from one goroutine; Write, Ping and Close may be called from any.
package ws

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

// The close codes this package sends and reports, from RFC 6455 section 7.4.
const (
	CloseNormal        uint16 = 1000
	CloseGoingAway     uint16 = 1001
	CloseProtocolError uint16 = 1002
	CloseUnsupported   uint16 = 1003
	CloseNoStatus      uint16 = 1005
	CloseBadPayload    uint16 = 1007
	ClosePolicy        uint16 = 1008
	CloseTooLarge      uint16 = 1009
)

// DefaultMaxMessage is the largest message a Conn accepts when
// [Conn.MaxMessage] is zero: one mebibyte.
const DefaultMaxMessage = 1 << 20

// guid is the string RFC 6455 appends to the client's key before hashing it.
// SHA-1 is the RFC's choice, not a security one: the handshake proves the
// peer speaks WebSocket, nothing more.
const guid = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// ErrClosed is returned by Read and Write once the connection is closed by
// this side.
var ErrClosed = errors.New("ws: connection closed")

// CloseError is how a connection ended: the code and reason the peer sent, or
// the ones this side sent when the peer broke the protocol. [Conn.Read]
// returns it, and nothing else, once the socket is done.
type CloseError struct {
	Code   uint16
	Reason string
}

// Error names the code, and the reason when there is one.
func (e *CloseError) Error() string {
	if e.Reason == "" {
		return "ws: closed with " + strconv.Itoa(int(e.Code))
	}
	return "ws: closed with " + strconv.Itoa(int(e.Code)) + ": " + e.Reason
}

// The frame opcodes.
const (
	opContinuation = 0x0
	opText         = 0x1
	opBinary       = 0x2
	opClose        = 0x8
	opPing         = 0x9
	opPong         = 0xA
)

// controlMax is the largest control-frame payload the RFC allows.
const controlMax = 125

// Conn is one WebSocket. Zero is not usable; [Accept] and [Dial] make one.
type Conn struct {
	// MaxMessage caps a message, fragments included, in bytes. A frame
	// claiming more closes the connection with 1009 before the bytes are read
	// or space is made for them. 0 means [DefaultMaxMessage].
	MaxMessage int
	// ReadTimeout is how long Read waits for the next frame before giving
	// up, or 0 to wait for ever. A server sets it to twice its ping interval,
	// so a peer that has gone quiet is found rather than kept.
	ReadTimeout time.Duration
	// WriteTimeout bounds each Write, Ping and Close, or 0 for no bound.
	WriteTimeout time.Duration

	nc     net.Conn
	br     *bufio.Reader
	server bool // the peer's frames are masked and ours are not

	// Reassembly of a fragmented message, owned by Read.
	frag []byte

	wmu       sync.Mutex
	wbuf      []byte
	sentClose bool
	// closed is read by Read on its goroutine and set by whoever closes, so
	// it is the one field shared without the write lock.
	closed atomic.Bool
}

// Accept upgrades r to a WebSocket and returns the connection, or answers r
// with the status the refusal deserves — 400 for a handshake that is not one,
// 403 for an Origin that is not this host, 426 for a version that is not 13 —
// and returns why. The handler has nothing left to write in either case.
//
// The Origin check is RFC 6455's own and stays here rather than in a
// middleware: the request is a GET, which the CSRF middleware waves through.
// The returned Conn has no deadlines: the ones net/http set on the connection
// are cleared, and the caller sets ReadTimeout and WriteTimeout instead.
func Accept(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	if !headerHasToken(r.Header, "Connection", "upgrade") || !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		http.Error(w, "this URL speaks WebSocket: send Connection: Upgrade and Upgrade: websocket", http.StatusBadRequest)
		return nil, errors.New("ws: not a websocket handshake")
	}
	if v := r.Header.Get("Sec-WebSocket-Version"); v != "13" {
		w.Header().Set("Sec-WebSocket-Version", "13")
		http.Error(w, "this server speaks WebSocket version 13", http.StatusUpgradeRequired)
		return nil, fmt.Errorf("ws: version %q, want 13", v)
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if raw, err := base64.StdEncoding.DecodeString(key); err != nil || len(raw) != 16 {
		http.Error(w, "Sec-WebSocket-Key must be 16 random bytes in base64", http.StatusBadRequest)
		return nil, fmt.Errorf("ws: key %q is not 16 bytes of base64", key)
	}
	origin := r.Header.Get("Origin")
	if u, err := url.Parse(origin); origin == "" || err != nil || !strings.EqualFold(u.Host, r.Host) {
		http.Error(w, "the page asking for this socket is not served by this host", http.StatusForbidden)
		return nil, fmt.Errorf("ws: origin %q is not this host %q", origin, r.Host)
	}

	nc, brw, err := http.NewResponseController(w).Hijack()
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return nil, fmt.Errorf("ws: hijacking the connection: %w", err)
	}
	// net/http set its own read and write deadlines on this connection, and
	// they would still fire. The caller's timeouts replace them.
	if err := nc.SetDeadline(time.Time{}); err != nil {
		nc.Close()
		return nil, fmt.Errorf("ws: clearing the deadlines: %w", err)
	}
	reply := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + acceptKey(key) + "\r\n\r\n"
	if _, err := io.WriteString(nc, reply); err != nil {
		nc.Close()
		return nil, fmt.Errorf("ws: writing the handshake: %w", err)
	}
	// The reader net/http handed back may already hold the first frame, so
	// frames are read through it rather than from the connection directly.
	return &Conn{nc: nc, br: brw.Reader, server: true}, nil
}

// Dial performs the client handshake over nc — a connection the caller
// already opened, so a test can hand in one end of a [net.Pipe] — asking for
// path on host, with an Origin of http://host, which is what a page served by
// that host would send.
func Dial(nc net.Conn, host, path string) (*Conn, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("ws: making a key: %w", err)
	}
	key := base64.StdEncoding.EncodeToString(nonce[:])
	req := "GET " + path + " HTTP/1.1\r\n" +
		"Host: " + host + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n" +
		"Origin: http://" + host + "\r\n\r\n"
	if _, err := io.WriteString(nc, req); err != nil {
		return nil, fmt.Errorf("ws: writing the handshake: %w", err)
	}
	br := bufio.NewReader(nc)
	res, err := http.ReadResponse(br, &http.Request{Method: http.MethodGet})
	if err != nil {
		return nil, fmt.Errorf("ws: reading the handshake: %w", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusSwitchingProtocols {
		return nil, fmt.Errorf("ws: handshake answered %s, want 101", res.Status)
	}
	if got := res.Header.Get("Sec-WebSocket-Accept"); got != acceptKey(key) {
		return nil, fmt.Errorf("ws: handshake accept key %q is not the one for this key", got)
	}
	return &Conn{nc: nc, br: br, server: false}, nil
}

// acceptKey is the answer to a client's key: the base64 of the SHA-1 of the
// key with the RFC's GUID appended. The RFC's own example is the test vector.
func acceptKey(key string) string {
	sum := sha1.Sum([]byte(key + guid))
	return base64.StdEncoding.EncodeToString(sum[:])
}

// headerHasToken reports whether a comma-separated header lists token,
// ignoring case: browsers send "Connection: keep-alive, Upgrade".
func headerHasToken(h http.Header, name, token string) bool {
	for _, v := range h.Values(name) {
		for t := range strings.SplitSeq(v, ",") {
			if strings.EqualFold(strings.TrimSpace(t), token) {
				return true
			}
		}
	}
	return false
}

// Read returns the next binary message, fragments reassembled. Pings are
// answered and pongs swallowed on the way. It returns a [*CloseError] once
// the connection is closed — by the peer, or by this side because the peer
// broke a rule — and [ErrClosed] after a Close of our own.
func (c *Conn) Read() ([]byte, error) {
	for {
		fin, op, payload, err := c.readFrame()
		if err != nil {
			return nil, err
		}
		switch op {
		case opBinary:
			if c.frag != nil {
				return nil, c.fail(CloseProtocolError, "a new message inside a fragmented one")
			}
			if fin {
				return payload, nil
			}
			c.frag = payload
		case opContinuation:
			if c.frag == nil {
				return nil, c.fail(CloseProtocolError, "a continuation with nothing to continue")
			}
			if len(c.frag)+len(payload) > c.maxMessage() {
				return nil, c.fail(CloseTooLarge, "message over "+strconv.Itoa(c.maxMessage())+" bytes")
			}
			c.frag = append(c.frag, payload...)
			if fin {
				msg := c.frag
				c.frag = nil
				return msg, nil
			}
		case opText:
			return nil, c.fail(CloseUnsupported, "text frames are not spoken here")
		case opPing:
			if err := c.writeFrame(opPong, payload); err != nil {
				return nil, err
			}
		case opPong:
			// A pong answers a ping this side sent. Arriving is the whole
			// message; the read deadline it refreshed is what it was for.
		case opClose:
			return nil, c.peerClosed(payload)
		default:
			return nil, c.fail(CloseProtocolError, "opcode "+strconv.Itoa(int(op))+" is not one")
		}
	}
}

// readFrame reads one frame's header and payload, checking every rule that
// can be checked before the payload is read: the reserved bits, the mask, the
// size, and the control-frame limits.
func (c *Conn) readFrame() (fin bool, op byte, payload []byte, err error) {
	if c.closed.Load() {
		return false, 0, nil, ErrClosed
	}
	if c.ReadTimeout > 0 {
		if err := c.nc.SetReadDeadline(time.Now().Add(c.ReadTimeout)); err != nil {
			return false, 0, nil, err
		}
	}
	var hdr [2]byte
	if _, err := io.ReadFull(c.br, hdr[:]); err != nil {
		return false, 0, nil, c.lost(err)
	}
	fin = hdr[0]&0x80 != 0
	op = hdr[0] & 0x0F
	masked := hdr[1]&0x80 != 0
	length := uint64(hdr[1] & 0x7F)

	if hdr[0]&0x70 != 0 {
		return false, 0, nil, c.fail(CloseProtocolError, "reserved bits set with no extension agreed")
	}
	if op&0x8 != 0 && (!fin || length > controlMax) {
		return false, 0, nil, c.fail(CloseProtocolError, "a control frame must be short and whole")
	}
	switch length {
	case 126:
		var ext [2]byte
		if _, err := io.ReadFull(c.br, ext[:]); err != nil {
			return false, 0, nil, c.lost(err)
		}
		length = uint64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err := io.ReadFull(c.br, ext[:]); err != nil {
			return false, 0, nil, c.lost(err)
		}
		length = binary.BigEndian.Uint64(ext[:])
		if length>>63 != 0 {
			return false, 0, nil, c.fail(CloseProtocolError, "a length with its top bit set")
		}
	}
	// A client masks and a server does not, and each side refuses the
	// other's mistake: an unmasked client frame is the RFC's one MUST-close.
	if masked != c.server {
		if c.server {
			return false, 0, nil, c.fail(CloseProtocolError, "a client frame must be masked")
		}
		return false, 0, nil, c.fail(CloseProtocolError, "a server frame must not be masked")
	}
	// The size is checked here, against the count in the header, so a frame
	// that claims a gigabyte costs a header and a close rather than a
	// gigabyte.
	if length > uint64(c.maxMessage()) || uint64(len(c.frag))+length > uint64(c.maxMessage()) {
		return false, 0, nil, c.fail(CloseTooLarge, "message over "+strconv.Itoa(c.maxMessage())+" bytes")
	}

	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(c.br, mask[:]); err != nil {
			return false, 0, nil, c.lost(err)
		}
	}
	payload = make([]byte, length)
	if _, err := io.ReadFull(c.br, payload); err != nil {
		return false, 0, nil, c.lost(err)
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i&3]
		}
	}
	return fin, op, payload, nil
}

// peerClosed answers the peer's close frame with one of our own and turns it
// into the error Read returns.
func (c *Conn) peerClosed(payload []byte) error {
	code, reason := CloseNoStatus, ""
	switch {
	case len(payload) == 1:
		return c.fail(CloseProtocolError, "a close frame with one byte of status")
	case len(payload) >= 2:
		code = binary.BigEndian.Uint16(payload)
		reason = string(payload[2:])
		if !validCloseCode(code) {
			return c.fail(CloseProtocolError, "close code "+strconv.Itoa(int(code))+" is not one")
		}
		if !utf8.ValidString(reason) {
			return c.fail(CloseBadPayload, "a close reason that is not UTF-8")
		}
	}
	echo := CloseNormal
	if len(payload) >= 2 {
		echo = code
	}
	c.wmu.Lock()
	if !c.sentClose {
		c.sentClose = true
		c.writeFrameLocked(opClose, closePayload(echo, ""))
	}
	c.closed.Store(true)
	c.nc.Close()
	c.wmu.Unlock()
	return &CloseError{Code: code, Reason: reason}
}

// validCloseCode reports whether a peer may send code, per section 7.4.
func validCloseCode(code uint16) bool {
	switch {
	case code >= 1000 && code <= 1003, code >= 1007 && code <= 1011, code >= 3000 && code <= 4999:
		return true
	}
	return false
}

// fail closes the connection because the peer broke a rule: a close frame
// with the code, best effort, then the socket. The error names the rule.
func (c *Conn) fail(code uint16, reason string) error {
	c.close(code, reason)
	return &CloseError{Code: code, Reason: reason}
}

// lost turns a read that ended early into the error Read returns. A timeout
// or a hang-up is not a protocol violation, so no close frame is owed; the
// socket is closed so nothing else waits on it.
func (c *Conn) lost(err error) error {
	if c.closed.Load() {
		return ErrClosed
	}
	if errors.Is(err, io.EOF) {
		err = io.ErrUnexpectedEOF
	}
	c.wmu.Lock()
	c.closed.Store(true)
	c.nc.Close()
	c.wmu.Unlock()
	return err
}

// Write sends p as one binary message.
func (c *Conn) Write(p []byte) error { return c.writeFrame(opBinary, p) }

// Ping sends a ping. A peer that is still there answers with a pong, which
// Read swallows; the point is the read deadline that arriving refreshes.
func (c *Conn) Ping(p []byte) error { return c.writeFrame(opPing, p) }

// Close sends a close frame with code and reason, if none has been sent yet,
// and closes the socket. It does not wait for the peer's own close frame: the
// peer has the code, which is what a close frame is for, and the socket
// closing behind it ends anything the peer was still sending. Safe to call
// more than once and from any goroutine; a Read in progress returns
// [ErrClosed].
func (c *Conn) Close(code uint16, reason string) error {
	c.close(code, reason)
	return nil
}

func (c *Conn) close(code uint16, reason string) {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if !c.sentClose && !c.closed.Load() {
		c.sentClose = true
		c.writeFrameLocked(opClose, closePayload(code, reason))
	}
	c.closed.Store(true)
	c.nc.Close()
}

// closePayload is a close frame's body: the code, then the reason cut to fit
// a control frame.
func closePayload(code uint16, reason string) []byte {
	if len(reason) > controlMax-2 {
		reason = reason[:controlMax-2]
	}
	return append(binary.BigEndian.AppendUint16(nil, code), reason...)
}

func (c *Conn) writeFrame(op byte, p []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if c.closed.Load() || c.sentClose {
		return ErrClosed
	}
	return c.writeFrameLocked(op, p)
}

// writeFrameLocked writes one whole frame: the header, the mask when this is
// the client, and the payload, in one write from a buffer the Conn keeps.
func (c *Conn) writeFrameLocked(op byte, p []byte) error {
	b := append(c.wbuf[:0], 0x80|op)
	maskBit := byte(0)
	if !c.server {
		maskBit = 0x80
	}
	switch n := len(p); {
	case n <= 125:
		b = append(b, maskBit|byte(n))
	case n <= 1<<16-1:
		b = append(b, maskBit|126)
		b = binary.BigEndian.AppendUint16(b, uint16(n))
	default:
		b = append(b, maskBit|127)
		b = binary.BigEndian.AppendUint64(b, uint64(n))
	}
	if c.server {
		b = append(b, p...)
	} else {
		var mask [4]byte
		rand.Read(mask[:])
		b = append(b, mask[:]...)
		start := len(b)
		b = append(b, p...)
		for i := range p {
			b[start+i] ^= mask[i&3]
		}
	}
	c.wbuf = b
	if c.WriteTimeout > 0 {
		if err := c.nc.SetWriteDeadline(time.Now().Add(c.WriteTimeout)); err != nil {
			return err
		}
	}
	_, err := c.nc.Write(b)
	return err
}

func (c *Conn) maxMessage() int {
	if c.MaxMessage <= 0 {
		return DefaultMaxMessage
	}
	return c.MaxMessage
}
