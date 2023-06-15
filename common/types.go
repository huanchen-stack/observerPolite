package common

import (
	"net/http"
	"time"
)

type Config struct {
	Timeout            time.Duration
	MaxRedirects       int
	RedirectSucceed    bool
	SessionLimit       int
	SessionIdleTimeout int
}

type Task struct {
	IP       string
	Domain   string
	Endpoint string
	Retries  int
	Resp     *http.Response
}

type AdminMsg struct {
	Msg   string
	Value int
}

type Connection struct {
	Client       *http.Client
	SessionAlive bool
}
