package http

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"io"
	"math"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/god-hunter/god-hunter/internal/log"
)

type JitterProfile struct {
	MinMs    int
	MaxMs    int
	Enable   bool
	ProxyURL string
}

type Client struct {
	httpClient *http.Client
	jitter     *JitterProfile
	userAgents []string
	requestID  uint64
	logger     *log.Logger
}

func NewClient(jitterRange string, proxyURL string) (*Client, error) {
	return NewClientWithLogger(jitterRange, proxyURL, log.NewLogger(true))
}

// NewClientWithLogger creates a client with a custom logger
func NewClientWithLogger(jitterRange string, proxyURL string, logger *log.Logger) (*Client, error) {
	c := &Client{
		jitter: &JitterProfile{
			MinMs:    800,
			MaxMs:    2400,
			Enable:   true,
			ProxyURL: proxyURL,
		},
		userAgents: []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
		},
		logger: logger,
	}

	// Parse jitter range
	if jitterRange != "" {
		parts := strings.Split(jitterRange, "-")
		if len(parts) == 2 {
			minMs, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
			maxMs, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
			if minMs >= 0 && maxMs > minMs {
				c.jitter.MinMs = minMs
				c.jitter.MaxMs = maxMs
			} else {
				logger.Warn("invalid jitter range, using defaults: %s", jitterRange)
			}
		}
	}

	// Configure HTTP transport with proper connection pooling
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       10,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		DisableKeepAlives:     false,
	}

	// Set proxy if provided
	if proxyURL != "" {
		parsedProxyURL, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}
		transport.Proxy = http.ProxyURL(parsedProxyURL)
		logger.Info("configured proxy: %s", proxyURL)
	}

	c.httpClient = &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	return c, nil
}

func (c *Client) applyJitter() {
	if !c.jitter.Enable || c.jitter.MaxMs <= c.jitter.MinMs {
		return
	}
	jitterRange := int64(c.jitter.MaxMs - c.jitter.MinMs)
	randBig, err := rand.Int(rand.Reader, big.NewInt(jitterRange))
	if err != nil {
		c.logger.Warn("failed to generate jitter: %v", err)
		return
	}
	jitterMs := c.jitter.MinMs + int(randBig.Int64())
	time.Sleep(time.Duration(jitterMs) * time.Millisecond)
}

func (c *Client) getRandomUserAgent() string {
	if len(c.userAgents) == 0 {
		return "Mozilla/5.0"
	}
	randBig, err := rand.Int(rand.Reader, big.NewInt(int64(len(c.userAgents))))
	if err != nil {
		c.logger.Warn("failed to select user agent: %v", err)
		return c.userAgents[0]
	}
	return c.userAgents[randBig.Int64()]
}

// RequestMetadata holds request and response details
type RequestMetadata struct {
	Method           string
	URL              string
	Headers          map[string]string
	Body             string
	ResponseStatus   int
	ResponseHeaders  map[string]string
	ResponseBody     string
	ResponseBodySize int64
	ContentType      string
	TimingMs         int64
	RequestID        string
	Retries          int
}

// Do performs an HTTP request with jitter, rate limiting, and human-like behavior
func (c *Client) Do(method, urlStr string, headers map[string]string, body string) (*RequestMetadata, error) {
	return c.DoWithContext(context.Background(), method, urlStr, headers, body)
}

// DoWithContext performs an HTTP request with context support
func (c *Client) DoWithContext(ctx context.Context, method, urlStr string, headers map[string]string, body string) (*RequestMetadata, error) {
	c.applyJitter()

	req, err := http.NewRequestWithContext(ctx, method, urlStr, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers with human-like defaults
	req.Header.Set("User-Agent", c.getRandomUserAgent())
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	// Override with custom headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Record request timing
	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body with size limit
	respBody := ""
	contentType := resp.Header.Get("Content-Type")
	bodySize := int64(0)

	if resp.Body != nil {
		limitedReader := io.LimitedReader{R: resp.Body, N: 10 * 1024 * 1024} // 10MB limit
		buf, err := io.ReadAll(&limitedReader)
		if err != nil && err != io.EOF {
			c.logger.Warn("error reading response body: %v", err)
		}
		respBody = string(buf)
		bodySize = int64(len(buf))
	}

	// Serialize response headers
	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		respHeaders[k] = strings.Join(v, ", ")
	}

	c.requestID++
	return &RequestMetadata{
		Method:           method,
		URL:              urlStr,
		Headers:          headers,
		Body:             body,
		ResponseStatus:   resp.StatusCode,
		ResponseHeaders:  respHeaders,
		ResponseBody:     respBody,
		ResponseBodySize: bodySize,
		ContentType:      contentType,
		TimingMs:         duration.Milliseconds(),
		RequestID:        fmt.Sprintf("req_%d", c.requestID),
		Retries:          0,
	}, nil
}

// CalculateDeviation calculates timing deviation for anomaly detection
func (c *Client) CalculateDeviation(timings []int64) float64 {
	if len(timings) < 2 {
		return 0
	}

	// Calculate mean
	var sum int64
	for _, t := range timings {
		sum += t
	}
	mean := float64(sum) / float64(len(timings))

	// Calculate standard deviation
	var sumSquares float64
	for _, t := range timings {
		diff := float64(t) - mean
		sumSquares += diff * diff
	}
	variance := sumSquares / float64(len(timings))
	return math.Sqrt(variance)
}

// Close gracefully closes the HTTP client and underlying connections
func (c *Client) Close() error {
	if c.httpClient != nil && c.httpClient.Transport != nil {
		if transport, ok := c.httpClient.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}
	c.logger.Debug("HTTP client connections closed")
	return nil
}

// IsJSON checks if content type is JSON
func IsJSON(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "application/json")
}

// IsHTML checks if content type is HTML
func IsHTML(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml")
}

// IsXML checks if content type is XML
func IsXML(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "application/xml") || strings.Contains(ct, "text/xml")
}
