package common

import (
	"time"
)

var GlobalConfig = Config{
	ExpectedRuntime: 10 * time.Second,
	Timeout:         30 * time.Second,
	Retries:         3,
	DBlogging:       true,
	DBCollection:    "xTrialScan",
}
