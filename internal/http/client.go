package http

import (
	"crypto/tls"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
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
}

func NewClient(jitterRange string, proxyURL string) (*Client, error) {
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
	}

	// Parse jitter range
	if jitterRange != "" {
		parts := strings.Split(jitterRange, "-")
		if len(parts) == 2 {
			minMs, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
			maxMs, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
			if minMs > 0 && maxMs > minMs {
				c.jitter.MinMs = minMs
				c.jitter.MaxMs = maxMs
			}
		}
	}

	// Configure HTTP transport
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	// Set proxy if provided
	if proxyURL != "" {
		proxyFunc := func(req *http.Request) (*url.URL, error) {
			return url.Parse(proxyURL)
		}
		transport.Proxy = proxyFunc
	}

	c.httpClient = &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	return c, nil
}

func (c *Client) applyJitter() {
	if !c.jitter.Enable {
		return
	}
	jitterMs := rand.Intn(c.jitter.MaxMs-c.jitter.MinMs) + c.jitter.MinMs
	time.Sleep(time.Duration(jitterMs) * time.Millisecond)
}

func (c *Client) getRandomUserAgent() string {
	return c.userAgents[rand.Intn(len(c.userAgents))]
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
	TimingMs         int64
	RequestID        string
}

// Do performs an HTTP request with jitter, rate limiting, and human-like behavior
func (c *Client) Do(method, url string, headers map[string]string, body string) (*RequestMetadata, error) {
	c.applyJitter()

	req, err := http.NewRequest(method, url, strings.NewReader(body))
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

	// Read response body
	respBody := ""
	if resp.Body != nil {
		buf := make([]byte, 1024*1024) // 1MB limit
		n, _ := resp.Body.Read(buf)
		respBody = string(buf[:n])
	}

	// Serialize response headers
	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		respHeaders[k] = strings.Join(v, ", ")
	}

	c.requestID++
	return &RequestMetadata{
		Method:          method,
		URL:             url,
		Headers:         headers,
		Body:            body,
		ResponseStatus:  resp.StatusCode,
		ResponseHeaders: respHeaders,
		ResponseBody:    respBody,
		TimingMs:        duration.Milliseconds(),
		RequestID:       fmt.Sprintf("req_%d", c.requestID),
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
