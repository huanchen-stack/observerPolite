package worker

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	cm "observerPolite/common"
	"strconv"
	"sync"
	"time"

	"github.com/temoto/robotstxt"
)

type DedicatedWorker struct {
	Domain        string
	WorkerTasks   chan cm.Task
	AllResultsRef *chan cm.Task
	WG            *sync.WaitGroup
	robotHTTP     *robotstxt.Group
	robotHTTPS    *robotstxt.Group
}

type DedicatedWorkerInterface interface {
	Start()
	HandleTask(task cm.Task)
}

func (dw *DedicatedWorker) ErrorLog(task *cm.Task, err *error) {
	if (*task).Retry == nil {
		(*task).Err = *err
	} else if (*task).Retry != nil && (*task).Retry.Retried {
		(*task).Retry.Err = *err
	} else {
		panic("error log is not working properly")
	}
}

func (dw *DedicatedWorker) RespLog(task *cm.Task, resp *http.Response) {
	if (*task).Retry == nil {
		(*task).Resp = resp
	} else if (*task).Retry != nil && (*task).Retry.Retried {
		(*task).Retry.Resp = resp
	} else {
		panic("resp log is not working properly")
	}
}

func (dw *DedicatedWorker) HandleTask(task cm.Task) {

	defer func() {
		*dw.AllResultsRef <- task
	}()

	parsedURL, _ := url.Parse(task.URL)

	// Robot Check (only if robot.txt is found)
	if parsedURL.Scheme == "http" {
		if dw.robotHTTP != nil && !dw.robotHTTP.Test(parsedURL.Path) {
			err := fmt.Errorf("path %s not allowd for http", parsedURL.Path)
			dw.ErrorLog(&task, &err)
			return
		}
	} else if parsedURL.Scheme == "https" {
		if dw.robotHTTPS != nil && !dw.robotHTTPS.Test(parsedURL.Path) {
			err := fmt.Errorf("path %s not allowd for https", parsedURL.Path)
			dw.ErrorLog(&task, &err)
			return
		}
	} else {
		err := fmt.Errorf("WRONG Scheme! (not http or https)")
		dw.ErrorLog(&task, &err)
		return
	}

	// DNS Lookup
	dialer := &net.Dialer{
		Timeout: time.Second * 5, // 5-second timeout
	}
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialer.DialContext(ctx, "udp", "8.8.8.8:53")
		},
	}
	ips, err := resolver.LookupIP(context.Background(), "ip", parsedURL.Hostname())
	if err != nil {
		err := fmt.Errorf("DNS error: %w", err)
		dw.ErrorLog(&task, &err)
		return
	}
	for _, ip := range ips {
		if ipv4 := ip.To4(); ipv4 != nil {
			task.IP = ipv4.String()
			break
		}
	}

	// TCP Connection
	port := parsedURL.Port()
	if port == "" {
		if parsedURL.Scheme == "https" {
			port = "443"
		} else if parsedURL.Scheme == "http" {
			port = "80"
		}
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(task.IP, port), cm.GlobalConfig.Timeout)
	if err != nil {
		err := fmt.Errorf("TCP error: %w", err)
		dw.ErrorLog(&task, &err)
		return
	}

	// TLS Handshake
	if parsedURL.Scheme == "https" {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: parsedURL.Hostname()})
		tlsConn.SetDeadline(time.Now().Add(cm.GlobalConfig.Timeout))
		err := tlsConn.Handshake()
		if err != nil {
			err := fmt.Errorf("TLS handshake error: %w", err)
			dw.ErrorLog(&task, &err)
			return
		}
	}

	// Issue a request
	req, err := http.NewRequest("GET", task.URL, nil)
	if err != nil {
		err := fmt.Errorf("request creation error: %w", err)
		dw.ErrorLog(&task, &err)
		return
	}
	req.Header.Set("User-Agent", "Web Measure/1.0 (https://webresearch.eecs.umich.edu/overview-of-web-measurements/) Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/79.0.3945.130 Safari/537.36")
	req.Header.Set("Cache-Control", "no-cache")

	// Redirect Chain
	var redirectChain *[]string
	if task.Retry == nil {
		redirectChain = &task.RedirectChain
	} else {
		redirectChain = &task.Retry.RedirectChain
	}
	*redirectChain = []string{}
	client := http.Client{
		Timeout: cm.GlobalConfig.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			*redirectChain = append(*redirectChain, strconv.Itoa(req.Response.StatusCode)+" "+req.URL.String())
			return nil
		},
	}

	// Send Request
	resp, err := client.Do(req)
	if err != nil {
		err := fmt.Errorf("HTTPS request error: %w", err)
		dw.ErrorLog(&task, &err)
		return
	}
	defer resp.Body.Close()

	dw.RespLog(&task, resp)
}

func (dw *DedicatedWorker) FetchRobot(scheme string) error {
	robotsURL := url.URL{
		Scheme: scheme,
		Host:   dw.Domain,
		Path:   "/robots.txt",
	}
	client := http.Client{Timeout: cm.GlobalConfig.Timeout * 2}
	resp, err := client.Get(robotsURL.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	robots, err := robotstxt.FromResponse(resp)
	if err != nil {
		return err
	}

	if scheme == "http" {
		dw.robotHTTP = robots.FindGroup("*")
	} else {
		dw.robotHTTPS = robots.FindGroup("*")
	}

	return nil
}

func (dw *DedicatedWorker) Start() {

	defer dw.WG.Done()

	crawlDelay := cm.GlobalConfig.ExpectedRuntime / time.Duration(len(dw.WorkerTasks))
	fmt.Printf("Hostname: %s  #TaskStrs: %d  Crawl-Delay: %s\n", dw.Domain, len(dw.WorkerTasks), crawlDelay.String())

	// Bring in some Randomness
	randSleep := time.Duration(rand.Int63n(int64(crawlDelay)))
	time.Sleep(randSleep)

	// Fetch robot.txt
	err := dw.FetchRobot("http")
	if err != nil {
		// TODO: do something
		fmt.Printf("robots.txt for %w (http) is not found\n", dw.Domain)
	}
	err = dw.FetchRobot("https")
	if err != nil {
		// TODO: do something
		fmt.Printf("robots.txt for %w (https) is not found\n", dw.Domain)
	}

	firstScan := true
	for task := range dw.WorkerTasks {
		if !firstScan {
			time.Sleep(crawlDelay)
		} else {
			firstScan = false
		}
		dw.HandleTask(task)
	}
}
