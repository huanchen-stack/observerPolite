package common

import (
	"time"
)

var GlobalConfig = Config{
	InputFileName:         "input.txt",
	ExpectedRuntime:       10 * time.Second,
	Timeout:               120 * time.Second,
	WorkerStress:          5000,
	WorkerRobotsCacheSize: 500,
	Retries:               3,
	DBlogging:             false,
	DBURI:                 "mongodb+srv://admin:admin@observerdb.borsr21.mongodb.net/?retryWrites=true&w=majority",
	//DBURI:              "mongodb://localhost:27017",
	DBWriteFrequency:   10 * time.Second,
	DBCollection:       "",
	DBCollectionComp:   "empty",
	ESelfTagBuffLen:    10240,
	RetryPoliteness:    5 * time.Second,
	PProfDumpFrequency: 5 * time.Second,
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
