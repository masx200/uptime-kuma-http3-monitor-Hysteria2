package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	flag "flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

import (
	"github.com/quic-go/quic-go/http3"
)

// Configuration structures
type EndpointConfig struct {
	Name             string
	TargetURL        string
	SNI              string
	Host             string
	PushToken        string
	KumaURL          string
	Fingerprint      string
	ExpectedStatus   int
}

type Config struct {
	Endpoints       []EndpointConfig
	KumaURL         string
	Interval        time.Duration
	Timeout         time.Duration
	FingerprintOnly bool
}

// Check result structure
type CheckResult struct {
	Success             bool
	ResponseTime        time.Duration
	CertFingerprint     string
	ExpectedFingerprint string
	HTTPStatusCode      int
	ExpectedHTTPStatus  int
	ErrorMsg            string
}

// Uptime Kuma push response
type KumaPushResponse struct {
	OK  bool   `json:"ok"`
	Msg string `json:"msg,omitempty"`
}

// Global counters for statistics
var (
	checkCount   int64
	successCount int64
	failCount    int64
)

func main() {
	// Parse command-line flags
	config, err := parseFlags()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Check for fingerprint-only mode (backward compatibility)
	if config.FingerprintOnly || len(config.Endpoints) == 0 || (len(config.Endpoints) == 1 && config.Endpoints[0].PushToken == "") {
		if len(config.Endpoints) == 0 {
			log.Fatal("Error: --target flag is required")
		}
		if !config.FingerprintOnly && len(config.Endpoints) > 0 && config.Endpoints[0].PushToken == "" {
			log.Println("Warning: No --push-token provided. Running in fingerprint-only mode.")
			log.Println("         Use --fingerprint-only flag explicitly to silence this warning.")
		}
		runFingerprintOnly(config)
		return
	}

	// Start monitoring service
	logInfo("Starting HTTP/3 monitoring service")
	logInfo("Configuration: interval=%s, timeout=%s, endpoints=%d",
		config.Interval, config.Timeout, len(config.Endpoints))

	for i, ep := range config.Endpoints {
		ep.Name = fmt.Sprintf("endpoint-%d", i+1)
		ep.KumaURL = config.KumaURL
		config.Endpoints[i] = ep
		logInfo("Endpoint %d: %s (SNI: %s)", i+1, ep.TargetURL, ep.SNI)
	}

	startMonitoring(config)
}

