package extensions

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// HTTPBridge handles network requests initiated by JS extensions.
type HTTPBridge struct {
	client *http.Client
}

// NewHTTPBridge creates a new HTTP client bridge with custom timeouts.
func NewHTTPBridge() *HTTPBridge {
	return &HTTPBridge{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Request performs a general HTTP request and returns the body string.
func (h *HTTPBridge) Request(method, url string, body string, headers map[string]string) (string, error) {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return "", err
	}

	// Set headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(respBody), nil
}

// DOMNode emulates a DOM element node inside the Goja JavaScript VM.
type DOMNode struct {
	selection *goquery.Selection
}

// Text extracts the inner text contents of the node.
func (n *DOMNode) Text() string {
	if n.selection == nil {
		return ""
	}
	return strings.TrimSpace(n.selection.Text())
}

// Attr retrieves the value of a specific attribute, returning empty if not found.
func (n *DOMNode) Attr(name string) string {
	if n.selection == nil {
		return ""
	}
	val, _ := n.selection.Attr(name)
	return val
}

// QuerySelector retrieves the first child node matching the CSS selector.
func (n *DOMNode) QuerySelector(selector string) *DOMNode {
	if n.selection == nil {
		return &DOMNode{selection: nil}
	}
	return &DOMNode{selection: n.selection.Find(selector).First()}
}

// QuerySelectorAll retrieves a slice of all matching child nodes.
func (n *DOMNode) QuerySelectorAll(selector string) []*DOMNode {
	var list []*DOMNode
	if n.selection == nil {
		return list
	}

	n.selection.Find(selector).Each(func(i int, s *goquery.Selection) {
		list = append(list, &DOMNode{selection: s})
	})
	return list
}

// ParseHTML parses raw HTML text and returns a root DOMNode selection node.
func ParseHTML(html string) (*DOMNode, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewBufferString(html))
	if err != nil {
		return nil, err
	}
	return &DOMNode{selection: doc.Selection}, nil
}
