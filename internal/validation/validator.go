package validation

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidateURL validates and normalizes a URL string
func ValidateURL(urlStr string) (string, error) {
	if urlStr == "" {
		return "", fmt.Errorf("URL cannot be empty")
	}

	// Trim whitespace
	urlStr = strings.TrimSpace(urlStr)

	// Ensure URL has scheme
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		urlStr = "https://" + urlStr
	}

	// Parse and validate
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("invalid URL format: %w", err)
	}

	// Validate host
	if parsedURL.Host == "" {
		return "", fmt.Errorf("URL must contain a valid host")
	}

	// Validate scheme
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", fmt.Errorf("URL scheme must be http or https, got: %s", parsedURL.Scheme)
	}

	return parsedURL.String(), nil
}

// ValidateSessionName validates a session name
func ValidateSessionName(name string) error {
	if name == "" {
		return fmt.Errorf("session name cannot be empty")
	}

	if len(name) > 255 {
		return fmt.Errorf("session name too long (max 255 characters)")
	}

	// Allow alphanumeric, hyphens, underscores, dots
	for _, ch := range name {
		if !((ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') ||
			ch == '-' || ch == '_' || ch == '.') {
			return fmt.Errorf("invalid character in session name: %c", ch)
		}
	}

	return nil
}

// ValidateHTTPMethod validates HTTP method
func ValidateHTTPMethod(method string) error {
	validMethods := map[string]bool{
		"GET":     true,
		"POST":    true,
		"PUT":     true,
		"DELETE":  true,
		"PATCH":   true,
		"HEAD":    true,
		"OPTIONS": true,
		"TRACE":   true,
	}

	if !validMethods[strings.ToUpper(method)] {
		return fmt.Errorf("unsupported HTTP method: %s", method)
	}

	return nil
}

// ValidateProxy validates a proxy URL
func ValidateProxy(proxyStr string) (string, error) {
	if proxyStr == "" {
		return "", nil
	}

	// Ensure proxy has scheme
	if !strings.HasPrefix(proxyStr, "http://") && !strings.HasPrefix(proxyStr, "https://") {
		proxyStr = "http://" + proxyStr
	}

	parsedURL, err := url.Parse(proxyStr)
	if err != nil {
		return "", fmt.Errorf("invalid proxy URL: %w", err)
	}

	if parsedURL.Host == "" {
		return "", fmt.Errorf("proxy URL must contain a valid host")
	}

	return proxyStr, nil
}

// ValidateJitterRange validates jitter range string (format: "min-max")
func ValidateJitterRange(jitterRange string) error {
	if jitterRange == "" {
		return nil // Empty is valid (uses defaults)
	}

	parts := strings.Split(jitterRange, "-")
	if len(parts) != 2 {
		return fmt.Errorf("jitter range format must be 'min-max', got: %s", jitterRange)
	}

	var minMs, maxMs int
	_, err := fmt.Sscanf(parts[0], "%d", &minMs)
	if err != nil {
		return fmt.Errorf("invalid minimum jitter value: %s", parts[0])
	}

	_, err = fmt.Sscanf(parts[1], "%d", &maxMs)
	if err != nil {
		return fmt.Errorf("invalid maximum jitter value: %s", parts[1])
	}

	if minMs < 0 {
		return fmt.Errorf("minimum jitter must be >= 0, got: %d", minMs)
	}

	if maxMs <= minMs {
		return fmt.Errorf("maximum jitter must be > minimum, got min=%d, max=%d", minMs, maxMs)
	}

	if maxMs > 300000 { // 5 minutes max
		return fmt.Errorf("maximum jitter cannot exceed 300000ms (5 minutes), got: %d", maxMs)
	}

	return nil
}