// Parse command-line flags
func parseFlags() (*Config, error) {
	var targets, snis, hosts, pushTokens, fingerprints []string
	var expectedStatusList []int
	var kumaURL, intervalStr, timeoutStr string
	var fingerprintOnly bool

	flag.Func("target", "HTTP/3 endpoint URL (can be specified multiple times)", func(val string) error {
		targets = append(targets, val)
		return nil
	})
	flag.Func("sni", "TLS SNI server name (can be specified multiple times)", func(val string) error {
		snis = append(snis, val)
		return nil
	})
	flag.Func("host", "HTTP Host header (can be specified multiple times)", func(val string) error {
		hosts = append(hosts, val)
		return nil
	})
	flag.Func("push-token", "Uptime Kuma push token (can be specified multiple times)", func(val string) error {
		pushTokens = append(pushTokens, val)
		return nil
	})
	flag.Func("fingerprint", "Expected TLS certificate SHA256 fingerprint (must match exactly)", func(val string) error {
		fingerprints = append(fingerprints, val)
		return nil
	})
	flag.Func("expected-status", "Expected HTTP status code (e.g., 200, 204, etc.)", func(val string) error {
		var status int
		_, err := fmt.Sscanf(val, "%d", &status)
		if err != nil || status < 100 || status > 599 {
			return fmt.Errorf("invalid HTTP status code: %s", val)
		}
		expectedStatusList = append(expectedStatusList, status)
		return nil
	})

	flag.StringVar(&kumaURL, "kuma-url", "http://localhost:3001", "Uptime Kuma instance URL")
	flag.StringVar(&intervalStr, "interval", "60", "Monitoring interval in seconds")
	flag.StringVar(&timeoutStr, "timeout", "10", "HTTP/3 connection timeout in seconds")
	flag.BoolVar(&fingerprintOnly, "fingerprint-only", false, "Extract certificate fingerprint only and exit")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "HTTP/3 Monitoring Service for Uptime Kuma\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "\nExamples:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  # Single endpoint with all validations\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  %s --target https://example.com:443 --sni example.com --host example.com \\\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "     --fingerprint abc123... --expected-status 200 --push-token TOKEN123\n")
		fmt.Fprintf(flag.CommandLine.Output(), "\n  # Multiple endpoints\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  %s --target https://ep1.com:443 --sni ep1.com --host ep1.com --expected-status 200 --push-token TOKEN1 \\\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "     --target https://ep2.com:443 --sni ep2.com --host ep2.com --expected-status 204 --push-token TOKEN2\n")
		fmt.Fprintf(flag.CommandLine.Output(), "\n  # Fingerprint only (backward compatible)\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  %s --fingerprint-only --target https://example.com:443 --sni example.com\n", os.Args[0])
	}

	flag.Parse()

	if len(targets) == 0 && !fingerprintOnly {
		return nil, fmt.Errorf("--target flag is required")
	}

	// Validate URLs
	for i, target := range targets {
		if !strings.HasPrefix(target, "https://") {
			return nil, fmt.Errorf("target URL %d must use HTTPS scheme: %s", i+1, target)
		}
	}

	// Pair endpoints with their configuration
	endpoints := make([]EndpointConfig, len(targets))
	for i := 0; i < len(targets); i++ {
		endpoints[i].TargetURL = targets[i]
		if i < len(snis) {
			endpoints[i].SNI = snis[i]
		}
		if i < len(hosts) {
			endpoints[i].Host = hosts[i]
		}
		if i < len(fingerprints) {
			endpoints[i].Fingerprint = fingerprints[i]
		}
		if i < len(expectedStatusList) {
			endpoints[i].ExpectedStatus = expectedStatusList[i]
		} else {
			// Default to 200 if not specified
			endpoints[i].ExpectedStatus = 200
		}
		if i < len(pushTokens) {
			endpoints[i].PushToken = pushTokens[i]
		} else if len(pushTokens) > 0 {
			// Reuse last token if fewer tokens than targets
			endpoints[i].PushToken = pushTokens[len(pushTokens)-1]
			logWarn("Endpoint %d: Reusing push token (fewer tokens than targets)", i+1)
		}
	}

	// Parse durations
	interval, err := time.ParseDuration(intervalStr + "s")
	if err != nil {
		return nil, fmt.Errorf("invalid interval: %w", err)
	}

	timeout, err := time.ParseDuration(timeoutStr + "s")
	if err != nil {
		return nil, fmt.Errorf("invalid timeout: %w", err)
	}

	if interval < 10*time.Second {
		logWarn("Interval less than 10 seconds may overwhelm targets")
	}

	return &Config{
		Endpoints:       endpoints,
		KumaURL:         kumaURL,
		Interval:        interval,
		Timeout:         timeout,
		FingerprintOnly: fingerprintOnly,
	}, nil
}

