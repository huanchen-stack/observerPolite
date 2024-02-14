package worker

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	cm "observerPolite/common"
	"strconv"
	"time"
)

// DNSLookUp can utilize multiple DNS servers initialized in the common package
//
//	A global round-robin implementation of DNSServer requires heavy usage of one mutex, which can be a potential runtime bottleneck.
//	This function uses randomness to approximate an even usage.
//	Error message is thrown to caller! Caller is responsible to handle/throw the errors!
func (wk *Worker) DNSLookUp(hostname string, IPOnly bool) (cm.DNSRecord, error) {
	dialer := &net.Dialer{
		Timeout: time.Second * 10,
	}

	var dnsServer string
	var port string
	if cm.GlobalConfig.DNSdist {
		dnsServer = "127.0.0.1"
		port = cm.GlobalConfig.DNSdistPort
	} else {
		secureRandomIndex := cm.GetRandomIndex(len(cm.DNSServers))
		dnsServer = cm.DNSServers[secureRandomIndex]
		port = ":53"
	}

	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialer.DialContext(ctx, "udp", dnsServer+port)
		},
	}

	dnsRecord := cm.DNSRecord{
		Hostname: hostname,
	}

	ips, err := resolver.LookupIP(context.Background(), "ip", hostname)
	if err != nil {
		err := fmt.Errorf("DNS error: %w", err)
		return dnsRecord, err
	}
	for _, ip := range ips {
		if ipv4 := ip.To4(); ipv4 != nil {
			if dnsRecord.IP == "" {
				dnsRecord.IP = ipv4.String()
			}
			if IPOnly {
				return dnsRecord, nil
			}
			dnsRecord.IPs = append(dnsRecord.IPs, ipv4.String())
		}
		if ipv6 := ip.To16(); !IPOnly && ipv6 != nil {
			dnsRecord.IPs = append(dnsRecord.IPs, ipv6.String())
		}
	}

	mxRecords, err := resolver.LookupMX(context.Background(), hostname)
	for _, mx := range mxRecords {
		dnsRecord.MXs = append(dnsRecord.MXs, mx.Host+" "+strconv.Itoa(int(mx.Pref)))
	}

	nsRecords, err := resolver.LookupNS(context.Background(), hostname)
	for _, ns := range nsRecords {
		dnsRecord.NSs = append(dnsRecord.NSs, ns.Host)
	}

	txtRecords, err := resolver.LookupTXT(context.Background(), hostname)
	for i, _ := range txtRecords {
		dnsRecord.TXTs = append(dnsRecord.TXTs, txtRecords[i])
	}

	return dnsRecord, nil
}

func (wk *Worker) getPort(scheme string) string {
	if scheme == "http" {
		return "80"
	} else {
		return "443"
	}
}

func (wk *Worker) TCPConnect(IP string, port string, timeoutMult int) (*net.Conn, error) {
	conn, err := net.DialTimeout(
		"tcp",
		net.JoinHostPort(IP, port),
		time.Duration(int64(cm.GlobalConfig.Timeout)*int64(timeoutMult)),
	)
	if err != nil {
		err := fmt.Errorf("TCP error: %w", err)
		return nil, err
	}
	return &conn, nil
}

func (wk *Worker) TLSConnect(conn net.Conn, hostname string, timeoutMult int) (*net.Conn, error) {
	tlsConn := tls.Client(conn, &tls.Config{ServerName: hostname})
	tlsConn.SetDeadline(time.Now().Add(
		time.Duration(int64(cm.GlobalConfig.Timeout) * int64(timeoutMult)),
	))
	err := tlsConn.Handshake()
	if err != nil {
		err := fmt.Errorf("TLS handshake error: %w", err)
		return nil, err
	}
	var tlsConn_ net.Conn = tlsConn
	return &tlsConn_, nil
}

// TransportLayerOT is a wrapper that consists TCP and TLS handshake
//
//	OT stands for ONE TIME, meaning the returned transport object can be only used ONCE!
//	Error message is collected and thrown to caller! Caller is responsible to handle/throw the errors!
func (wk *Worker) TransportLayerOT(taskIP string, scheme string, hostname string, timeoutMult int) (*http.Transport, error) {
	// TCP Connection
	conn, err := wk.TCPConnect(taskIP, wk.getPort(scheme), timeoutMult)
	if err != nil {
		return nil, err
	}

	// (secure) transport layer
	transport := &http.Transport{
		DisableKeepAlives: true,
	}
	var tlsConn *net.Conn
	// VERY IMPORTANT TO USE DIALCONTEXT FOR HTTP AND DIALTLSCONTEXT FOR HTTPS
	if scheme == "http" {
		transport.DialContext = func(_ context.Context, network, addr string) (net.Conn, error) {
			return *conn, nil
		}
	} else {
		tlsConn, err = wk.TLSConnect(*conn, hostname, timeoutMult)
		if err != nil {
			return nil, err
		}
		transport.DialTLSContext = func(_ context.Context, network, addr string) (net.Conn, error) {
			return *tlsConn, nil
		}
	}

	return transport, nil
}

