package pageenhance

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/gofurry/go-steam-core/internal/config"
	"github.com/gofurry/go-steam-core/internal/rules"
)

const (
	OnErrorPassThrough = "pass_through"
	OnErrorFailClosed  = "fail_closed"

	DefaultMaxBodySize = int64(1024 * 1024)
)

type Config struct {
	Enabled            bool
	OnError            string
	MaxBodySize        int64
	Assets             []Asset
	Transforms         []Transform
	CustomTransformers []Transformer
}

type Asset struct {
	Path        string
	File        string
	ContentType string
}

type Transform struct {
	Name         string
	Match        Match
	HeaderSet    map[string]string
	HeaderRemove []string
	InjectHead   string
	InjectBody   string
	Replace      []Replacement
}

type Match struct {
	Providers    []string
	Hosts        []string
	PathPrefixes []string
	ContentTypes []string
	StatusCodes  []int
}

type Replacement struct {
	Old   string
	New   string
	Count int
}

type ResponseMeta struct {
	Provider    string
	Host        string
	Path        string
	ContentType string
	StatusCode  int
}

type Event struct {
	Action    string
	Transform string
	Reason    string
	Provider  string
	Host      string
	Path      string
	Status    int
}

type TransformResult struct {
	Body         []byte
	Applied      bool
	BodyModified bool
	SkipReason   string
}

type Transformer interface {
	Name() string
	Match(ResponseMeta) bool
	NeedsBody() bool
	Transform(*http.Response, []byte, ResponseMeta) (TransformResult, error)
}

type Status struct {
	Enabled    bool   `json:"enabled"`
	OnError    string `json:"on_error"`
	Transforms int    `json:"transforms"`
	Assets     int    `json:"assets"`
	Applied    uint64 `json:"applied"`
	Skipped    uint64 `json:"skipped"`
	Errors     uint64 `json:"errors"`
}

type Pipeline struct {
	cfg     Config
	applied atomic.Uint64
	skipped atomic.Uint64
	errors  atomic.Uint64
}

func ConfigFromApp(cfg config.Config) Config {
	out := Config{
		Enabled:     cfg.PageEnhance.Enabled,
		OnError:     cfg.PageEnhance.OnError,
		MaxBodySize: cfg.PageEnhance.MaxBodySize,
		Assets:      make([]Asset, 0, len(cfg.PageEnhance.Assets)),
		Transforms:  make([]Transform, 0, len(cfg.PageEnhance.Transforms)),
	}
	for _, asset := range cfg.PageEnhance.Assets {
		out.Assets = append(out.Assets, Asset{
			Path:        asset.Path,
			File:        asset.File,
			ContentType: asset.ContentType,
		})
	}
	for _, transform := range cfg.PageEnhance.Transforms {
		out.Transforms = append(out.Transforms, Transform{
			Name: transform.Name,
			Match: Match{
				Providers:    append([]string(nil), transform.Match.Providers...),
				Hosts:        append([]string(nil), transform.Match.Hosts...),
				PathPrefixes: append([]string(nil), transform.Match.PathPrefixes...),
				ContentTypes: append([]string(nil), transform.Match.ContentTypes...),
				StatusCodes:  append([]int(nil), transform.Match.StatusCodes...),
			},
			HeaderSet:    cloneStringMap(transform.Headers.Set),
			HeaderRemove: append([]string(nil), transform.Headers.Remove...),
			InjectHead:   transform.InjectHead,
			InjectBody:   transform.InjectBody,
			Replace:      replacementsFromConfig(transform.Replace),
		})
	}
	return out
}

func New(cfg Config) *Pipeline {
	if strings.TrimSpace(cfg.OnError) == "" {
		cfg.OnError = OnErrorPassThrough
	}
	if cfg.MaxBodySize <= 0 {
		cfg.MaxBodySize = DefaultMaxBodySize
	}
	return &Pipeline{cfg: cfg}
}

func (p *Pipeline) Enabled() bool {
	return p != nil && p.cfg.Enabled
}

func (p *Pipeline) Status() Status {
	if p == nil {
		return Status{}
	}
	return Status{
		Enabled:    p.cfg.Enabled,
		OnError:    p.cfg.OnError,
		Transforms: len(p.cfg.Transforms) + len(p.cfg.CustomTransformers),
		Assets:     len(p.cfg.Assets),
		Applied:    p.applied.Load(),
		Skipped:    p.skipped.Load(),
		Errors:     p.errors.Load(),
	}
}

