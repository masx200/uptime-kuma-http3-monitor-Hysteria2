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
	Name      string
	TargetURL string
	SNI       string
	PushToken string
	KumaURL   string
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
	Success         bool
	ResponseTime    time.Duration
	CertFingerprint string
	ErrorMsg        string
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
	var targets, snis, pushTokens []string
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
	flag.Func("push-token", "Uptime Kuma push token (can be specified multiple times)", func(val string) error {
		pushTokens = append(pushTokens, val)
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
		fmt.Fprintf(flag.CommandLine.Output(), "  # Single endpoint\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  %s --target https://example.com:443 --sni example.com --push-token TOKEN123\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "\n  # Multiple endpoints\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  %s --target https://ep1.com:443 --sni ep1.com --push-token TOKEN1 \\\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "     --target https://ep2.com:443 --sni ep2.com --push-token TOKEN2\n")
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

	result, err := CheckHTTP3(endpoint.TargetURL, endpoint.SNI, config.Timeout)
	if err != nil {
		log.Fatalf("Check failed: %v", err)
	}

	if !result.Success {
		log.Fatalf("Connection failed: %s", result.ErrorMsg)
	}

	// Print in original format
	fmt.Println("连接成功！")
	fmt.Printf("服务器证书通用名称: %s\n", result.CertFingerprint)
	fmt.Printf("证书 SHA256 指纹: %x\n", result.CertFingerprint)
}

// Check HTTP/3 endpoint
func CheckHTTP3(target, sni string, timeout time.Duration) (*CheckResult, error) {
	startTime := time.Now()

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

	// Create HEAD request
	req, err := http.NewRequestWithContext(ctx, "HEAD", target, nil)
	if err != nil {
		return &CheckResult{
			Success:  false,
			ErrorMsg: fmt.Sprintf("request creation failed: %v", err),
		}, err
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return &CheckResult{
			Success:  false,
			ErrorMsg: fmt.Sprintf("connection failed: %v", err),
		}, err
	}
	defer resp.Body.Close()

	// Calculate response time
	responseTime := time.Since(startTime)

	// Get TLS state
	tlsState := resp.TLS
	if tlsState == nil {
		return &CheckResult{
			Success:  false,
			ErrorMsg: "unable to get TLS connection state",
		}, fmt.Errorf("no TLS state")
	}

	// Get certificate
	if len(tlsState.PeerCertificates) == 0 {
		return &CheckResult{
			Success:  false,
			ErrorMsg: "server provided no certificates",
		}, fmt.Errorf("no certificates")
	}

	serverCert := tlsState.PeerCertificates[0]

	// Calculate certificate fingerprint
	var fingerprint [32]byte = sha256.Sum256(serverCert.Raw)

	return &CheckResult{
		Success:         true,
		ResponseTime:    responseTime,
		CertFingerprint: fmt.Sprintf("%x", fingerprint),
		ErrorMsg:        "OK",
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

	// Create HTTP client with 5-second timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Execute push request
	resp, err := client.Get(fullURL)
	if err != nil {
		return fmt.Errorf("push request failed: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var kumaResp KumaPushResponse
	if err := json.NewDecoder(resp.Body).Decode(&kumaResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if resp.StatusCode == 404 {
		return fmt.Errorf("push token not found or monitor not active")
	}

	if resp.StatusCode >= 500 && resp.StatusCode < 600 {
		// Server error, will retry
		return fmt.Errorf("server error: %d", resp.StatusCode)
	}

	if !kumaResp.OK {
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

	result, err := CheckHTTP3(endpoint.TargetURL, endpoint.SNI, timeout)

	if err != nil && !result.Success {
		// Check failed
		atomic.AddInt64(&failCount, 1)
		logError("endpoint=%s status=down msg=\"%s\"", endpoint.Name, result.ErrorMsg)
	} else if result.Success {
		atomic.AddInt64(&successCount, 1)
		logInfo("endpoint=%s status=up ping=%dms msg=OK",
			endpoint.Name, result.ResponseTime.Milliseconds())
	}

	// Push to Uptime Kuma (with retry)
	pushErr := PushStatus(endpoint.KumaURL, endpoint.PushToken, result, endpoint.Name)
	if pushErr != nil {
		if strings.Contains(pushErr.Error(), "server error") {
			// Retry once on 5xx errors
			logWarn("endpoint=%s push failed (server error), retrying...", endpoint.Name)
			time.Sleep(1 * time.Second)
			pushErr = PushStatus(endpoint.KumaURL, endpoint.PushToken, result, endpoint.Name)
			if pushErr != nil {
				logWarn("endpoint=%s push retry failed: %v", endpoint.Name, pushErr)
			} else {
				logInfo("endpoint=%s push succeeded on retry", endpoint.Name)
			}
		} else if strings.Contains(pushErr.Error(), "not found") {
			logError("endpoint=%s push failed: %s. Check push token and monitor activation.",
				endpoint.Name, pushErr)
		} else {
			logWarn("endpoint=%s push failed: %v", endpoint.Name, pushErr)
		}
	}
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
