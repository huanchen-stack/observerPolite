package retrymanager

import (
	"context"
	"fmt"
	cm "observerPolite/common"
	db "observerPolite/mongodb"
	wk "observerPolite/worker"
	"strings"
	"sync"
	"time"
)

type RetryManager struct {
	AllResults    *chan cm.Task
	TaskPrintsRef *chan cm.TaskPrint
	RetryBuff     []cm.Task
	dbConnPrev    db.DBConn
	mutex         sync.Mutex
}

type RetryManagerInterface interface {
	Start()
	HandleRetry(task cm.Task)
}

func (rm *RetryManager) NeedsRetry(taskPrint cm.TaskPrint) bool {
	prevResult := rm.dbConnPrev.GetOne("url", taskPrint.URL) // GetOne (customized) always returns struct
	if prevResult.URL == "" {
		return false
	}
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
	// DB for comp (create index when doesn't exist)
	rm.dbConnPrev = db.DBConn{context.Background(), nil, nil, nil}
	rm.dbConnPrev.Connect()
	rm.dbConnPrev.Collection = rm.dbConnPrev.Database.Collection(cm.GlobalConfig.DBCollectionComp) //comp collection
	rm.dbConnPrev.CreateIndex("url")

	go func() {
		ticker := time.NewTicker(cm.GlobalConfig.RetryPoliteness)
		for range ticker.C {
			fmt.Println("retry manager wakes up")
			rm.mutex.Lock()

			// Naive schedule
			start := time.Duration(0.0)
			for i, _ := range rm.RetryBuff {
				rm.RetryBuff[i].Schedule = start
				start = time.Duration(
					(start.Seconds() +
						cm.GlobalConfig.RetryPoliteness.Seconds()/float64(len(rm.RetryBuff))) *
						float64(time.Second))
				rm.RetryBuff[i].Retry = &cm.AutoRetryHTTPS{
					Retried: true,
				}
			}

			var wg sync.WaitGroup
			wg.Add(len(rm.RetryBuff))

			allRetryResults := make(chan cm.Task, len(rm.RetryBuff))
			worker := wk.GeneralWorker{
				WorkerTasks:   make(chan cm.Task, len(rm.RetryBuff)),
				AllResultsRef: &allRetryResults,
			}
			for i, _ := range rm.RetryBuff {
				worker.WorkerTasks <- rm.RetryBuff[i]
			}
			close(worker.WorkerTasks)
			rm.RetryBuff = rm.RetryBuff[:0]

			rm.mutex.Unlock()

			go worker.StartWTasks()

			go func() {
				for retryResult := range allRetryResults {
					*rm.TaskPrintsRef <- cm.PrintTask(retryResult)
					wg.Done()
				}
			}()

			wg.Wait()
			close(allRetryResults)
		}
	}()

	for result := range *rm.AllResults {
		resultPrint := cm.PrintTask(result)
		if rm.NeedsRetry(resultPrint) {
			rm.mutex.Lock()
			rm.RetryBuff = append(rm.RetryBuff, result)
			rm.mutex.Unlock()
		} else {
			*rm.TaskPrintsRef <- resultPrint
		}
	}
}
