package worker

import (
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	cm "observerPolite/common"
	"strconv"
	"strings"
	"time"

	"github.com/temoto/robotstxt"
)

type DedicatedWorker struct {
	Domain        string
	WorkerTasks   chan cm.Task
	AllResultsRef *chan cm.Task
	robotHTTP     *robotstxt.Group
	robotHTTPS    *robotstxt.Group
}

type DedicatedWorkerInterface interface {
	Start()
	HandleTask(task cm.Task)
}

func (dw *DedicatedWorker) HandleTask(task cm.Task) {
	// fmt.Println("work on task: ", task.Domain)

	// Add results to chan: TODO: add to DB instead
	// Auto HTTPS retry
	defer func() {
		if (task.Resp == nil || task.Resp.StatusCode != 200) && strings.HasPrefix(task.URL, "http://") {
			task.URL = strings.Replace(task.URL, "http://", "https://", 1)
			task.AutoRetryHTTPS = true
			dw.HandleTask(task)
		} else {
			*dw.AllResultsRef <- task
		}
	}()

	parsedURL, _ := url.Parse(task.URL)

	// Robot Check (only if robot.txt is found)
	if parsedURL.Scheme == "http" {
		if dw.robotHTTP != nil && !dw.robotHTTP.Test(parsedURL.Path) {
			task.Err = fmt.Errorf("path %s not allowd for http", parsedURL.Path)
			return
		}
	} else if parsedURL.Scheme == "https" {
		if dw.robotHTTPS != nil && !dw.robotHTTPS.Test(parsedURL.Path) {
			task.Err = fmt.Errorf("path %s not allowd for https", parsedURL.Path)
			return
		}
	} else {
		task.Err = fmt.Errorf("WRONG Scheme! (not http or https)")
		return
	}

	// DNS Lookup
	ips, err := net.LookupIP(parsedURL.Hostname())
	if err != nil {
		task.Err = fmt.Errorf("DNS error: %w", err)
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
	conn, err := net.Dial("tcp", net.JoinHostPort(task.IP, port))
	if err != nil {
		task.Err = fmt.Errorf("TCP error: %w", err)
		return
	}

	var resp *http.Response

	// TLS Handshake
	if parsedURL.Scheme == "https" {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: parsedURL.Hostname()})
		err := tlsConn.Handshake()
		if err != nil {
			task.Err = fmt.Errorf("TLS handshake error: %w", err)
			return
		}
	}

	// Issue a request
	req, err := http.NewRequest("GET", task.URL, nil)
	if err != nil {
		task.Err = fmt.Errorf("request creation error: %w", err)
		return
	}
	req.Header.Set("User-Agent", "Web Measure/1.0 (https://webresearch.eecs.umich.edu/overview-of-web-measurements/) Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/79.0.3945.130 Safari/537.36")

	// Redirect Chain
	client := http.Client{
		Timeout: time.Second * 30,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			task.RedirectChain = append(task.RedirectChain, strconv.Itoa(req.Response.StatusCode)+" "+req.URL.String())
			return nil
		},
	}

	// Send Request
	resp, err = client.Do(req)
	if err != nil {
		task.Err = fmt.Errorf("HTTPS request error: %w", err)
		return
	}
	defer resp.Body.Close()

	task.Resp = resp
	task.Err = err
}

func (dw *DedicatedWorker) FetchRobot(scheme string) error {
	robotsURL := url.URL{
		Scheme: scheme,
		Host:   dw.Domain,
		Path:   "/robots.txt",
	}

	resp, err := http.Get(robotsURL.String())
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
	crawlDelay := cm.GlobalConfig.ExpectedRuntime / time.Duration(len(dw.WorkerTasks))

	// Bring in some Randomness
	randSleep := time.Duration(rand.Int63n(int64(crawlDelay)))
	time.Sleep(randSleep)

	// Fetch robot.txt
	err := dw.FetchRobot("http")
	if err != nil {
		fmt.Println("robots.txt for %w (http) is not found", dw.Domain)
	}
	err = dw.FetchRobot("https")
	if err != nil {
		fmt.Println("robots.txt for %w (https) is not found", dw.Domain)
	}

	for task := range dw.WorkerTasks {
		dw.HandleTask(task)
		time.Sleep(crawlDelay) // sleep periodially
	}
}
