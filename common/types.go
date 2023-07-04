package common

import (
	"net/http"
	"time"
)

type Config struct {
	ExpectedRuntime time.Duration
	Retries         int
}

type Task struct {
	Domain         string
	URL            string
	AutoRetryHTTPS bool
	IP             string
	RedirectChain  []string
	Resp           *http.Response
	Err            error
}

type TaskPrint struct {
	Domain         string
	URL            string
	IP             string
	AutoRetryHTTPS bool
	StatusCode     int
	RedirectChain  []string
	Err            string
}