// Run in fingerprint-only mode (backward compatible)
func runFingerprintOnly(config *Config) {
	if len(config.Endpoints) == 0 {
		log.Fatal("Error: --target is required")
	}

	if len(config.Endpoints) > 1 {
		log.Fatal("Error: Fingerprint mode only supports a single target")
	}

	endpoint := config.Endpoints[0]
	if endpoint.SNI == "" {
		log.Fatal("Error: --sni is required")
	}

	logInfo("Starting HTTP/3 connection test")
	logInfo("Target URL: %s", endpoint.TargetURL)
	logInfo("SNI: %s", endpoint.SNI)
	if endpoint.Host != "" {
		logInfo("Host header: %s", endpoint.Host)
	}
	if endpoint.Fingerprint != "" {
		logInfo("Expected fingerprint: %s", endpoint.Fingerprint)
	}
	if endpoint.ExpectedStatus > 0 {
		logInfo("Expected HTTP status: %d", endpoint.ExpectedStatus)
	}
	logInfo("Timeout: %s", config.Timeout)

	result, err := CheckHTTP3(endpoint.TargetURL, endpoint.SNI, endpoint.Host, endpoint.Fingerprint, endpoint.ExpectedStatus, config.Timeout)
	if err != nil {
		logError("Check failed: %v", err)
		log.Fatalf("连接失败: %s", result.ErrorMsg)
	}

	if !result.Success {
		logError("Connection failed: %s", result.ErrorMsg)
		log.Fatalf("连接失败: %s", result.ErrorMsg)
	}

	// Print result
	fmt.Println("\n========== 连接成功！==========")
	fmt.Printf("响应时间: %d ms\n", result.ResponseTime.Milliseconds())
	fmt.Printf("HTTP 状态码: %d\n", result.HTTPStatusCode)
	fmt.Printf("证书 SHA256 指纹: %s\n", result.CertFingerprint)

	// Validate fingerprint if provided
	if endpoint.Fingerprint != "" {
		fmt.Println("\n---------- 证书指纹验证 ----------")
		if result.ExpectedFingerprint == result.CertFingerprint {
			logInfo("Certificate fingerprint validation: PASSED")
			fmt.Println("证书指纹验证: 成功 ✓")
		} else {
			logError("Certificate fingerprint validation: FAILED")
			fmt.Printf("证书指纹验证: 失败 ✗\n")
			fmt.Printf("  期望: %s\n", endpoint.Fingerprint)
			fmt.Printf("  实际: %s\n", result.CertFingerprint)
			os.Exit(1)
		}
	}

	// Validate HTTP status code
	if result.ExpectedHTTPStatus > 0 {
		fmt.Println("\n---------- HTTP 状态码验证 ----------")
		if result.HTTPStatusCode == result.ExpectedHTTPStatus {
			logInfo("HTTP status code validation: PASSED (expected: %d, got: %d)", result.ExpectedHTTPStatus, result.HTTPStatusCode)
			fmt.Printf("HTTP 状态码验证: 成功 ✓ (期望: %d)\n", result.ExpectedHTTPStatus)
		} else {
			logError("HTTP status code validation: FAILED (expected: %d, got: %d)", result.ExpectedHTTPStatus, result.HTTPStatusCode)
			fmt.Printf("HTTP 状态码验证: 失败 ✗\n")
			fmt.Printf("  期望: %d\n", result.ExpectedHTTPStatus)
			fmt.Printf("  实际: %d\n", result.HTTPStatusCode)
			os.Exit(1)
		}
	}

	fmt.Println("\n================================")
	logInfo("All validations passed successfully!")
}