func (p *Pipeline) ServeAsset(w http.ResponseWriter, req *http.Request, host string) (bool, []Event) {
	if !p.Enabled() {
		return false, nil
	}
	for _, asset := range p.cfg.Assets {
		if req.URL == nil || req.URL.Path != asset.Path {
			continue
		}
		event := Event{Action: "asset", Transform: asset.Path, Host: host, Path: asset.Path, Status: http.StatusOK}
		data, err := os.ReadFile(asset.File)
		if err != nil {
			p.errors.Add(1)
			event.Action = "error"
			event.Reason = fmt.Sprintf("read_asset: %v", err)
			event.Status = http.StatusInternalServerError
			http.Error(w, "page enhance asset failed", http.StatusInternalServerError)
			return true, []Event{event}
		}
		contentType := strings.TrimSpace(asset.ContentType)
		if contentType == "" {
			contentType = http.DetectContentType(data)
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
		p.applied.Add(1)
		return true, []Event{event}
	}
	return false, nil
}

func (p *Pipeline) ApplyResponse(resp *http.Response, host string) ([]Event, error) {
	return p.ApplyResponseWithMeta(resp, ResponseMeta{Host: host})
}

func (p *Pipeline) ApplyResponseWithMeta(resp *http.Response, meta ResponseMeta) ([]Event, error) {
	if !p.Enabled() || resp == nil {
		return nil, nil
	}
	meta = completeResponseMeta(resp, meta)
	transforms := p.matchingTransforms(meta)
	customTransformers := p.matchingCustomTransformers(meta)
	if len(transforms) == 0 && len(customTransformers) == 0 {
		return nil, nil
	}

	originalHeader := resp.Header.Clone()
	var originalBody []byte
	var body []byte
	bodyLoaded := false
	needsBody := transformsNeedBody(transforms) || transformersNeedBody(customTransformers)
	events := make([]Event, 0, len(transforms)+len(customTransformers))

	if needsBody {
		var err error
		var skipReason string
		body, skipReason, err = p.readBody(resp)
		if err != nil {
			p.errors.Add(1)
			event := eventFromMeta("error", "", err.Error(), meta)
			if p.cfg.OnError == OnErrorFailClosed {
				return []Event{event}, err
			}
			p.restore(resp, originalHeader, originalBody, bodyLoaded)
			return []Event{event}, nil
		}
		if body == nil {
			if skipReason == "" {
				skipReason = "body_unavailable"
			}
			for _, transform := range transforms {
				if transform.hasBodyOps() {
					p.skipped.Add(1)
					events = append(events, eventFromMeta("skip", transform.label(), skipReason, meta))
				}
			}
			for _, transformer := range customTransformers {
				if transformer.NeedsBody() {
					p.skipped.Add(1)
					events = append(events, eventFromMeta("skip", transformerName(transformer), skipReason, meta))
				}
			}
		} else {
			originalBody = append([]byte(nil), body...)
			bodyLoaded = true
		}
	}

	modifiedBody := false
	for _, transform := range transforms {
		appliedHeader := applyHeaders(resp.Header, transform)
		appliedBody := false
		if bodyLoaded && transform.hasBodyOps() {
			next, applied, skippedReason, err := applyBodyTransform(body, transform)
			if err != nil {
				p.errors.Add(1)
				event := eventFromMeta("error", transform.label(), err.Error(), meta)
				if p.cfg.OnError == OnErrorFailClosed {
					return append(events, event), err
				}
				p.restore(resp, originalHeader, originalBody, bodyLoaded)
				return append(events, event), nil
			}
			if skippedReason != "" {
				p.skipped.Add(1)
				events = append(events, eventFromMeta("skip", transform.label(), skippedReason, meta))
			}
			if applied {
				body = next
				appliedBody = true
				modifiedBody = true
			}
		}
		if appliedHeader || appliedBody {
			p.applied.Add(1)
			events = append(events, eventFromMeta("apply", transform.label(), "", meta))
		}
	}
	for _, transformer := range customTransformers {
		if transformer.NeedsBody() && !bodyLoaded {
			continue
		}
		result, err := transformer.Transform(resp, body, meta)
		name := transformerName(transformer)
		if err != nil {
			p.errors.Add(1)
			event := eventFromMeta("error", name, err.Error(), meta)
			if p.cfg.OnError == OnErrorFailClosed {
				return append(events, event), err
			}
			p.restore(resp, originalHeader, originalBody, bodyLoaded)
			return append(events, event), nil
		}
		if result.SkipReason != "" {
			p.skipped.Add(1)
			events = append(events, eventFromMeta("skip", name, result.SkipReason, meta))
		}
		if result.BodyModified {
			body = result.Body
			modifiedBody = true
		}
		if result.Applied {
			p.applied.Add(1)
			events = append(events, eventFromMeta("apply", name, "", meta))
		}
	}
	if modifiedBody {
		setResponseBody(resp, body)
		resp.Header.Del("ETag")
		resp.Header.Del("Content-MD5")
	} else if bodyLoaded {
		setResponseBody(resp, body)
	}
	return events, nil
}

func completeResponseMeta(resp *http.Response, meta ResponseMeta) ResponseMeta {
	if resp.Request != nil && resp.Request.URL != nil {
		if meta.Path == "" {
			meta.Path = resp.Request.URL.Path
		}
	}
	if meta.ContentType == "" {
		meta.ContentType = responseContentType(resp.Header.Get("Content-Type"))
	}
	if meta.StatusCode == 0 {
		meta.StatusCode = resp.StatusCode
	}
	return meta
}

func (p *Pipeline) matchingTransforms(meta ResponseMeta) []Transform {
	if len(p.cfg.Transforms) == 0 {
		return nil
	}
	out := make([]Transform, 0, len(p.cfg.Transforms))
	for _, transform := range p.cfg.Transforms {
		if transform.Match.matches(meta) {
			out = append(out, transform)
		}
	}
	return out
}

func (p *Pipeline) matchingCustomTransformers(meta ResponseMeta) []Transformer {
	if len(p.cfg.CustomTransformers) == 0 {
		return nil
	}
	out := make([]Transformer, 0, len(p.cfg.CustomTransformers))
	for _, transformer := range p.cfg.CustomTransformers {
		if transformer != nil && transformer.Match(meta) {
			out = append(out, transformer)
		}
	}
	return out
}

func (p *Pipeline) readBody(resp *http.Response) ([]byte, string, error) {
	if resp.Body == nil {
		return nil, "body_unavailable", nil
	}
	if enc := strings.TrimSpace(resp.Header.Get("Content-Encoding")); enc != "" {
		return nil, "unsupported_content_encoding", nil
	}
	if resp.ContentLength > p.cfg.MaxBodySize {
		return nil, "body_too_large", nil
	}
	original := resp.Body
	limited := io.LimitReader(resp.Body, p.cfg.MaxBodySize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		_ = original.Close()
		return nil, "", fmt.Errorf("read_response_body: %w", err)
	}
	if int64(len(data)) > p.cfg.MaxBodySize {
		resp.Body = multiReadCloser{Reader: io.MultiReader(bytes.NewReader(data), original), Closer: original}
		return nil, "body_too_large", nil
	}
	_ = original.Close()
	return data, "", nil
}

func (p *Pipeline) restore(resp *http.Response, header http.Header, body []byte, bodyLoaded bool) {
	resp.Header = header.Clone()
	if bodyLoaded {
		setResponseBody(resp, body)
	}
}

func (m Match) matches(meta ResponseMeta) bool {
	if len(m.Providers) > 0 && !matchesStringList(m.Providers, meta.Provider) {
		return false
	}
	if len(m.Hosts) > 0 && !matchesHostList(m.Hosts, meta.Host) {
		return false
	}
	if len(m.PathPrefixes) > 0 && !matchesPrefixList(m.PathPrefixes, meta.Path) {
		return false
	}
	if len(m.ContentTypes) > 0 && !matchesStringList(m.ContentTypes, meta.ContentType) {
		return false
	}
	if len(m.StatusCodes) > 0 && !matchesStatusCode(m.StatusCodes, meta.StatusCode) {
		return false
	}
	return true
}

func (t Transform) hasBodyOps() bool {
	return t.InjectHead != "" || t.InjectBody != "" || len(t.Replace) > 0
}

func (t Transform) label() string {
	if strings.TrimSpace(t.Name) != "" {
		return t.Name
	}
	return "unnamed"
}

func transformsNeedBody(transforms []Transform) bool {
	for _, transform := range transforms {
		if transform.hasBodyOps() {
			return true
		}
	}
	return false
}

func transformersNeedBody(transformers []Transformer) bool {
	for _, transformer := range transformers {
		if transformer.NeedsBody() {
			return true
		}
	}
	return false
}

func transformerName(transformer Transformer) string {
	name := strings.TrimSpace(transformer.Name())
	if name == "" {
		return "unnamed"
	}
	return name
}

func eventFromMeta(action, transform, reason string, meta ResponseMeta) Event {
	return Event{
		Action:    action,
		Transform: transform,
		Reason:    reason,
		Provider:  meta.Provider,
		Host:      meta.Host,
		Path:      meta.Path,
		Status:    meta.StatusCode,
	}
}

func applyHeaders(header http.Header, transform Transform) bool {
	applied := false
	for _, key := range transform.HeaderRemove {
		if _, ok := header[key]; ok {
			applied = true
		}
		header.Del(key)
	}
	for key, value := range transform.HeaderSet {
		header.Set(key, value)
		applied = true
	}
	return applied
}

func applyBodyTransform(body []byte, transform Transform) ([]byte, bool, string, error) {
	applied := false
	var skipped []string
	next := body
	if transform.InjectHead != "" {
		updated, ok := insertBeforeFold(next, []byte("</head>"), []byte(transform.InjectHead))
		if !ok {
			skipped = append(skipped, "missing_head_end")
		} else {
			next = updated
			applied = true
		}
	}
	if transform.InjectBody != "" {
		updated, ok := insertBeforeFold(next, []byte("</body>"), []byte(transform.InjectBody))
		if !ok {
			skipped = append(skipped, "missing_body_end")
		} else {
			next = updated
			applied = true
		}
	}
	for _, repl := range transform.Replace {
		if repl.Old == "" {
			return nil, false, "", fmt.Errorf("replace old value is required")
		}
		count := repl.Count
		if count <= 0 {
			count = -1
		}
		replaced := []byte(strings.Replace(string(next), repl.Old, repl.New, count))
		if !bytes.Equal(next, replaced) {
			applied = true
			next = replaced
		} else {
			skipped = append(skipped, "replace_not_found")
		}
	}
	return next, applied, strings.Join(skipped, ","), nil
}

func insertBeforeFold(body, marker, insert []byte) ([]byte, bool) {
	idx := bytes.Index(bytes.ToLower(body), bytes.ToLower(marker))
	if idx < 0 {
		return body, false
	}
	out := make([]byte, 0, len(body)+len(insert))
	out = append(out, body[:idx]...)
	out = append(out, insert...)
	out = append(out, body[idx:]...)
	return out, true
}

func setResponseBody(resp *http.Response, body []byte) {
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
}

type multiReadCloser struct {
	io.Reader
	io.Closer
}

func responseContentType(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if idx := strings.Index(value, ";"); idx >= 0 {
		value = strings.TrimSpace(value[:idx])
	}
	return value
}

func matchesHostList(patterns []string, host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	for _, pattern := range patterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if strings.HasPrefix(pattern, "*.") {
			suffix := strings.TrimPrefix(pattern, "*.")
			if host == suffix || strings.HasSuffix(host, "."+suffix) {
				return true
			}
			continue
		}
		if host == pattern {
			return true
		}
	}
	return false
}

func matchesPrefixList(prefixes []string, path string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func matchesStringList(values []string, value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, candidate := range values {
		if strings.ToLower(strings.TrimSpace(candidate)) == value {
			return true
		}
	}
	return false
}

func matchesStatusCode(values []int, value int) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func replacementsFromConfig(values []config.PageEnhanceReplaceConfig) []Replacement {
	if len(values) == 0 {
		return nil
	}
	out := make([]Replacement, 0, len(values))
	for _, value := range values {
		out = append(out, Replacement{Old: value.Old, New: value.New, Count: value.Count})
	}
	return out
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func NormalizeHostForConfig(host string) (string, error) {
	if strings.HasPrefix(host, "*.") {
		normalized, err := rules.NormalizeHost(strings.TrimPrefix(host, "*."))
		if err != nil {
			return "", err
		}
		return "*." + normalized, nil
	}
	return rules.NormalizeHost(host)
}
