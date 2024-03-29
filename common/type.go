package common

import (
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
	SentinelPoliteness int
	UserAgent          string
	DBlogging          bool
	DBURI              string
	DBWriteFrequency   time.Duration
	DBCollection       string
	DBCollectionComp   string
	ESelfTagBuffLen    int
	DNSdist            bool
	DNSdistPort        string
	RetryPoliteness    time.Duration
	PProfDumpFrequency time.Duration

	HeartbeatEmailFrom string
	HeartbeatEmailTo   string
	HeartbeatEmailPW   string
	HeartbeatDuration  time.Duration
} // all hyperparameters users are allowed to config

type Redirect struct {
	Statuscode int
	Location   string
	LocationIP string
}

type DNSRecord struct {
	Hostname string
	IP       string
	IPs      []string
	CNAMEs   []string
	MXs      []string
	NSs      []string
	TXTs     []string
	PTRs     []string
}

type Sentinel struct {
	Handling     bool
	Handled      bool
	HealthErrMsg string
	DNSRecords   []DNSRecord
}

type TaskStrsByHostname struct {
	Hostname   string
	Schedule   time.Duration
	Politeness time.Duration
	Sentinel   *Sentinel
	TaskStrs   *chan string // taskStr: f"{url}, {source(wiki article)}"
} // this is assigned to workers; each hostname has a struct like this

type RespPrint struct {
	StatusCode int
	Header     map[string][]string
	ETag       string // ETag in http response
	ESelfTag   string // ETag computed by this scanner
	Size       int
} // created for TaskPrint and RetryPrint; db logging helper

type DstChangePrint struct {
	Scheme   bool
	Hostname bool
	Path     bool
	Query    bool
} // created for RetryPrint; redirect analysis helper

type RetryHTTPPrint struct {
	Retried       bool
	RedirectChain []Redirect
	DstChange     DstChangePrint
	Resp          RespPrint
	Err           string
} // created for the TaskPrint struct

type TaskPrint struct {
	Source        string
	Hostname      string
	URL           string
	IP            string
	DNSRecords    []DNSRecord
	RedirectChain []Redirect
	DstChange     DstChangePrint
	Resp          RespPrint
	Err           string
	NeedsRetry    bool
	Retry         RetryHTTPPrint
} // stores scan results

func (tp TaskPrint) GetURL() string { return tp.URL }

type RobotsPrint struct {
	URL         string
	Expiration  time.Time
	RespBodyStr string // store the entire robots.txt
} // db logging helper

func (rp RobotsPrint) GetURL() string { return rp.URL }
