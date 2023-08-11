package common

import (
	"time"
)

var GlobalConfig = Config{
	InputFileName:   "input.txt",
	ExpectedRuntime: 10 * time.Second,
	Timeout:         120 * time.Second,
	Retries:         3,
	DBlogging:       false,
	//DBURI:           "mongodb+srv://admin:admin@observerdb.borsr21.mongodb.net/?retryWrites=true&w=majority",
	DBURI:           "mongodb://localhost:27017",
	DBCollection:    "xTrialScan",
	ESelfTagBuffLen: 10240,
}
