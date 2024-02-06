package common

import (
	"flag"
	"time"
)

var GlobalConfig = Config{
	Debugging:         true,
	InputFileName:     "input.txt",
	ExpectedRuntime:   1 * time.Second,  // expected total runtime
	Timeout:           20 * time.Second, // see generalworker.go
	WorkerStress:      5000,             // max number of tasks per worker
	RobotsBuffSize:    3,                // write robots to db in batches (also serve as "cache" before writing to db)
	Retries:           3,                // max number of retries when connection err
	HostCheckSlowdown: 3,                // slowdown rand factor for host healthcheck, fetching robots.txt & sitemap.xml
	UserAgent:         "Web Measure/1.0 (https://webresearch.eecs.umich.edu/overview-of-web-measurements/) Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/79.0.3945.130 Safari/537.36",
	DBlogging:         true, // write to database or print only
	DBURI:             "mongodb+srv://admin:admin@observerdb.borsr21.mongodb.net/?retryWrites=true&w=majority",
	//DBURI:              "mongodb://localhost:27017",    // use local mongodb on fable.eecs.umich.edu
	DBWriteFrequency:   5 * time.Second, // write scan results to DB in batches
	DBCollection:       "T",             // new db collection name
	DBCollectionComp:   "EmptyScan",     // prev db collection name (for comparison -> retry)
	ESelfTagBuffLen:    4096000,         // buff size for self Etag compute
	DNSdist:            false,
	DNSdistPort:        "5353",
	RetryPoliteness:    1 * time.Second,  // retry frequency
	PProfDumpFrequency: 60 * time.Second, // profiler (heap/goroutine) dump frequency (for debug)

	//Heartbeat configurations are deprecated... neglect for now
	HeartbeatEmailFrom: "sunhuanchen99@gmail.com",
	HeartbeatEmailPW:   "", //TODO:use app password
	HeartbeatEmailTo:   "huanchen@umich.edu",
	HeartbeatDuration:  10 * time.Second,
}

func ParseFlags() {
	flag.StringVar(
		&GlobalConfig.DBCollection, "dbCollection",
		GlobalConfig.DBCollection, "New db collection name")
	flag.StringVar(
		&GlobalConfig.DBCollectionComp, "dbCollectionComp",
		GlobalConfig.DBCollectionComp, "Previous db collection name")

	flag.Parse()
}

var DNSServers = []string{
	//"127.0.0.1",

	"8.8.8.8",
	"8.8.4.4",
	"1.1.1.1",
	"1.0.0.1",

	// "76.76.2.0",
	// "76.76.10.0",
	// "9.9.9.9",
	// "149.112.112.112",
	// "208.67.222.222",
	// "208.67.220.220",
	// "185.228.168.9",
	// "185.228.169.9",
	// "76.76.19.19",
	// "76.223.122.150",
	// "94.140.14.14",
	// "84.140.15.15",

	// "24.218.117.81",
	// "206.74.78.88",
	// "35.128.46.58",
	// "71.174.91.15",
	// "23.239.215.98",
	// "65.153.78.153",
	// "54.187.61.200",
	// "169.55.51.86",
	// "76.188.165.136",
	// "50.223.52.23",
	// "173.162.106.221",
	// "172.84.132.171",
	// "12.119.50.62",
	// "104.238.212.185",
	// "12.200.199.75",
	// "63.41.144.124",
	// "108.49.223.152",
	// "24.19.134.239",
	// "12.201.30.201",
	// "52.235.135.129",
	// "50.229.170.233",
	// "173.223.99.56",
	// "73.253.160.199",
}

var ExcludedList = map[string]struct{}{}
