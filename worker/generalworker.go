package worker

import (
	"container/heap"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/temoto/robotstxt"
	"io"
	"net"
	"net/http"
	"net/url"
	cm "observerPolite/common"
	db "observerPolite/mongodb"
	"strconv"
	"strings"
	"sync"
	"time"
)

type GeneralWorker struct {
	WorkerTasksHeap cm.HeapSlice       // for first try
	WorkerTasksStrs chan string        // for retry
	AllResultsRef   *chan cm.TaskPrint // those tasks can be copied into channels
	RBConn          *db.RobotsDBConn
	bypassRobots    bool
}

type GeneralWorkerInterface interface {
	Start()
	HandleTask(task cm.TaskPrint)
}

// ErrorLog writes to either task.err section or task.retry.err section, depending on if this is a retry
//
//	same as RespLog
func (gw *GeneralWorker) ErrorLog(task *cm.TaskPrint, err error) {
	if task.Retry.Retried {
		task.Retry.Err = cm.PrintErr(err)
	} else {
		task.Err = cm.PrintErr(err)
	}
}

func (gw *GeneralWorker) RespLog(task *cm.TaskPrint, resp *http.Response) {
	if task.Retry.Retried {
		task.Retry.Resp = cm.PrintResp(resp)
	} else {
		task.Resp = cm.PrintResp(resp)
	}
}

func (gw *GeneralWorker) DstChangeLog(task *cm.TaskPrint) {
	if task.Retry.Retried {
		task.Retry.DstChange = cm.PrintDstChange(task.URL, task.Retry.RedirectChain)
	} else {
		task.DstChange = cm.PrintDstChange(task.URL, task.RedirectChain)
	}
}

// DNSLookUp can utilize multiple DNS servers initialized in the common package
//
//	A global round-robin implementation of DNSServer requires heavy usage of one mutex, which can be a potential runtime bottleneck.
//	This function uses randomness to approximate an even usage.
//	Error message is thrown to caller! Caller is responsible to handle/throw the errors!
func DNSLookUp(hostname string) (string, error) {
	dialer := &net.Dialer{
		Timeout: time.Second * 10,
	}
	secureRandomIndex := cm.GetRandomIndex(len(cm.DNSServers))
	dnsServer := cm.DNSServers[secureRandomIndex]
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialer.DialContext(ctx, "udp", dnsServer+":53")
		},
	}
	ips, err := resolver.LookupIP(context.Background(), "ip", hostname)
	if err != nil {
		err := fmt.Errorf("DNS error: %w", err)
		//gw.ErrorLog(&task, &err)
		return "", err
	}
	var IP string
	for _, ip := range ips {
		if ipv4 := ip.To4(); ipv4 != nil {
			IP = ipv4.String()
			break
		}
	}
	return IP, nil
}

func getPort(parsedURL *url.URL) string {
	port := parsedURL.Port()
	if port == "" {
		if parsedURL.Scheme == "https" {
			port = "443"
		} else if parsedURL.Scheme == "http" {
			port = "80"
		}
	}
	return port
}

