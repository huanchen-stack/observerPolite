package common

import (
	"net/http"
	"time"
)

type Config struct {
	Debugging          bool
	InputFileName      string
	ExpectedRuntime    time.Duration
	Timeout            time.Duration
	WorkerStress       int
	RobotsBuffSize     int
	Retries            int
	DBlogging          bool
	DBURI              string
	DBWriteFrequency   time.Duration
	DBCollection       string
	DBCollectionComp   string
	ESelfTagBuffLen    int
	RetryPoliteness    time.Duration
	PProfDumpFrequency time.Duration
	HeartbeatEmailFrom string
	HeartbeatEmailTo   string
	HeartbeatEmailPW   string
	HeartbeatDuration  time.Duration
} // all hyperparameters users are allowed to config

type RetryHTTP struct {
	Retried       bool
	RedirectChain []string
	Resp          *http.Response
	Err           error
} // created for the Task struct

type Task struct {
	Source        string
	Hostname      string
	URL           string
	IP            string
	RedirectChain []string
	Resp          *http.Response
	Err           error
	Retry         *RetryHTTP
} // stores scan results

type TaskStrsByHostname struct {
	Schedule   time.Duration
	Politeness time.Duration
	TaskStrs   chan string // taskStr: f"{url}, {source(wiki article)}"
} // this is assigned to workers; each hostname has a struct like this

type RespPrint struct {
	StatusCode int
	Header     map[string][]string
	ETag       string // ETag in http response
	ESelfTag   string // ETag computed by this scanner
} // created for TaskPrint and RetryPrint; db logging helper

type DstChangePrint struct {
	Scheme   bool
	Hostname bool
	Path     bool
	Query    bool
} // created for RetryPrint; redirect analysis helper

type RetryPrint struct {
	Retried       bool
	RedirectChain []string
	DstChange     DstChangePrint
	Resp          RespPrint
	Err           string
} // created for TaskPrint; db logging helper

type TaskPrint struct {
	SourceURL     string
	Domain        string
	URL           string
	IP            string
	RedirectChain []string
	DstChange     DstChangePrint
	Resp          RespPrint
	Err           string
	NeedsRetry    bool
	Retry         RetryPrint
} // db logging helper

type RobotsPrint struct {
	URL         string
	Expiration  time.Time
	RespBodyStr string // store the entire robots.txt
} // db logging helper
