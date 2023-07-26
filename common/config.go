package common

import (
	"time"
)

var GlobalConfig = Config{
	ExpectedRuntime: 10 * time.Second,
	Timeout:         30 * time.Second,
	Retries:         3,
	DBlogging:       true,
	//DBURI:           "mongodb+srv://admin:admin@observerdb.borsr21.mongodb.net/?retryWrites=true&w=majority",
	DBURI:        "mongodb://localhost:27017",
	DBCollection: "xTrialScan",
}