// Check HTTP/3 endpoint
func CheckHTTP3(target, sni, host, expectedFingerprint string, expectedStatus int, timeout time.Duration) (*CheckResult, error) {
	startTime := time.Now()

	logInfo("Initializing HTTP/3 connection to %s", target)
	logInfo("TLS Configuration:")
	logInfo("  - SNI: %s", sni)
	logInfo("  - InsecureSkipVerify: true")
	if host != "" {
		logInfo("  - Host header: %s", host)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Create TLS config
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         sni,
	}

	// Create HTTP/3 transport
	roundTripper := &http3.Transport{
		TLSClientConfig: tlsConfig,
	}

	// Create HTTP client
	client := &http.Client{
		Transport: roundTripper,
	}

	logInfo("Creating HTTP HEAD request...")

	// Create HEAD request
	req, err := http.NewRequestWithContext(ctx, "HEAD", target, nil)
	if err != nil {
		logError("Failed to create HTTP request: %v", err)
		return &CheckResult{
			Success:            false,
			ExpectedHTTPStatus: expectedStatus,
			ErrorMsg:           fmt.Sprintf("request creation failed: %v", err),
		}, err
	}

	// Set Host header if provided
	if host != "" {
		req.Host = host
		logInfo("Setting Host header: %s", host)
	}

	logInfo("Sending HTTP/3 request...")

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		logError("HTTP/3 request failed: %v", err)
		return &CheckResult{
			Success:            false,
			ExpectedHTTPStatus: expectedStatus,
			ErrorMsg:           fmt.Sprintf("connection failed: %v", err),
		}, err
	}
	defer resp.Body.Close()

	// Calculate response time
	responseTime := time.Since(startTime)
	logInfo("Response received in %d ms", responseTime.Milliseconds())

	// Get TLS state
	tlsState := resp.TLS
	if tlsState == nil {
		logError("Unable to retrieve TLS connection state")
		return &CheckResult{
			Success:            false,
			ExpectedHTTPStatus: expectedStatus,
			ErrorMsg:           "unable to get TLS connection state",
		}, fmt.Errorf("no TLS state")
	}

	logInfo("TLS connection established successfully")
	logInfo("TLS Version: %d", tlsState.Version)
	logInfo("TLS Cipher Suite: %x", tlsState.CipherSuite)

	// Get certificate
	if len(tlsState.PeerCertificates) == 0 {
		logError("Server provided no certificates")
		return &CheckResult{
			Success:            false,
			ExpectedHTTPStatus: expectedStatus,
			ErrorMsg:           "server provided no certificates",
		}, fmt.Errorf("no certificates")
	}

	serverCert := tlsState.PeerCertificates[0]
	logInfo("Certificate information:")
	logInfo("  - Subject: %s", serverCert.Subject)
	logInfo("  - Issuer: %s", serverCert.Issuer)
	logInfo("  - NotBefore: %s", serverCert.NotBefore.Format(time.RFC3339))
	logInfo("  - NotAfter: %s", serverCert.NotAfter.Format(time.RFC3339))

	// Calculate certificate fingerprint
	var fingerprint [32]byte = sha256.Sum256(serverCert.Raw)
	fingerprintStr := fmt.Sprintf("%x", fingerprint)

	logInfo("Certificate SHA256 fingerprint: %s", fingerprintStr)
	logInfo("HTTP Status Code: %d", resp.StatusCode)

	// Validate fingerprint if provided
	if expectedFingerprint != "" {
		logInfo("Validating certificate fingerprint...")
		// Normalize both fingerprints for comparison (remove any whitespace/colons)
		expectedNormalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(expectedFingerprint, ":", ""), " ", ""))
		actualNormalized := strings.ToLower(strings.ReplaceAll(fingerprintStr, " ", ""))

		if expectedNormalized != actualNormalized {
			logError("Certificate fingerprint mismatch!")
			logError("  Expected: %s", expectedFingerprint)
			logError("  Got: %s", fingerprintStr)
			return &CheckResult{
				Success:             false,
				ResponseTime:        responseTime,
				CertFingerprint:     fingerprintStr,
				ExpectedFingerprint: expectedFingerprint,
				HTTPStatusCode:      resp.StatusCode,
				ExpectedHTTPStatus:  expectedStatus,
				ErrorMsg:            fmt.Sprintf("certificate fingerprint mismatch: expected %s, got %s", expectedFingerprint, fingerprintStr),
			}, fmt.Errorf("fingerprint mismatch")
		}
		logInfo("Certificate fingerprint validation: PASSED")
	}

	// Validate HTTP status code if expected status is set
	if expectedStatus > 0 {
		logInfo("Validating HTTP status code...")
		if resp.StatusCode != expectedStatus {
			logError("HTTP status code mismatch!")
			logError("  Expected: %d", expectedStatus)
			logError("  Got: %d", resp.StatusCode)
			return &CheckResult{
				Success:             false,
				ResponseTime:        responseTime,
				CertFingerprint:     fingerprintStr,
				ExpectedFingerprint: expectedFingerprint,
				HTTPStatusCode:      resp.StatusCode,
				ExpectedHTTPStatus:  expectedStatus,
				ErrorMsg:            fmt.Sprintf("HTTP status code mismatch: expected %d, got %d", expectedStatus, resp.StatusCode),
			}, fmt.Errorf("status code mismatch")
		}
		logInfo("HTTP status code validation: PASSED (expected %d, got %d)", expectedStatus, resp.StatusCode)
	}

	logInfo("Connection test completed successfully")

	return &CheckResult{
		Success:             true,
		ResponseTime:        responseTime,
		CertFingerprint:     fingerprintStr,
		ExpectedFingerprint: expectedFingerprint,
		HTTPStatusCode:      resp.StatusCode,
		ExpectedHTTPStatus:  expectedStatus,
		ErrorMsg:            "OK",
	}, nil
}