// MakeClient uses the One Time http.transport object to create http.client object.
//
//	Upon redirection, MakeClient creates new One Time http.transport objects.
//	Error message is collected and thrown to caller! Caller is responsible to handle/throw the errors!
func (wk *Worker) MakeClient(scheme string, hostname string, redirectChain *[]cm.Redirect, dnsRecords *[]cm.DNSRecord) (*http.Client, error) {
	ipOnly := false
	for i, _ := range *dnsRecords {
		if (*dnsRecords)[i].Hostname == hostname && len((*dnsRecords)[i].IPs) != 0 {
			ipOnly = true
		}
	}
	dnsRecord, err := wk.DNSLookUp(hostname, ipOnly)
	if err != nil {
		return nil, err
	}
	*dnsRecords = append(*dnsRecords, dnsRecord)
	taskIP := dnsRecord.IP

	// Auto retry upon transport layer errors (conn err)
	timeoutMult := 1
	transport, err := wk.TransportLayerOT(taskIP, scheme, hostname, timeoutMult)
	for err != nil {
		if timeoutMult > cm.GlobalConfig.Retries {
			return nil, err
		}
		timeoutMult++
		transport, err = wk.TransportLayerOT(taskIP, scheme, hostname, timeoutMult)
	}

	var client *http.Client
	client = &http.Client{
		Transport: transport,
		Timeout:   time.Duration(int64(cm.GlobalConfig.Timeout) * int64(timeoutMult)),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}

			if hostname != req.URL.Hostname() {
				hostname = req.URL.Hostname()
				ipOnly = false
				for i, _ := range *dnsRecords {
					if (*dnsRecords)[i].Hostname == hostname && len((*dnsRecords)[i].IPs) != 0 {
						ipOnly = true
					}
				}
				dnsRecord, err = wk.DNSLookUp(hostname, ipOnly)
				if err != nil {
					*redirectChain = append(
						*redirectChain,
						cm.Redirect{
							Statuscode: 0,
							LocationIP: "",
							Location:   req.URL.String(),
						},
					)
					return err
				}
				*dnsRecords = append(*dnsRecords, dnsRecord)
				taskIP = dnsRecord.IP
			}
			var newTransport *http.Transport
			newTransport, err = wk.TransportLayerOT(taskIP, req.URL.Scheme, req.URL.Hostname(), timeoutMult)
			if err != nil {
				*redirectChain = append(
					*redirectChain,
					cm.Redirect{
						Statuscode: 0,
						Location:   req.URL.String(),
						LocationIP: taskIP,
					},
				)
				return err
			}
			client.Transport = newTransport

			*redirectChain = append(
				*redirectChain,
				cm.Redirect{
					Statuscode: req.Response.StatusCode,
					Location:   req.URL.String(),
					LocationIP: taskIP,
				},
			)
			return nil
		},
		Jar: nil,
	}

	return client, nil
}

func (wk *Worker) MakeRequest(urlStr string) *http.Request {
	req, _ := http.NewRequest("GET", urlStr, nil)
	req.Header.Set("User-Agent", cm.GlobalConfig.UserAgent)
	req.Header.Set("Cache-Control", "no-cache")

	return req
}

func (wk *Worker) HTTPGET(parsedURL *url.URL, redirectChain *[]cm.Redirect, dnsRecords *[]cm.DNSRecord) (*http.Response, error) {
	if redirectChain == nil {
		redirectChainTmp := make([]cm.Redirect, 0)
		redirectChain = &redirectChainTmp
	}
	if dnsRecords == nil {
		dnsRecordsTmp := make([]cm.DNSRecord, 0)
		dnsRecords = &dnsRecordsTmp
	}

	client, err := wk.MakeClient(parsedURL.Scheme, parsedURL.Hostname(), redirectChain, dnsRecords)
	if err != nil {
		return nil, err
	}
	request := wk.MakeRequest(parsedURL.String())
	response, err := client.Do(request)
	if err != nil {
		//panic(err)
		err = fmt.Errorf("HTTP request error: %w", err)
		return nil, err
	}
	return response, nil
}
