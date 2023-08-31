package common

import (
	"time"
)

var GlobalConfig = Config{
	InputFileName:   "input.txt",
	ExpectedRuntime: 10 * time.Second,
	Timeout:         120 * time.Second,
	WorkerStress:    5000,
	Retries:         3,
	DBlogging:       false,
	DBURI:           "mongodb+srv://admin:admin@observerdb.borsr21.mongodb.net/?retryWrites=true&w=majority",
	//DBURI:              "mongodb://localhost:27017",
	DBCollection:       "",
	DBCollectionComp:   "empty",
	ESelfTagBuffLen:    10240,
	RetryPoliteness:    5 * time.Second,
	HeartbeatEmailFrom: "sunhuanchen99@gmail.com",
	HeartbeatEmailPW:   "", //TODO:use app password
	HeartbeatEmailTo:   "huanchen@umich.edu",
	HeartbeatDuration:  10 * time.Second,
}
