package nativehost

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
)

func encode(t *testing.T, msgs ...any) []byte {
	t.Helper()
	var buf bytes.Buffer
	for _, m := range msgs {
		data, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if err := binary.Write(&buf, binary.LittleEndian, uint32(len(data))); err != nil {
			t.Fatalf("length: %v", err)
		}
		buf.Write(data)
	}
	return buf.Bytes()
}

func decode(t *testing.T, b []byte) []Response {
	t.Helper()
	var out []Response
	r := bytes.NewReader(b)
	for r.Len() > 0 {
		var length uint32
		if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
			t.Fatalf("decode length: %v", err)
		}
		body := make([]byte, length)
		if _, err := r.Read(body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		var resp Response
		if err := json.Unmarshal(body, &resp); err != nil {
			t.Fatalf("decode json: %v", err)
		}
		out = append(out, resp)
	}
	return out
}

func TestRunSingleRequest(t *testing.T) {
	in := encode(t, Request{URL: "https://example.com/", CurrentBrowsers: []string{"Chrome"}})
	var out bytes.Buffer

	err := run(bytes.NewReader(in), &out, func(req Request) Response {
		if req.URL != "https://example.com/" {
			t.Errorf("URL = %q", req.URL)
		}
		return Response{Redirect: true, Browser: "Firefox"}
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	got := decode(t, out.Bytes())
	if len(got) != 1 {
		t.Fatalf("want 1 response, got %d", len(got))
	}
	if !got[0].Redirect || got[0].Browser != "Firefox" {
		t.Errorf("response = %+v", got[0])
	}
}

func TestRunMultipleRequests(t *testing.T) {
	in := encode(t,
		Request{URL: "https://a/"},
		Request{URL: "https://b/"},
		Request{URL: "https://c/"},
	)
	var out bytes.Buffer

	count := 0
	err := run(bytes.NewReader(in), &out, func(req Request) Response {
		count++
		return Response{Redirect: count%2 == 1}
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	got := decode(t, out.Bytes())
	if len(got) != 3 {
		t.Fatalf("want 3 responses, got %d", len(got))
	}
	if !got[0].Redirect || got[1].Redirect || !got[2].Redirect {
		t.Errorf("redirect pattern = %v %v %v", got[0].Redirect, got[1].Redirect, got[2].Redirect)
	}
}

func TestRunCleanEOF(t *testing.T) {
	if err := run(bytes.NewReader(nil), &bytes.Buffer{}, func(Request) Response { return Response{} }); err != nil {
		t.Errorf("EOF should be nil, got %v", err)
	}
}

func TestRunRejectsOversizedFrame(t *testing.T) {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint32(MaxMessageSize+1))

	err := run(&buf, &bytes.Buffer{}, func(Request) Response { return Response{} })
	if err == nil || !strings.Contains(err.Error(), "invalid message length") {
		t.Errorf("expected length-validation error, got %v", err)
	}
}

func TestRunRejectsZeroLength(t *testing.T) {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint32(0))

	err := run(&buf, &bytes.Buffer{}, func(Request) Response { return Response{} })
	if err == nil {
		t.Error("expected error for zero-length frame")
	}
}

func TestRunBadJSONReturnsErrorResponse(t *testing.T) {
	var in bytes.Buffer
	body := []byte(`{not valid json`)
	binary.Write(&in, binary.LittleEndian, uint32(len(body)))
	in.Write(body)

	var out bytes.Buffer
	if err := run(&in, &out, func(Request) Response { return Response{Redirect: true} }); err != nil {
		t.Fatalf("run: %v", err)
	}

	got := decode(t, out.Bytes())
	if len(got) != 1 || got[0].Error == "" {
		t.Errorf("expected error response, got %+v", got)
	}
}

func TestIsCurrentBrowser(t *testing.T) {
	r := Request{CurrentBrowsers: []string{"Google Chrome", "chrome"}}
	cases := map[string]bool{
		"Google Chrome": true,
		"google chrome": true,
		"CHROME":        true,
		"Firefox":       false,
		"":              false,
	}
	for in, want := range cases {
		if got := r.IsCurrentBrowser(in); got != want {
			t.Errorf("IsCurrentBrowser(%q) = %v, want %v", in, got, want)
		}
	}
}
