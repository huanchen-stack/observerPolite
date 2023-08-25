package retrymanager

import (
	"context"
	"fmt"
	cm "observerPolite/common"
	db "observerPolite/mongodb"
	"strings"
	"time"
)

type RetryManager struct {
	BuffTasks  chan cm.TaskPrint
	dbConnPrev db.DBConn
}

type RetryManagerInterface interface {
	Start()
	HandleRetry(task cm.Task)
}

func (rm *RetryManager) HandleRetry(taskPrint cm.TaskPrint) bool {
	prevResult := (rm.dbConnPrev.GetOne("url", taskPrint.URL))
	var prevStatusCode int
	var prevDst, prevErr, prevEtag, prevESelfTag string

	if prevResult.Retry.Retried && prevResult.Retry.Resp.StatusCode != 429 { //retried and not too many requests
		//use prev retried result
		prevStatusCode = prevResult.Retry.Resp.StatusCode
		if len(prevResult.Retry.RedirectChain) > 0 {
			prevDst = prevResult.Retry.RedirectChain[len(prevResult.Retry.RedirectChain)-1]
			prevDst = strings.Split(prevDst, "")[len(strings.Split(prevDst, ""))-1]
		} else {
			prevDst = ""
		}
		prevErr = prevResult.Retry.Err
		prevEtag = prevResult.Retry.Resp.ETag
		prevESelfTag = prevResult.Retry.Resp.ESelfTag
	} else {
		prevStatusCode = prevResult.Resp.StatusCode
		if len(prevResult.RedirectChain) > 0 {
			prevDst = prevResult.RedirectChain[len(prevResult.RedirectChain)-1]
			prevDst = strings.Split(prevDst, "")[len(strings.Split(prevDst, ""))-1]
		} else {
			prevDst = ""
		}
		prevErr = prevResult.Err
		prevEtag = prevResult.Resp.ETag
		prevESelfTag = prevResult.Resp.ESelfTag
	}

	if prevStatusCode != taskPrint.Resp.StatusCode {
		// TODO: do something
		return true
	}

	var dst string
	if len(taskPrint.RedirectChain) > 0 {
		dst = taskPrint.RedirectChain[len(taskPrint.RedirectChain)-1]
		dst = strings.Split(dst, "")[len(strings.Split(dst, ""))-1]
	} else {
		dst = ""
	}
	if prevDst != dst {
		// TODO: do something
		return true
	}

	if prevErr != "" || prevEtag != "" || prevESelfTag != "" {
		// TODO: cur task doesn't possess Etag or ESelfTag
	}

	return false
}

func (rm *RetryManager) Start() {
	rm.dbConnPrev = db.DBConn{context.Background(), nil, nil, nil}
	rm.dbConnPrev.Connect()
	rm.dbConnPrev.Collection = rm.dbConnPrev.Database.Collection(cm.GlobalConfig.DBCollectionComp) //comp collection
	rm.dbConnPrev.CreateIndex("url")

	ticker := time.NewTicker(cm.GlobalConfig.RetryPoliteness)
	for range ticker.C {
		for len(rm.BuffTasks) > 0 { // no greed
			taskPrint := <-rm.BuffTasks
			if rm.HandleRetry(taskPrint) {
				fmt.Printf("needs retry: %s\n", taskPrint.URL)
			}
		}
	}
}
