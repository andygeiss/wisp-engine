package ws

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

// discard is a net.Conn with nothing behind it, so the frame reader can be
// fed bytes from a buffer and its close frames go nowhere.
type discard struct{}

func (discard) Read([]byte) (int, error)         { return 0, io.EOF }
func (discard) Write(p []byte) (int, error)      { return len(p), nil }
func (discard) Close() error                     { return nil }
func (discard) LocalAddr() net.Addr              { return nil }
func (discard) RemoteAddr() net.Addr             { return nil }
func (discard) SetDeadline(time.Time) error      { return nil }
func (discard) SetReadDeadline(time.Time) error  { return nil }
func (discard) SetWriteDeadline(time.Time) error { return nil }

func TestAcceptKeyMatchesTheRFC(t *testing.T) {
	t.Parallel()
	if got := acceptKey("dGhlIHNhbXBsZSBub25jZQ=="); got != "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=" {
		t.Errorf("acceptKey = %q", got)
	}
}

// FuzzRead feeds the frame reader anything at all. It has to end in a message
// no larger than the cap or in one of the three errors a caller is written
// for, and never in a panic or an allocation the cap did not allow.
func FuzzRead(f *testing.F) {
	f.Add([]byte{0x82, 0x81, 1, 2, 3, 4, 'x' ^ 1})                          // one masked byte
	f.Add([]byte{0x82, 0x01, 'x'})                                          // unmasked
	f.Add([]byte{0x89, 0x80, 0, 0, 0, 0})                                   // a ping
	f.Add([]byte{0x88, 0x82, 0, 0, 0, 0, 0x03, 0xE8})                       // close 1000
	f.Add([]byte{0x02, 0x81, 0, 0, 0, 0, 'a', 0x80, 0x81, 0, 0, 0, 0, 'b'}) // two fragments
	f.Add([]byte{0x82, 0xFE, 0xFF, 0xFF, 0, 0, 0, 0})                       // 65535 promised
	f.Add([]byte{0x82, 0xFF, 0x7F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
	f.Add([]byte{0x82, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
	f.Fuzz(func(t *testing.T, data []byte) {
		const cap = 4096
		c := &Conn{nc: discard{}, br: bufio.NewReader(bytes.NewReader(data)), server: true, MaxMessage: cap}
		for {
			msg, err := c.Read()
			if err != nil {
				if _, ok := errors.AsType[*CloseError](err); !ok && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, ErrClosed) {
					t.Fatalf("an error a caller is not written for: %v", err)
				}
				if _, again := c.Read(); !errors.Is(again, ErrClosed) {
					t.Fatalf("Read after an error = %v, want ErrClosed", again)
				}
				return
			}
			if len(msg) > cap {
				t.Fatalf("accepted a %d-byte message over the %d cap", len(msg), cap)
			}
		}
	})
}
