package common

import (
	"net/http"
	"time"
)

type Config struct {
	InputFileName   string
	ExpectedRuntime time.Duration
	Timeout         time.Duration
	Retries         int
	DBlogging       bool
	DBURI           string
	DBCollection    string
	ESelfTagBuffLen int
}

type AutoRetryHTTPS struct {
	Retried       bool
	RedirectChain []string
	Resp          *http.Response
	Err           error
}

type Task struct {
	SourceURL      string
	Domain         string
	URL            string
	IP             string
	RedirectChain  []string
	Resp           *http.Response
	Err            error
	AutoRetryHTTPS *AutoRetryHTTPS
}

type RespPrint struct {
	StatusCode int
	Header     map[string][]string
	ETag       string
	ESelfTag   string // if Etag is not provided in header
}

type AutoRetryHTTPSPrint struct {
	Retried       bool
	RedirectChain []string
	Resp          RespPrint
	Err           string
}

type TaskPrint struct {
	SourceURL      string
	Domain         string
	URL            string
	IP             string
	RedirectChain  []string
	Resp           RespPrint
	Err            string
	AutoRetryHTTPS AutoRetryHTTPSPrint
}
