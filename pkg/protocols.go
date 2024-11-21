// Initially from: https://github.com/jesusprubio/up @ 784898b4b4e72ccb80b520c0dfbe8ebbc72b87fe
// Copyright Jesús Rubio <jesusprubio@gmail.com>
// MIT License

package pkg

const (
	DNS   string = "dns"
	HTTP  string = "http"
	HTTPS string = "https"
	TCP   string = "tcp"
)

// //nolint:gochecknoglobals
// var protocols = map[string]Protocol{
// 	DNS:  &DNSProtocol{},
// 	HTTP: &HTTPProtocol{},
// 	TCP:  &TCPProtocol{},
// }

// type Protocol interface {
// 	Type() string
// 	// Probe(args *ProtocolArgs, timeout time.Duration) *Report
// }

// type DNSProtocol struct{}

// type HTTPProtocol struct{}

// type TCPProtocol struct{}

// func ProtocolByScheme(scheme string) (*Protocol, bool) {
// 	if scheme == HTTPS {
// 		scheme = HTTP
// 	}
// 	p, ok := protocols[scheme]
// 	if !ok {
// 		return nil, ok
// 	}
// 	return &p, ok
// }

// func (p *DNSProtocol) Type() string {
// 	return DNS
// }

// func (p *HTTPProtocol) Type() string {
// 	return HTTP
// }

// func (p *TCPProtocol) Type() string {
// 	return TCP
// }

// // func (p *HTTPProtocol) Probe(args *ProtocolArgs, timeout time.Duration) *Report {
// func HTTPProtoProbe(url string, timeout time.Duration) *Report {
// 	cli := &http.Client{Timeout: timeout} //nolint:exhaustruct
// 	start := time.Now()
// 	resp, err := cli.Get(url) //nolint:noctx
// 	report := BuildReport(HTTP, start)
// 	if err != nil {
// 		report.Error = fmt.Errorf("making request to %s: %w", url, err)
// 		return report
// 	}
// 	err = resp.Body.Close()
// 	if err != nil {
// 		report.Error = fmt.Errorf("closing response body: %w", err)
// 		return report
// 	}
// 	report.Response = resp.Status
// 	return report
// }

// // func (p *TCPProtocol) Probe(args *ProtocolArgs, timeout time.Duration) *Report {
// func TCPProtoProbe(hostPort string, timeout time.Duration) *Report {
// 	start := time.Now()
// 	conn, err := net.DialTimeout("tcp", hostPort, timeout)
// 	report := BuildReport(TCP, start)
// 	if err != nil {
// 		report.Error = fmt.Errorf("making request to %s: %w", hostPort, err)
// 		return report
// 	}
// 	err = conn.Close()
// 	if err != nil {
// 		report.Error = fmt.Errorf("closing connection: %w", err)
// 		return report
// 	}
// 	report.Response = conn.LocalAddr().String()
// 	return report
// }

// // func (p *DNSProtocol) Probe(args *ProtocolArgs, timeout time.Duration) *Report {
// func DNSProtoProbe(dnsResolver string, domain string, timeout time.Duration) *Report {
// 	r := &net.Resolver{ //nolint:exhaustruct
// 		PreferGo: true,
// 		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
// 			d := net.Dialer{ //nolint:exhaustruct
// 				Timeout: timeout,
// 			}
// 			return d.DialContext(ctx, network, dnsResolver)
// 		},
// 	}
// 	start := time.Now()
// 	addr, err := r.LookupHost(context.Background(), domain)
// 	report := BuildReport(DNS, start)
// 	if err != nil {
// 		report.Error = fmt.Errorf("error resolving %s: %w", domain, err)
// 		return report
// 	}
// 	report.Response = fmt.Sprintf("%s @ %s", addr[0], dnsResolver)
// 	return report
// }