func TCPConnect(IP string, port string, timeoutMult int) (*net.Conn, error) {
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

func TLSConnect(conn net.Conn, hostname string, timeoutMult int) (*net.Conn, error) {
	tlsConn := tls.Client(conn, &tls.Config{ServerName: hostname})
	tlsConn.SetDeadline(time.Now().Add(
		time.Duration(int64(cm.GlobalConfig.Timeout) * int64(timeoutMult)),
	))
	err := tlsConn.Handshake()
	if err != nil {
		err := fmt.Errorf("TLS handshake error: %w", err)
		return nil, err
	}
	var tlsConn_ net.Conn
	tlsConn_ = tlsConn
	return &tlsConn_, nil
}

// TransportLayerOT handles everything before (exclude) the application layer (include TLS)
//
//	OT stands for ONE TIME, meaning the returned transport object can be only used ONCE!
//	Error message is collected and thrown to caller! Caller is responsible to handle/throw the errors!
func TransportLayerOT(parsedURL *url.URL, taskIP *string, timeoutMult int) (*http.Transport, error) {
	// DNS Lookup
	if *taskIP == "" {
		IP, err := DNSLookUp(parsedURL.Hostname())
		if err != nil {
			return nil, err
		}
		*taskIP = IP
	}

	// TCP Connection
	// TODO: how to reuse those connections when redirecting???
	conn, err := TCPConnect(*taskIP, getPort(parsedURL), timeoutMult)
	if err != nil {
		return nil, err
	}

	// (secure) transport layer
	transport := &http.Transport{
		DisableKeepAlives: true,
	}
	var tlsConn *net.Conn
	// VERY IMPORTANT TO USE DIALCONTEXT FOR HTTP AND DIALTLSCONTEXT FOR HTTPS
	if parsedURL.Scheme == "http" {
		transport.DialContext = func(_ context.Context, network, addr string) (net.Conn, error) {
			return *conn, nil
		}
	} else {
		tlsConn, err = TLSConnect(*conn, parsedURL.Hostname(), timeoutMult)
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
func MakeClient(parsedURL *url.URL, redirectChain *[]string, taskIP *string) (*http.Client, error) {

	// Auto retry upon transport layer errors (conn err)
	timeoutMult := 1
	transport, err := TransportLayerOT(parsedURL, taskIP, timeoutMult)
	for err != nil {
		if timeoutMult > cm.GlobalConfig.Retries {
			return nil, err
		}
		timeoutMult++
		transport, err = TransportLayerOT(parsedURL, taskIP, timeoutMult)
	}

	var client *http.Client
	client = &http.Client{
		Transport: transport,
		Timeout:   time.Duration(int64(cm.GlobalConfig.Timeout) * int64(timeoutMult)),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}

			// use prev timeoutMult for the same task
			//		(ideally for the same hostname with a timeout on this knowledge)
			newTransport, err := TransportLayerOT(req.URL, taskIP, timeoutMult)
			if err != nil {
				return err
			}
			client.Transport = newTransport

			*redirectChain = append(
				*redirectChain,
				strconv.Itoa(req.Response.StatusCode)+" "+req.URL.String(),
			)
			return nil
		},
		Jar: nil,
	}

	return client, nil
}

func MakeRequest(parsedURL *url.URL) (*http.Request, error) {
	req, err := http.NewRequest("GET", parsedURL.String(), nil)
	if err != nil {
		err := fmt.Errorf("request creation error: %w", err)
		return nil, err
	}
	req.Header.Set("User-Agent", "Web Measure/1.0 (https://webresearch.eecs.umich.edu/overview-of-web-measurements/) Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/79.0.3945.130 Safari/537.36")
	req.Header.Set("Cache-Control", "no-cache")
	return req, nil
}

// HandleTask is the CORE function of this scanner.
//
//	(url sanity) -> check robotstxt cache/DB -> make client
//	IF NEEDED -> (make robots request -> client.Do<req> -> robots db logging -> make new client ) ->
//	-> make task request -> client.Do<req> -> resp/err logging
func (gw *GeneralWorker) HandleTask(task cm.TaskPrint, wg *sync.WaitGroup) {
	var client *http.Client
	var req *http.Request
	var resp *http.Response
	var err error

	defer func() {
		gw.RespLog(&task, resp)
		gw.ErrorLog(&task, err)
		*gw.AllResultsRef <- task
		(*wg).Done()
	}()

	// excluded hostnames
	parsedURL, _ := url.Parse(task.URL)
	if _, ok := cm.ExcludedList[parsedURL.Hostname()]; ok {
		err = fmt.Errorf("Excluded Hostname!")
		return
	}

	// scheme sanity check
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		err = fmt.Errorf("WRONG Scheme! (not http or https)")
		return
	}

	// robot.txt check
	robotsCached := false
	if !gw.bypassRobots {
		group := gw.RBConn.Get(parsedURL.Scheme, parsedURL.Hostname())
		if group != nil {
			robotsCached = true
			if !group.Test(parsedURL.Path) {
				err = fmt.Errorf("path %s not allowd by robots", parsedURL.Path)
				return
			}
		}
	}

	var redirectChain *[]string
	if task.Retry.Retried == false {
		redirectChain = &task.RedirectChain
	} else {
		redirectChain = &task.Retry.RedirectChain
	}
	*redirectChain = []string{}

	client, err = MakeClient(parsedURL, redirectChain, &(task.IP))
	if err != nil {
		return
	}

	if !gw.bypassRobots && !robotsCached {
		req, _ = MakeRequest(&url.URL{
			Scheme: parsedURL.Scheme,
			Host:   parsedURL.Hostname(),
			Path:   "/robots.txt",
		})
		resp, err = client.Do(req)

		var robotsGroup *robotstxt.Group
		var respBodyStr string
		var expiration time.Time
		if err != nil {
			robotsGroup = &robotstxt.Group{}
		} else {
			//defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			respBodyStr = string(body)
			expireStr := resp.Header.Get("Expires")
			resp.Body.Close()
			resp = nil
			var robotsRaw *robotstxt.RobotsData
			robotsRaw, err = robotstxt.FromString(respBodyStr)
			if err != nil {
				robotsGroup = &robotstxt.Group{}
			} else {
				robotsGroup = robotsRaw.FindGroup("*")
				if expireStr != "" {
					expiration, _ = time.Parse(time.RFC1123, expireStr)
				}
			}
		}
		if expiration.IsZero() {
			expiration = time.Now().Add(600 * time.Second) // TODO: Decide on this!
		}
		if !robotsGroup.Test(parsedURL.Path) {
			err = fmt.Errorf("path %s not allowd for http", parsedURL.Path)
			gw.ErrorLog(&task, err)
			return
		}

		gw.RBConn.Add(
			parsedURL.Scheme,
			parsedURL.Hostname(),
			expiration,
			robotsGroup,
			respBodyStr,
		)

		*redirectChain = (*redirectChain)[:0]
		client, err = MakeClient(parsedURL, redirectChain, &task.IP)
		if err != nil {
			gw.ErrorLog(&task, err)
			return
		}
	}

	req, _ = MakeRequest(parsedURL)
	resp, err = client.Do(req)
	if err != nil {
		//panic(err)
		err = fmt.Errorf("HTTPS request error: %w", err)
		return
	}
}

