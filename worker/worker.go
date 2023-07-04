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
)

type DedicatedWorker struct {
	Domain        string
	WorkerTasks   chan cm.Task
	AllResultsRef *chan cm.Task
}

type DedicatedWorkerInterface interface {
	Start()
	HandleTask(task cm.Task)
}

func (dw *DedicatedWorker) HandleTask(task cm.Task) {
	// fmt.Println("work on task: ", task.Domain)

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
		// TLS handshake
		tlsConn := tls.Client(conn, &tls.Config{ServerName: parsedURL.Hostname()})
		err := tlsConn.Handshake()
		if err != nil {
			task.Err = fmt.Errorf("TLS handshake error: %w", err)
			return
		}
	}

	req, err := http.NewRequest("GET", task.URL, nil)
	if err != nil {
		task.Err = fmt.Errorf("Request creation error: %w", err)
		return
	}

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
	resp, err = client.Do(req)
	if err != nil {
		task.Err = fmt.Errorf("HTTPS request error: %w", err)
		return
	}
	defer resp.Body.Close()

	task.Resp = resp
	task.Err = err
}

func (dw *DedicatedWorker) Start() {
	crawlDelay := cm.GlobalConfig.ExpectedRuntime / time.Duration(len(dw.WorkerTasks))

	randSleep := time.Duration(rand.Int63n(int64(crawlDelay)))
	time.Sleep(randSleep)

	for task := range dw.WorkerTasks {
		dw.HandleTask(task)
		time.Sleep(crawlDelay)
	}
}
