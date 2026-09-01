// Package tagger calls the external tagging service that turns an idea's text
// into a list of suggested tags.
package tagger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrUnavailable means the tagging service could not be reached, answered with
// an error, or replied with something this client cannot read. Callers map it
// onto a 502 rather than leaking the upstream's own status or body.
var ErrUnavailable = errors.New("tagging service unavailable")

// MaxTextBytes caps what is sent upstream. Idea bodies are small, and a cap
// keeps one oversized note from turning into a slow, expensive request.
const MaxTextBytes = 8 << 10 // 8 KiB

// maxResponseBytes caps what is read back, so a misbehaving or wrong endpoint
// cannot stream an unbounded body into memory.
const maxResponseBytes = 64 << 10

// Client calls the tagging service. The zero value is not usable; use New.
type Client struct {
	url  string
	http *http.Client
}

// New returns a client for the endpoint at url. A blank url yields nil, which
// is the disabled state: callers check Enabled before offering the feature.
func New(url string, timeout time.Duration) *Client {
	if url == "" {
		return nil
	}
	return &Client{url: url, http: &http.Client{Timeout: timeout}}
}

// Enabled reports whether a tagging service is configured. It is nil-safe so
// callers can hold a possibly-nil client without guarding every use.
func (c *Client) Enabled() bool { return c != nil }

// request and response mirror the service's wire format.
type request struct {
	Text string `json:"text"`
}

type response struct {
	Tags []string `json:"tags"`
}

// Tag sends text to the service and returns the tags it suggests, in the order
// given. The returned names are raw: normalising and de-duplicating them is the
// caller's job, since that is the same rule the rest of the API applies.
//
// Every failure wraps ErrUnavailable, so an outage upstream is never reported
// as a fault in the idea being tagged. The underlying cause is wrapped
// alongside it rather than flattened into text, so a caller can still match on
// it — context.DeadlineExceeded, say — and the log keeps the detail.
func (c *Client) Tag(ctx context.Context, text string) ([]string, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("%w: no tagging service configured", ErrUnavailable)
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	if len(text) > MaxTextBytes {
		text = truncateBytes(text, MaxTextBytes)
	}

	payload, err := json.Marshal(request{Text: text})
	if err != nil {
		return nil, fmt.Errorf("%w: encode request: %w", ErrUnavailable, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("%w: build request: %w", ErrUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: service returned status %d", ErrUnavailable, res.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("%w: read response: %w", ErrUnavailable, err)
	}

	var decoded response
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("%w: response is not the expected JSON", ErrUnavailable)
	}
	return decoded.Tags, nil
}

// truncateBytes cuts text to at most limit bytes without splitting a rune, so
// the service never receives a mangled final character.
func truncateBytes(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	cut := limit
	for cut > 0 && !isRuneStart(text[cut]) {
		cut--
	}
	return text[:cut]
}

// isRuneStart reports whether b begins a UTF-8 rune, i.e. is not a
// continuation byte (10xxxxxx).
func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }
