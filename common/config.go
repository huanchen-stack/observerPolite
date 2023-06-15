package common

import (
	"time"
)

var GlobalConfig = Config{
	Timeout:            15 * time.Second, // TLS establishment timeout
	MaxRedirects:       10,               // Maximum number of redirects
	RedirectSucceed:    false,            // Mark all redirects as success (not implemented)
	SessionLimit:       10000,            // Maximum number of concurrent sessions (Router Limit)
	SessionIdleTimeout: 120,              // Threshold for killing idle sessions (Router Cleanup)
}

var SemTLSConn = make(chan struct{}, 3000) // Concurrency limit on TLS establisthments
var SemGETReq = make(chan struct{}, 10000) // Concurrency limit on GET requests
