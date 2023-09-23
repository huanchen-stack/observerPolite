package common

import (
	"net/http"
	"time"
)

type Config struct {
	InputFileName      string
	ExpectedRuntime    time.Duration
	Timeout            time.Duration
	WorkerStress       int
	Retries            int
	DBlogging          bool
	DBURI              string
	DBWriteFrequency   time.Duration
	DBCollection       string
	DBCollectionComp   string
	ESelfTagBuffLen    int
	RetryPoliteness    time.Duration
	HeartbeatEmailFrom string
	HeartbeatEmailTo   string
	HeartbeatEmailPW   string
	HeartbeatDuration  time.Duration
}

type AutoRetryHTTPS struct {
	Retried       bool
	RedirectChain []string
	Resp          *http.Response
	Err           error
}

type Task struct {
	Source        string
	Hostname      string
	URL           string
	Schedule      time.Duration
	IP            string
	RedirectChain []string
	Resp          *http.Response
	Err           error
	Retry         *AutoRetryHTTPS
}

type TaskStrsByHostname struct {
	Schedule   time.Duration
	Politeness time.Duration
	TaskStrs   chan string
}

type RespPrint struct {
	StatusCode int
	Header     map[string][]string
	ETag       string
	ESelfTag   string
}

type DstChangePrint struct {
	Scheme   bool
	Hostname bool
	Path     bool
	Query    bool
}

type RetryPrint struct {
	Retried       bool
	RedirectChain []string
	DstChange     DstChangePrint
	Resp          RespPrint
	Err           string
}

type TaskPrint struct {
	SourceURL     string
	Domain        string
	URL           string
	IP            string
	RedirectChain []string
	DstChange     DstChangePrint
	Resp          RespPrint
	Err           string
	Retry         RetryPrint
}
