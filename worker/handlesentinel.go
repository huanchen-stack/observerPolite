package worker

import (
	"fmt"
	"github.com/temoto/robotstxt"
	"github.com/yterajima/go-sitemap"
	"io"
	"math/rand"
	"net/url"
	cm "observerPolite/common"
	"strings"
	"time"
)

func (wk *Worker) HealthCheck(scheme string, hostname string, dnsRecords *[]cm.DNSRecord) error {
	response, err := wk.HTTPGET(&url.URL{
		Scheme: scheme,
		Host:   hostname,
		Path:   "/",
	}, nil, dnsRecords)
	if err != nil {
		return err // includes dns, tcp, tls (currently set scheme to https always)
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return fmt.Errorf("status: %d", response.StatusCode)
	}

	return nil
}

func (wk *Worker) FetchSitemap(scheme string, hostname string) {
	fetchLock.Lock()
	sitemap.SetFetch(myFetch)
	fetchLock.Unlock()

	smap, err := sitemap.Get((&url.URL{
		Scheme: scheme,
		Host:   hostname,
		Path:   "/sitemap.xml",
	}).String(), nil)
	if err != nil {
		return
	}

	wk.SPConn.WriteChan <- smap
}

func (wk *Worker) FetchRobots(scheme string, hostname string) *robotstxt.Group {
	response, err := wk.HTTPGET(&url.URL{
		Scheme: scheme,
		Host:   hostname,
		Path:   "/robots.txt",
	}, nil, nil)
	if err != nil || response.StatusCode != 200 {
		return &robotstxt.Group{}
	}
	defer response.Body.Close()

	var robotsGroup *robotstxt.Group
	var respBodyStr string
	var expiration time.Time
	if err != nil {
		robotsGroup = &robotstxt.Group{}
	} else {
		body, _ := io.ReadAll(response.Body)
		respBodyStr = string(body)
		expireStr := response.Header.Get("Expires")
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
		expiration = time.Now().Add(cm.GlobalConfig.ExpectedRuntime)
	}

	wk.RBConn.Add(scheme, hostname, expiration, robotsGroup, respBodyStr)

	return robotsGroup
}

func (wk *Worker) PseudoHandleSentinel(taskStrsByHostname *cm.TaskStrsByHostname) {
	closedTaskStrs := *taskStrsByHostname.TaskStrs
	newTaskStrs := make(chan string, len(closedTaskStrs)+1)
	i := 0
	for str := range closedTaskStrs {
		fmt.Println(taskStrsByHostname.Hostname, "-[", i, "]-", str)
		newTaskStrs <- str
		i++
	}
	newTaskStrs <- "sitemap"
	close(newTaskStrs)

	*taskStrsByHostname.TaskStrs = newTaskStrs
}

// HandleSentinel consists of the following
//  1. a host health check, including DNS record lookup and log
//  2. a robots check, may or may not need to access DB, send request
//     2.* upon a success in finding robots.txt, need to filter the entire channel
//  3. a sitemap fetch
//  4. upon new robots.txt or sitemap.xml(txt), need to write to DB
//
// Return # WG decrement!
func (wk *Worker) HandleSentinel(taskStrsByHostname *cm.TaskStrsByHostname) int {
	closedTaskStrs := *taskStrsByHostname.TaskStrs
	newTaskStrs := make(chan string, len(closedTaskStrs)+1)

	defer func() {
		//	fmt.Println(taskStrsByHostname.Hostname, "-[", i, "]-", str)
		close(newTaskStrs)
		*taskStrsByHostname.TaskStrs = newTaskStrs
		taskStrsByHostname.Sentinel.Handled = true
		taskStrsByHostname.Sentinel.Handling = false
	}()

	fmt.Println("doing health check for", taskStrsByHostname.Hostname)
	err := wk.HealthCheck("https", taskStrsByHostname.Hostname, &taskStrsByHostname.Sentinel.DNSRecords)
	if err != nil {
		taskStrsByHostname.Sentinel.HealthErrMsg = err.Error()
		for taskStr := range closedTaskStrs {
			newTaskStrs <- taskStr
		}
		fmt.Println("health check err for", taskStrsByHostname.Hostname, "| due to", taskStrsByHostname.Sentinel.HealthErrMsg)
		return 0
	}

	// randomly insert sitemap fetch as a task
	randomFactor := rand.Intn(len(closedTaskStrs) + 1)

	// fetch robots from DB/host prior to filtering
	if taskStrsByHostname.Hostname == "www.microsoft.com" {
		fmt.Println()
	}
	var group **robotstxt.Group
	groupHTTP := wk.RBConn.Get("http", taskStrsByHostname.Hostname)
	if groupHTTP == nil {
		groupHTTP = wk.FetchRobots("http", taskStrsByHostname.Hostname)
	}
	groupHTTPS := wk.RBConn.Get("https", taskStrsByHostname.Hostname)
	if groupHTTPS == nil {
		groupHTTPS = wk.FetchRobots("https", taskStrsByHostname.Hostname)
	}

	// wait group decrement
	wgDec := 0

	i := 0
	for taskStr := range closedTaskStrs {
		if i == randomFactor {
			newTaskStrs <- "sitemap"
			fmt.Println(taskStrsByHostname.Hostname, "inserted sitemap task")
		}
		parsedURLInner, _ := url.Parse(
			strings.TrimSpace(
				strings.Split(
					strings.TrimSpace(
						taskStr,
					),
					",",
				)[0],
			),
		)
		if parsedURLInner.Scheme == "http" {
			group = &groupHTTP
		} else {
			group = &groupHTTPS
		}
		if (*group).Test(parsedURLInner.Path) {
			newTaskStrs <- taskStr
			fmt.Println(taskStrsByHostname.Hostname, "-[", i, "]-", taskStr)
		} else {
			wgDec++
			fmt.Println("Robots excluded:", parsedURLInner.String())
		}
		i += 1
	}
	return wgDec
}
