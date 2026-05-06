// Package nativehost implements Chrome's Native Messaging wire protocol so
// browser extensions can ask the daemon to route a URL — i.e. when a link
// click happens INSIDE a Chromium-family browser and would otherwise stay
// in that same browser. The OS doesn't see those clicks, only the browser
// does, so a small extension intercepts them and forwards each URL to us
// over stdio.
//
// Wire format (Chrome MV3 spec):
//   - 4-byte little-endian uint32 length prefix
//   - UTF-8 JSON payload
//   - one request → one response, repeated until stdin EOF
//   - inbound message size capped at 1 MiB by Chrome; we mirror that limit
package nativehost

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// MaxMessageSize is the inbound size cap (matches Chrome's 1 MiB ceiling).
const MaxMessageSize = 1 << 20

// Request is what the browser extension sends per click.
type Request struct {
	URL string `json:"url"`
	// CurrentBrowsers lists names that the calling browser identifies as.
	// The Chrome build of the extension sends e.g. ["Google Chrome",
	// "Chrome", "google-chrome"]; the Brave build sends Brave-flavoured
	// aliases. The handler uses this to decide passthrough vs redirect.
	CurrentBrowsers []string `json:"current_browsers,omitempty"`
}

// IsCurrentBrowser reports whether name (case-insensitive) is one of the
// aliases the calling browser sent for itself.
func (r Request) IsCurrentBrowser(name string) bool {
	for _, b := range r.CurrentBrowsers {
		if equalFold(b, name) {
			return true
		}
	}
	return false
}

// Response is the answer back to the extension.
//
// Redirect=true means the host has already opened the URL elsewhere; the
// extension must cancel the in-browser navigation. Redirect=false means
// the extension should let the navigation proceed.
type Response struct {
	Redirect bool   `json:"redirect"`
	Browser  string `json:"browser,omitempty"`
	Profile  string `json:"profile,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Handler decides what to do with one Request. It must always return a
// Response (use Error for soft failures); returning an error here is
// treated as fatal and ends the session.
type Handler func(Request) Response

// Run reads/writes Chrome's native-messaging frames on stdin/stdout until
// EOF. Returns nil on clean EOF; any other error is fatal for the host
// process.
func Run(handle Handler) error {
	return run(os.Stdin, os.Stdout, handle)
}

func run(in io.Reader, out io.Writer, handle Handler) error {
	r := bufio.NewReader(in)
	for {
		var length uint32
		if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("read length prefix: %w", err)
		}
		if length == 0 || length > MaxMessageSize {
			return fmt.Errorf("invalid message length %d", length)
		}

		buf := make([]byte, length)
		if _, err := io.ReadFull(r, buf); err != nil {
			return fmt.Errorf("read body (%d bytes): %w", length, err)
		}

		var req Request
		if err := json.Unmarshal(buf, &req); err != nil {
			if werr := writeMessage(out, Response{Error: "bad request: " + err.Error()}); werr != nil {
				return werr
			}
			continue
		}

		resp := handle(req)
		if err := writeMessage(out, resp); err != nil {
			return fmt.Errorf("write response: %w", err)
		}
	}
}

func writeMessage(w io.Writer, msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if len(data) > MaxMessageSize {
		return fmt.Errorf("response %d bytes exceeds %d-byte limit", len(data), MaxMessageSize)
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(len(data))); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// equalFold avoids importing strings just for one comparison and lets us
// match without allocation.
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
