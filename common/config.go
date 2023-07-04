package common

import (
	"time"
)

var GlobalConfig = Config{
	ExpectedRuntime: 10 * time.Second,
	Retries:         3,
}
