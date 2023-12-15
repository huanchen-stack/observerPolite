package common

import (
	"flag"
	"time"
)

var GlobalConfig = Config{
	Debugging:       true,
	InputFileName:   "input.txt",
	ExpectedRuntime: 1 * time.Second,  // expected total runtime
	Timeout:         10 * time.Second, // see generalworker.go
	WorkerStress:    5000,             // max number of tasks per worker
	RobotsBuffSize:  3,                // write robots to db in batches (also serve as "cache" before writing to db)
	Retries:         3,                // max number of retries when connection err
	DBlogging:       true,             // write to database or print only
	DBURI:           "mongodb+srv://admin:admin@observerdb.borsr21.mongodb.net/?retryWrites=true&w=majority",
	//DBURI:              "mongodb://localhost:27017",    // use local mongodb on fable.eecs.umich.edu
	DBWriteFrequency:   5 * time.Second,  // write scan results to DB in batches
	DBCollection:       "T",              // new db collection name
	DBCollectionComp:   "EmptyScan",      // prev db collection name (for comparison -> retry)
	ESelfTagBuffLen:    4096000,          // buff size for self Etag compute
	RetryPoliteness:    1 * time.Second,  // retry frequency
	PProfDumpFrequency: 60 * time.Second, // profiler (heap/goroutine) dump frequency (for debug)

	//Heartbeat configurations are deprecated... neglect for now
	HeartbeatEmailFrom: "sunhuanchen99@gmail.com",
	HeartbeatEmailPW:   "", //TODO:use app password
	HeartbeatEmailTo:   "huanchen@umich.edu",
	HeartbeatDuration:  10 * time.Second,
}

func ParseFlags() {
	flag.BoolVar(&GlobalConfig.Debugging, "debugging", GlobalConfig.Debugging, "Debugging with pprof")
	flag.StringVar(&GlobalConfig.InputFileName, "inputFileName", GlobalConfig.InputFileName, "Name of the input file")
	flag.DurationVar(&GlobalConfig.ExpectedRuntime, "expectedRuntime", GlobalConfig.ExpectedRuntime, "Expected total runtime")
	flag.DurationVar(&GlobalConfig.Timeout, "timeout", GlobalConfig.Timeout, "General worker timeout")
	flag.IntVar(&GlobalConfig.WorkerStress, "workerStress", GlobalConfig.WorkerStress, "Max number of tasks per worker")
	flag.IntVar(&GlobalConfig.RobotsBuffSize, "robotsBuffSize", GlobalConfig.RobotsBuffSize, "Write robots to db in batches")
	flag.IntVar(&GlobalConfig.Retries, "retries", GlobalConfig.Retries, "Max number of retries when connection error")
	flag.BoolVar(&GlobalConfig.DBlogging, "dbLogging", GlobalConfig.DBlogging, "Write to database or print only")
	flag.StringVar(&GlobalConfig.DBURI, "dbURI", GlobalConfig.DBURI, "Database URI")
	flag.DurationVar(&GlobalConfig.DBWriteFrequency, "dbWriteFrequency", GlobalConfig.DBWriteFrequency, "Write scan results to DB in batches")
	flag.StringVar(&GlobalConfig.DBCollection, "dbCollection", GlobalConfig.DBCollection, "New db collection name")
	flag.StringVar(&GlobalConfig.DBCollectionComp, "dbCollectionComp", GlobalConfig.DBCollectionComp, "Previous db collection name")
	flag.IntVar(&GlobalConfig.ESelfTagBuffLen, "eSelfTagBuffLen", GlobalConfig.ESelfTagBuffLen, "Buffer size for self ETag compute")
	flag.DurationVar(&GlobalConfig.RetryPoliteness, "retryPoliteness", GlobalConfig.RetryPoliteness, "Retry frequency")
	flag.DurationVar(&GlobalConfig.PProfDumpFrequency, "pProfDumpFrequency", GlobalConfig.PProfDumpFrequency, "Profiler dump frequency")

	//flag.StringVar(&GlobalConfig.HeartbeatEmailFrom, "heartbeatEmailFrom", GlobalConfig.HeartbeatEmailFrom, "Heartbeat email from address")
	//flag.StringVar(&GlobalConfig.HeartbeatEmailPW, "heartbeatEmailPW", GlobalConfig.HeartbeatEmailPW, "Heartbeat email password")
	//flag.StringVar(&GlobalConfig.HeartbeatEmailTo, "heartbeatEmailTo", GlobalConfig.HeartbeatEmailTo, "Heartbeat email to address")
	//flag.DurationVar(&GlobalConfig.HeartbeatDuration, "heartbeatDuration", GlobalConfig.HeartbeatDuration, "Heartbeat duration")

	flag.Parse()
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

var ExcludedList = map[string]struct{}{}