// FetchTask is essentially a spread out K-sort.
//
//	Data structure: heap[hostname] -> list of taskStrs
//		each LIST has a scheduled start time (scan timestamp) and a politeness (scan duration)
//	POP: find the LIST with the closest start time, pop a task, increment (next) start time by politeness
//	FetchTask create tasks from taskStrs, this is to save heap memory.
func (gw *GeneralWorker) FetchTask() (cm.TaskPrint, time.Duration) {
	taskStrsByHostname := heap.Pop(&gw.WorkerTasksHeap).(cm.TaskStrsByHostname)

	taskStr := <-taskStrsByHostname.TaskStrs
	line := strings.TrimSpace(taskStr)
	strL := strings.Split(line, ",")
	URL := strings.TrimSpace(strL[0])
	src := strings.TrimSpace(strL[1])
	parsedURL, _ := url.Parse(URL)
	taskSchedule := taskStrsByHostname.Schedule
	task := cm.TaskPrint{
		Source:   src,
		Hostname: parsedURL.Hostname(),
		URL:      URL,
	}

	if len(taskStrsByHostname.TaskStrs) != 0 {
		taskStrsByHostname.Schedule += taskStrsByHostname.Politeness
		heap.Push(&gw.WorkerTasksHeap, taskStrsByHostname)
	}

	return task, taskSchedule
}

func (gw *GeneralWorker) Start() {

	var wg sync.WaitGroup
	start := time.Now()

	for len(gw.WorkerTasksHeap) != 0 {
		task, schedule := gw.FetchTask()
		time.Sleep(schedule - time.Since(start))
		//fmt.Printf("Time: %.2f | Initiating task %s (scheduled: %.2f)\n",
		//	time.Since(start).Seconds(), task.URL, task.Schedule.Seconds())
		wg.Add(1)
		go gw.HandleTask(task, &wg)
	}

	wg.Wait()
}

// StartRetry is a special start funtion made for retry. This func is called by retry manager only.
//
//	If a worker is started with this func, there's no need to create tasks from strings or perform robots check.
func (gw *GeneralWorker) StartRetry(politeness time.Duration) {
	gw.bypassRobots = true
	var wg sync.WaitGroup

	for urlStr := range gw.WorkerTasksStrs {
		tmpTask := &cm.TaskPrint{
			URL: urlStr,
		}
		tmpTask.Retry.Retried = true
		go gw.HandleTask(*tmpTask, &wg)
		wg.Add(1)
		time.Sleep(politeness)
		//fmt.Printf("Time: %.2f | Initiating task %s (scheduled: %.2f)\n",
		//	time.Since(start).Seconds(), task.URL, task.Schedule.Seconds())
	}
	wg.Wait()
}
