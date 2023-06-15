package main

import (
	"flag"
	cm "observerPolite/common"
)

func init() {
	flag.DurationVar(&cm.GlobalConfig.Timeout, "timeout", cm.GlobalConfig.Timeout, "TLS establishment timeout")
	flag.IntVar(&cm.GlobalConfig.MaxRedirects, "max-redirects", cm.GlobalConfig.MaxRedirects, "Maximum number of redirects")
	flag.BoolVar(&cm.GlobalConfig.RedirectSucceed, "redirect-succeed", cm.GlobalConfig.RedirectSucceed, "Mark all redirects as success")
	flag.IntVar(&cm.GlobalConfig.SessionLimit, "session-limit", cm.GlobalConfig.SessionLimit, "Maximum number of concurrent sessions")
	flag.IntVar(&cm.GlobalConfig.SessionIdleTimeout, "session-idle-timeout", cm.GlobalConfig.SessionIdleTimeout, "Threshold for killing idle sessions")

	var TLSConnLimit, GETReqLimit int
	flag.IntVar(&TLSConnLimit, "tls-conn-limit", 1000, "Concurrency limit on TLS establishments")
	flag.IntVar(&GETReqLimit, "get-req-limit", cm.GlobalConfig.SessionLimit, "Concurrency limit on GET requests")

	flag.Parse()

	cm.SemTLSConn = make(chan struct{}, TLSConnLimit)
	cm.SemGETReq = make(chan struct{}, GETReqLimit)
}