// Push status to Uptime Kuma
func PushStatus(kumaURL, pushToken string, result *CheckResult, endpointName string) error {
	// Build push URL
	pushURL := kumaURL + "/api/push/" + pushToken

	// Build query parameters
	params := url.Values{}
	if result.Success {
		params.Add("status", "up")
		params.Add("ping", fmt.Sprintf("%.0f", float64(result.ResponseTime.Milliseconds())))
		params.Add("msg", "OK")
	} else {
		params.Add("status", "down")
		// Truncate error message if too long
		msg := result.ErrorMsg
		if len(msg) > 250 {
			msg = msg[:247] + "..."
		}
		params.Add("msg", msg)
	}

	fullURL := pushURL + "?" + params.Encode()
	logInfo("Push URL: %s?status=%s&ping=%s&msg=%s",
		pushURL,
		params.Get("status"),
		params.Get("ping"),
		params.Get("msg"))

	// Create HTTP client with 5-second timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Execute push request
	resp, err := client.Get(fullURL)
	if err != nil {
		logError("HTTP request to Uptime Kuma failed: %v", err)
		return fmt.Errorf("push request failed: %w", err)
	}
	defer resp.Body.Close()

	logInfo("Uptime Kuma response status: %d", resp.StatusCode)

	// Parse response
	var kumaResp KumaPushResponse
	if err := json.NewDecoder(resp.Body).Decode(&kumaResp); err != nil {
		logError("Failed to parse Uptime Kuma response: %v", err)
		return fmt.Errorf("failed to parse response: %w", err)
	}

	logInfo("Uptime Kuma response: ok=%v, msg=%s", kumaResp.OK, kumaResp.Msg)

	if resp.StatusCode == 404 {
		logError("Push token not found or monitor not active (404)")
		return fmt.Errorf("push token not found or monitor not active")
	}

	if resp.StatusCode >= 500 && resp.StatusCode < 600 {
		// Server error, will retry
		logWarn("Uptime Kuma server error: %d", resp.StatusCode)
		return fmt.Errorf("server error: %d", resp.StatusCode)
	}

	if !kumaResp.OK {
		logError("Uptime Kuma rejected push: %s", kumaResp.Msg)
		return fmt.Errorf("push rejected: %s", kumaResp.Msg)
	}

	return nil
}

// Start monitoring service
func startMonitoring(config *Config) {
	// Create stop channel
	stopCh := make(chan struct{})

	// WaitGroup for goroutines
	var wg sync.WaitGroup

	// Launch goroutine for each endpoint
	for _, endpoint := range config.Endpoints {
		wg.Add(1)
		go func(ep EndpointConfig) {
			defer wg.Done()
			monitorEndpoint(ep, config.Interval, config.Timeout, stopCh)
		}(endpoint)
	}

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	// Wait for signal
	<-sigCh
	logInfo("Shutdown signal received, stopping monitors...")

	// Close stop channel to signal all goroutines to stop
	close(stopCh)

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logInfo("All monitors stopped gracefully")
	case <-time.After(30 * time.Second):
		logWarn("Shutdown timeout exceeded, forcing exit")
	}

	// Print statistics
	logInfo("Final statistics: total=%d, success=%d, failed=%d",
		atomic.LoadInt64(&checkCount),
		atomic.LoadInt64(&successCount),
		atomic.LoadInt64(&failCount))
}

// Monitor single endpoint
func monitorEndpoint(endpoint EndpointConfig, interval, timeout time.Duration, stopCh chan struct{}) {
	defer func() {
		if r := recover(); r != nil {
			logError("endpoint=%s panic recovered: %v", endpoint.Name, r)
		}
	}()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logInfo("endpoint=%s Starting monitor", endpoint.Name)

	// Perform first check immediately
	checkAndPush(endpoint, timeout)

	for {
		select {
		case <-stopCh:
			logInfo("endpoint=%s Stopping monitor", endpoint.Name)
			return
		case <-ticker.C:
			checkAndPush(endpoint, timeout)
		}
	}
}

