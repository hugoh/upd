package pkg

// Package probes implements network connectivity probes for various protocols.
//
// Supported protocols:
//   - HTTP/HTTPS
//   - TCP
//   - DNS
//
// Basic usage:
//
//	probe := probes.NewHTTPProbe("https://example.com")
//	report := probe.Probe(ctx, time.Second)
//	if report.IsError() {
//		fmt.Println("Connection failed:", report.Error())
//	} else {
//		fmt.Println("Connection successful, response:", report.Response())
//	}
//
// Checks and Probes:
//
// The package provides both Check and Probe types. A Check combines a Probe
// with a timeout, while a Probe is just the connection test logic.
//
// Example - Check List:
//
//	checks := &pkg.CheckList{
//		Ordered: []*pkg.Check{
//			{
//				Probe:   pkg.NewHTTPProbe("https://example.com"),
//				Timeout: 10 * time.Second,
//			},
//		},
//	}
//
// Checker Interface:
//
// The Checker interface allows custom handling of check lifecycle events:
//   - CheckRun called before probe execution
//   - ProbeSuccess called on successful probe
//   - ProbeFailure called on failed probe
//
// Example - Custom Checker:
//
//	type LoggingChecker struct{}
//
//	func (l *LoggingChecker) CheckRun(c pkg.Check) {
//		fmt.Printf("Running check: %s\n", c.Probe)
//	}
//
//	func (l *LoggingChecker) ProbeSuccess(report *pkg.Report) {
//		fmt.Printf("Success: %s, elapsed: %v\n", report.Response(), report.Elapsed())
//	}
//
//	func (l *LoggingChecker) ProbeFailure(report *pkg.Report) {
//		fmt.Printf("Failed: %s, error: %v\n", report.Protocol(), report.Error())
//	}
//
// Report Structure:
//
// Each probe returns a Report containing:
//   - Protocol: The protocol used (http, tcp, dns)
//   - Response: The response from the probe (e.g., HTTP status code)
//   - Elapsed: Time taken to complete the probe
//   - Error: Any error that occurred during the probe
//
// Thread Safety:
//
// Probes are thread-safe and can be used concurrently from multiple goroutines.
// Each Probe maintains its own state where necessary.
//
// Context Usage:
//
// All Probe methods require a context parameter. The context is used for:
//   - Timeout control
//   - Cancellation
//   - Request-scoped values
//
// Example - With Context:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//	report := probe.Probe(ctx, 2*time.Second)
//
// Error Handling:
//
// Errors returned by Probe methods are wrapped and preserve the original error.
// Use errors.Is or errors.As to check for specific error types.
//
//	probe := pkg.NewDNSProbe("8.8.8.8:53", "example.com")
//	report := probe.Probe(ctx, time.Second)
//	if report.error != nil {
//		if errors.Is(report.error, context.DeadlineExceeded) {
//			fmt.Println("Probe timed out")
//		}
//	}
