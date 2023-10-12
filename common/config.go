package common

import (
	"time"
)

var GlobalConfig = Config{
	InputFileName:   "input.txt",
	ExpectedRuntime: 30 * time.Second,  // expected total runtime
	Timeout:         120 * time.Second, // see generalworker.go
	WorkerStress:    5000,              // max number of tasks per worker
	RobotsBuffSize:  5,                 // write robots to db in batches (also serve as "cache" before writing to db)
	Retries:         3,                 // max number of retries when connection err
	DBlogging:       true,              // write to database or print only
	DBURI:           "mongodb+srv://admin:admin@observerdb.borsr21.mongodb.net/?retryWrites=true&w=majority",
	//DBURI:              "mongodb://localhost:27017",    // use local mongodb on fable.eecs.umich.edu
	DBWriteFrequency:   10 * time.Second, // write scan results to DB in batches
	DBCollection:       "CompTest2_",     // new db collection name
	DBCollectionComp:   "CompTest1_",     // prev db collection name (for comparison -> retry)
	ESelfTagBuffLen:    10240,            // buff size for self Etag compute
	RetryPoliteness:    5 * time.Second,  // retry frequency
	PProfDumpFrequency: 5 * time.Second,  // profiler (heap/goroutine) dump frequency (for debug)

	//Heartbeat configurations are deprecated... neglect for now
	HeartbeatEmailFrom: "sunhuanchen99@gmail.com",
	HeartbeatEmailPW:   "", //TODO:use app password
	HeartbeatEmailTo:   "huanchen@umich.edu",
	HeartbeatDuration:  10 * time.Second,
}

var DNSServers = []string{
	"8.8.8.8",
	"8.8.4.4",
	"1.1.1.1",
	"1.0.0.1",
	"76.76.2.0",
	"76.76.10.0",
	"9.9.9.9",
	"149.112.112.112",
	"208.67.222.222",
	"208.67.220.220",
	// "185.228.168.9",
	// "185.228.169.9",
	// "76.76.19.19",
	// "76.223.122.150",
	// "94.140.14.14",
	// "84.140.15.15",
}
