package main

import (
	"time"
)

type Config struct {
	Timeout         time.Duration
	MaxRedirects    int
	RedirectSucceed bool
}

var GlobalConfig = Config{}
