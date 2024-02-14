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
	sitemap.SetFetch(myFetch)

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
		Path:   "/",
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

// HandleSentinel consists of the following
//  1. a host health check, including DNS record lookup and log
//  2. a robots check, may or may not need to access DB, send request
//     2.* upon a success in finding robots.txt, need to filter the entire channel
//  3. a sitemap fetch
//  4. upon new robots.txt or sitemap.xml(txt), need to write to DB
func (wk *Worker) HandleSentinel(taskStrsByHostname *cm.TaskStrsByHostname) {
	// Get parsedURL
	taskStr := <-(*taskStrsByHostname).TaskStrs
	(*taskStrsByHostname).TaskStrs <- taskStr

	line := strings.TrimSpace(taskStr)
	strL := strings.Split(line, ",")
	URL := strings.TrimSpace(strL[0])
	parsedURL, _ := url.Parse(URL)

	// HealthCheck (step 1)
	//   first assume scheme is https, handleTask needs to ignore this errmsg if scheme is http
	var err error
	wk.HealthCheck("https", parsedURL.Hostname(), &taskStrsByHostname.Sentinel.DNSRecords)
	if err != nil {
		(*taskStrsByHostname).Sentinel.HealthErrMsg = err.Error()
		return
	}

	// rand shuffle the fetchSitemap task into the deck, fetchTask is responsible to tell this
	randomFactor := rand.Intn(len(taskStrsByHostname.TaskStrs))
	(*taskStrsByHostname).TaskStrs <- "sitemap"
	for i := 0; i < randomFactor; i++ {
		shuffledStr := <-(*taskStrsByHostname).TaskStrs
		(*taskStrsByHostname).TaskStrs <- shuffledStr
	}
	wk.FetchSitemap("https", parsedURL.Hostname())

	// Fetch robots (DB read + may or may not request); filter the channel (step 2, 2.*, 4)
	var group **robotstxt.Group
	groupHTTP := wk.RBConn.Get("http", parsedURL.Hostname())
	groupHTTPS := wk.RBConn.Get("https", parsedURL.Hostname())
	for i := 0; i < len((*taskStrsByHostname).TaskStrs); i++ {
		taskStrInner := <-(*taskStrsByHostname).TaskStrs
		(*taskStrsByHostname).TaskStrs <- taskStrInner
		parsedURLInner, _ := url.Parse(
			strings.TrimSpace(
				strings.Split(
					strings.TrimSpace(taskStrInner),
					",",
				)[0],
			),
		)
		if parsedURLInner.Scheme == "http" {
			group = &groupHTTP
		} else {
			group = &groupHTTPS
		}
		if *group == nil {
			*group = wk.FetchRobots(parsedURLInner.Scheme, parsedURLInner.Hostname())
		}
		if (*group).Test(parsedURLInner.Path) {
			(*taskStrsByHostname).TaskStrs <- taskStr
		} else {
			fmt.Println("Robots excluded:", parsedURLInner.String())
		}
	}
}