// Check and push status
func checkAndPush(endpoint EndpointConfig, timeout time.Duration) {
	atomic.AddInt64(&checkCount, 1)

	logInfo("---------- Starting check for %s ----------", endpoint.Name)
	logInfo("Configuration:")
	logInfo("  - Target: %s", endpoint.TargetURL)
	logInfo("  - SNI: %s", endpoint.SNI)
	if endpoint.Host != "" {
		logInfo("  - Host: %s", endpoint.Host)
	}
	if endpoint.Fingerprint != "" {
		logInfo("  - Expected fingerprint: %s", endpoint.Fingerprint)
	}
	if endpoint.ExpectedStatus > 0 {
		logInfo("  - Expected status: %d", endpoint.ExpectedStatus)
	}

	result, err := CheckHTTP3(endpoint.TargetURL, endpoint.SNI, endpoint.Host, endpoint.Fingerprint, endpoint.ExpectedStatus, timeout)

	if err != nil && !result.Success {
		// Check failed
		atomic.AddInt64(&failCount, 1)
		logError("Check FAILED for %s", endpoint.Name)
		logError("Error: %s", result.ErrorMsg)
		logError("Total checks: %d, Success: %d, Failed: %d",
			atomic.LoadInt64(&checkCount),
			atomic.LoadInt64(&successCount),
			atomic.LoadInt64(&failCount))
	} else if result.Success {
		atomic.AddInt64(&successCount, 1)
		logInfo("Check PASSED for %s", endpoint.Name)
		logInfo("Response time: %d ms", result.ResponseTime.Milliseconds())
		logInfo("HTTP status: %d", result.HTTPStatusCode)
		logInfo("Certificate fingerprint: %s", result.CertFingerprint)
		logInfo("Total checks: %d, Success: %d, Failed: %d",
			atomic.LoadInt64(&checkCount),
			atomic.LoadInt64(&successCount),
			atomic.LoadInt64(&failCount))
	}

	// Push to Uptime Kuma (with retry)
	logInfo("Pushing status to Uptime Kuma...")
	logInfo("  - Kuma URL: %s", endpoint.KumaURL)
	logInfo("  - Status: %s", map[bool]string{true: "up", false: "down"}[result.Success])
	if result.Success {
		logInfo("  - Ping: %d ms", result.ResponseTime.Milliseconds())
	} else {
		logInfo("  - Message: %s", result.ErrorMsg)
	}

	pushErr := PushStatus(endpoint.KumaURL, endpoint.PushToken, result, endpoint.Name)
	if pushErr != nil {
		if strings.Contains(pushErr.Error(), "server error") {
			// Retry once on 5xx errors
			logWarn("Push failed (server error), retrying in 1 second...")
			time.Sleep(1 * time.Second)
			pushErr = PushStatus(endpoint.KumaURL, endpoint.PushToken, result, endpoint.Name)
			if pushErr != nil {
				logError("Push retry failed: %v", pushErr)
			} else {
				logInfo("Push succeeded on retry")
			}
		} else if strings.Contains(pushErr.Error(), "not found") {
			logError("Push failed: %s", pushErr)
			logError("Please check:")
			logError("  1. Push token is correct")
			logError("  2. Monitor is active in Uptime Kuma")
		} else {
			logWarn("Push failed: %v", pushErr)
		}
	} else {
		logInfo("Status pushed to Uptime Kuma successfully")
	}

	logInfo("---------- Check completed for %s ----------\n", endpoint.Name)
}

// Logging functions
func logInfo(format string, args ...interface{}) {
	log.Printf("[INFO] "+format, args...)
}

func logWarn(format string, args ...interface{}) {
	log.Printf("[WARN] "+format, args...)
}

func logError(format string, args ...interface{}) {
	log.Printf("[ERROR] "+format, args...)
}
