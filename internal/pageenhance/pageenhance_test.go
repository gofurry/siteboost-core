package pageenhance

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyResponseTransformsHeadersAndHTML(t *testing.T) {
	pipeline := New(Config{
		Enabled:     true,
		OnError:     OnErrorPassThrough,
		MaxBodySize: DefaultMaxBodySize,
		Transforms: []Transform{{
			Name: "demo",
			Match: Match{
				ContentTypes: []string{"text/html"},
				StatusCodes:  []int{200},
			},
			HeaderSet:    map[string]string{"X-Enhanced": "yes"},
			HeaderRemove: []string{"X-Remove"},
			InjectHead:   `<script src="/local.js"></script>`,
			InjectBody:   `<div id="boost"></div>`,
			Replace: []Replacement{{
				Old: "old text",
				New: "new text",
			}},
		}},
	})
	resp := testResponse(`<html><head><title>x</title></head><body>old text</body></html>`)
	resp.Header.Set("Content-Type", "text/html; charset=utf-8")
	resp.Header.Set("ETag", `"abc"`)
	resp.Header.Set("X-Remove", "gone")
	events, err := pipeline.ApplyResponse(resp, "store.steampowered.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatalf("events are empty")
	}
	body := readResponseBody(t, resp)
	for _, want := range []string{`<script src="/local.js"></script></head>`, `<div id="boost"></div></body>`, "new text"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body = %q, want %q", body, want)
		}
	}
	if got := resp.Header.Get("X-Enhanced"); got != "yes" {
		t.Fatalf("X-Enhanced = %q", got)
	}
	if got := resp.Header.Get("X-Remove"); got != "" {
		t.Fatalf("X-Remove = %q", got)
	}
	if got := resp.Header.Get("ETag"); got != "" {
		t.Fatalf("ETag should be removed, got %q", got)
	}
	if status := pipeline.Status(); status.Applied != 1 || status.Errors != 0 {
		t.Fatalf("status = %#v", status)
	}
}

func TestApplyResponsePassThroughRestoresOriginalOnTransformError(t *testing.T) {
	pipeline := New(Config{
		Enabled: true,
		OnError: OnErrorPassThrough,
		Transforms: []Transform{{
			Name:      "bad",
			HeaderSet: map[string]string{"X-Enhanced": "yes"},
			Replace:   []Replacement{{New: "nope"}},
		}},
	})
	resp := testResponse(`<html><head></head><body>original</body></html>`)
	resp.Header.Set("Content-Type", "text/html")
	_, err := pipeline.ApplyResponse(resp, "store.steampowered.com")
	if err != nil {
		t.Fatal(err)
	}
	if body := readResponseBody(t, resp); body != `<html><head></head><body>original</body></html>` {
		t.Fatalf("body = %q", body)
	}
	if got := resp.Header.Get("X-Enhanced"); got != "" {
		t.Fatalf("header should be restored, got %q", got)
	}
	if status := pipeline.Status(); status.Errors != 1 {
		t.Fatalf("status = %#v", status)
	}
}

func TestApplyResponseFailClosedReturnsTransformError(t *testing.T) {
	pipeline := New(Config{
		Enabled: true,
		OnError: OnErrorFailClosed,
		Transforms: []Transform{{
			Name:    "bad",
			Replace: []Replacement{{New: "nope"}},
		}},
	})
	resp := testResponse(`<html><head></head><body>original</body></html>`)
	resp.Header.Set("Content-Type", "text/html")
	if _, err := pipeline.ApplyResponse(resp, "store.steampowered.com"); err == nil {
		t.Fatalf("ApplyResponse returned nil error")
	}
}

func TestApplyResponseBodyLimitPreservesOriginalStream(t *testing.T) {
	pipeline := New(Config{
		Enabled:     true,
		OnError:     OnErrorPassThrough,
		MaxBodySize: 4,
		Transforms: []Transform{{
			Name:    "replace",
			Replace: []Replacement{{Old: "a", New: "z"}},
		}},
	})
	resp := testResponse("abcdef")
	resp.ContentLength = -1
	resp.Header.Del("Content-Length")
	events, err := pipeline.ApplyResponse(resp, "store.steampowered.com")
	if err != nil {
		t.Fatal(err)
	}
	if body := readResponseBody(t, resp); body != "abcdef" {
		t.Fatalf("body = %q", body)
	}
	if len(events) != 1 || events[0].Action != "skip" || events[0].Reason != "body_too_large" {
		t.Fatalf("events = %#v", events)
	}
}

func TestApplyResponseCustomTransformer(t *testing.T) {
	pipeline := New(Config{
		Enabled: true,
		CustomTransformers: []Transformer{
			customTransformer{
				name:      "custom",
				provider:  "steam",
				needsBody: true,
			},
		},
	})
	resp := testResponse(`<html><body>custom</body></html>`)
	resp.Header.Set("Content-Type", "text/html")
	events, err := pipeline.ApplyResponseWithMeta(resp, ResponseMeta{
		Provider: "steam",
		Host:     "store.steampowered.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if body := readResponseBody(t, resp); !strings.Contains(body, "enhanced") {
		t.Fatalf("body = %q", body)
	}
	if got := resp.Header.Get("X-Custom"); got != "yes" {
		t.Fatalf("X-Custom = %q", got)
	}
	if len(events) != 1 || events[0].Action != "apply" || events[0].Provider != "steam" {
		t.Fatalf("events = %#v", events)
	}
	if status := pipeline.Status(); status.Transforms != 1 || status.Applied != 1 {
		t.Fatalf("status = %#v", status)
	}
}

func TestServeAsset(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "asset.js")
	writeTestFile(t, file, `console.log("ok");`)
	pipeline := New(Config{
		Enabled: true,
		Assets: []Asset{{
			Path:        "/asset.js",
			File:        file,
			ContentType: "application/javascript",
		}},
	})
	req := httptest.NewRequest(http.MethodGet, "http://store.steampowered.com/asset.js", nil)
	rec := httptest.NewRecorder()
	served, events := pipeline.ServeAsset(rec, req, "store.steampowered.com")
	if !served {
		t.Fatalf("asset was not served")
	}
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `console.log("ok");` {
		t.Fatalf("status/body = %d/%q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/javascript" {
		t.Fatalf("content type = %q", got)
	}
	if len(events) != 1 || events[0].Action != "asset" {
		t.Fatalf("events = %#v", events)
	}
}

type customTransformer struct {
	name      string
	provider  string
	needsBody bool
}

func (t customTransformer) Name() string {
	return t.name
}

func (t customTransformer) Match(meta ResponseMeta) bool {
	return meta.Provider == t.provider
}

func (t customTransformer) NeedsBody() bool {
	return t.needsBody
}

func (t customTransformer) Transform(resp *http.Response, body []byte, _ ResponseMeta) (TransformResult, error) {
	resp.Header.Set("X-Custom", "yes")
	return TransformResult{
		Body:         []byte(strings.ReplaceAll(string(body), "custom", "enhanced")),
		Applied:      true,
		BodyModified: true,
	}, nil
}

func testResponse(body string) *http.Response {
	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       httptest.NewRequest(http.MethodGet, "http://store.steampowered.com/", nil),
	}
}

func readResponseBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
