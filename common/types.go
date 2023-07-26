package common

import (
	"net/http"
	"time"
)

type Config struct {
	ExpectedRuntime time.Duration
	Timeout         time.Duration
	Retries         int
	DBlogging       bool
	DBURI           string
	DBCollection    string
}

type AutoRetryHTTPS struct {
	Retried       bool
	RedirectChain []string
	Resp          *http.Response
	Err           error
}

type Task struct {
	Domain         string
	URL            string
	IP             string
	RedirectChain  []string
	Resp           *http.Response
	Err            error
	AutoRetryHTTPS *AutoRetryHTTPS
}
