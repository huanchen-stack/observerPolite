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
	"sync"
	"time"

	"github.com/temoto/robotstxt"
)

func FetchRobot(scheme string, hostname string) *robotstxt.Group {
	robotsURL := url.URL{
		Scheme: scheme,
		Host:   hostname,
		Path:   "/robots.txt",
	}
	client := http.Client{Timeout: cm.GlobalConfig.Timeout * 2}
	resp, err := client.Get(robotsURL.String())
	if err != nil {
		return &robotstxt.Group{}
	}
	defer resp.Body.Close()

	robots, err := robotstxt.FromResponse(resp)
	if err != nil {
		return &robotstxt.Group{}
	}

	return robots.FindGroup("*")
}

type GeneralWorker struct {
	WorkerTasks   chan cm.Task
	AllResultsRef *chan cm.Task // those tasks can be copied into channels
	robotstxt     map[string]map[string]*robotstxt.Group
}

type GeneralWorkerInterface interface {
	Start()
	HandleTask(task cm.Task)
}

func (gw *GeneralWorker) ErrorLog(task *cm.Task, err *error) {
	if (*task).Retry == nil {
		(*task).Err = *err
	} else if (*task).Retry != nil && (*task).Retry.Retried {
		(*task).Retry.Err = *err
	} else {
		panic("error log is not working properly")
	}
}

func (gw *GeneralWorker) RespLog(task *cm.Task, resp *http.Response) {
	if (*task).Retry == nil {
		(*task).Resp = resp
	} else if (*task).Retry != nil && (*task).Retry.Retried {
		(*task).Retry.Resp = resp
	} else {
		panic("resp log is not working properly")
	}
}

func (gw *GeneralWorker) HandleTask(task cm.Task, wg *sync.WaitGroup) {

	defer func() {
		*gw.AllResultsRef <- task
		(*wg).Done()
	}()

	parsedURL, _ := url.Parse(task.URL)
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		err := fmt.Errorf("WRONG Scheme! (not http or https)")
		gw.ErrorLog(&task, &err)
		return
	}

	// robot.txt check
	if _, ok := gw.robotstxt[parsedURL.Scheme][parsedURL.Hostname()]; !ok {
		gw.robotstxt[parsedURL.Scheme][parsedURL.Hostname()] = FetchRobot(
			parsedURL.Scheme, parsedURL.Hostname())
	}
	if !gw.robotstxt[parsedURL.Scheme][parsedURL.Hostname()].Test(parsedURL.Path) {
		err := fmt.Errorf("path %s not allowd for http", parsedURL.Path)
		gw.ErrorLog(&task, &err)
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
		gw.ErrorLog(&task, &err)
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
		gw.ErrorLog(&task, &err)
		return
	}

	// TLS Handshake
	if parsedURL.Scheme == "https" {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: parsedURL.Hostname()})
		tlsConn.SetDeadline(time.Now().Add(cm.GlobalConfig.Timeout))
		err := tlsConn.Handshake()
		if err != nil {
			err := fmt.Errorf("TLS handshake error: %w", err)
			gw.ErrorLog(&task, &err)
			return
		}
	}

	// Issue a request
	req, err := http.NewRequest("GET", task.URL, nil)
	if err != nil {
		err := fmt.Errorf("request creation error: %w", err)
		gw.ErrorLog(&task, &err)
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
		gw.ErrorLog(&task, &err)
		return
	}
	defer resp.Body.Close()

	gw.RespLog(&task, resp)
}

func (gw *GeneralWorker) Start() {
	gw.robotstxt = make(map[string]map[string]*robotstxt.Group)
	gw.robotstxt["http"] = make(map[string]*robotstxt.Group)
	gw.robotstxt["https"] = make(map[string]*robotstxt.Group)

	var wg sync.WaitGroup
	start := time.Now()
	for task := range gw.WorkerTasks {
		time.Sleep(task.Schedule - time.Since(start))
		//fmt.Printf("Time: %.2f | Initiating task %s (scheduled: %.2f)\n",
		//	time.Since(start).Seconds(), task.URL, task.Schedule.Seconds())
		wg.Add(1)
		go gw.HandleTask(task, &wg)
	}
	wg.Wait()
}
