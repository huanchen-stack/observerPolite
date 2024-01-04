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
	AllResults    *chan cm.TaskPrint
	TaskPrintsRef *chan cm.TaskPrint
	//RetryBuff     []string
	retryBuffChan chan string
	dbConnPrev    db.DBConn
	//mutex         sync.Mutex // For RetryBuff
}

type RetryManagerInterface interface {
	Start()
	HandleRetry(task cm.TaskPrint)
}

// NeedsRetry tells if a task needs retry by comparing to previous scan results
//
//	This func connects to database's prev collections and fetch previous scan results.
//	Use the latest scan result (except for "429 too many requests"), i.e., if prev scan used a retry, use that retry result!
func (rm *RetryManager) NeedsRetry(taskPrint cm.TaskPrint) bool {
	// if this try is blocked by robots.txt, don't retry
	if len(taskPrint.Err) >= 4 && (taskPrint.Err[0:4] == "path" || taskPrint.Err[0:4] == "Excl") {
		return false
	}

	prevResult := rm.dbConnPrev.GetOneAsync("url", taskPrint.URL) // GetOne (customized) always returns struct
	if prevResult.URL == "" {
		return false
	}
	var prevStatusCode int
	var prevDst, prevErr, prevEtag, prevESelfTag string

	if prevResult.Retry.Retried && prevResult.Retry.Resp.StatusCode != 429 { //retried and not too many requests, use retried result
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
	} else { // otherwise use the normal result
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
		return true
	}

	if prevErr != "" || prevEtag != "" || prevESelfTag != "" {
		// TODO: cur task doesn't possess Etag or ESelfTag
	}

	return false
}

// Start does the following:
//  1. Connect to prev database
//  2. Wakes up periodically to perform retries
//  3. Make printable (db friendly) structs and send those to main
func (rm *RetryManager) Start() {
	// Connect to prev DB collection (start another connection to DB)
	// DB for comp (create index when doesn't exist)
	rm.dbConnPrev = db.DBConn{
		Ctx:           context.Background(),
		ReadBatchChan: make(chan db.DBRequest, 500000),
	}
	rm.dbConnPrev.Connect()
	rm.dbConnPrev.Collection = rm.dbConnPrev.Database.Collection(cm.GlobalConfig.DBCollectionComp) //comp collection
	rm.dbConnPrev.CreateIndex("url")
	rm.retryBuffChan = make(chan string, 500000)
	go db.BatchProcessor[cm.TaskPrint](&rm.dbConnPrev)

	go func() {
		ticker := time.NewTicker(cm.GlobalConfig.RetryPoliteness)
		for range ticker.C {
			fmt.Println("retry manager wakes up")

			curLen := len(rm.retryBuffChan)
			retryBuff := make([]string, curLen)
			for i := 0; i < curLen; i++ {
				retryBuff[i] = <-rm.retryBuffChan
			}
			//rm.mutex.Lock()

			// WG: +1 per retry task assigned to worker
			var wg sync.WaitGroup
			wg.Add(len(retryBuff))

			allRetryResults := make(chan cm.TaskPrint, len(retryBuff))
			politeness := time.Duration( // use float64 -> inf to get around div by 0 exception
				float64(cm.GlobalConfig.RetryPoliteness) / float64(len(retryBuff)))
			worker := wk.GeneralWorker{
				WorkerTasksStrs: make(chan string, len(retryBuff)),
				AllResultsRef:   &allRetryResults,
			}
			for i, _ := range retryBuff {
				worker.WorkerTasksStrs <- retryBuff[i]
			}
			close(worker.WorkerTasksStrs)
			//rm.RetryBuff = rm.RetryBuff[:0]

			//rm.mutex.Unlock()

			// GO! RETRY WORKER GO!
			go worker.StartRetry(politeness)

			go func() {
				for retryResult := range allRetryResults {
					*rm.TaskPrintsRef <- retryResult
					wg.Done()
				}
			}()

			wg.Wait() // WG: +1 per retry task assigned to worker; -1 per retry task sent to DB (print)
			close(allRetryResults)
		}
	}()

	for result := range *rm.AllResults {
		go func(localResult cm.TaskPrint) { // send db requests from multiple routines
			if rm.NeedsRetry(localResult) {
				rm.retryBuffChan <- localResult.URL
				localResult.NeedsRetry = true
			}

			*rm.TaskPrintsRef <- localResult
		}(result)
	}

	//for result := range *rm.AllResults {
	//
	//	if rm.NeedsRetry(result) { // append to buff if task needs retry
	//		rm.mutex.Lock()
	//		rm.RetryBuff = append(rm.RetryBuff, result.URL)
	//		rm.mutex.Unlock()
	//		result.NeedsRetry = true
	//	}
	//
	//	*rm.TaskPrintsRef <- result
	//
	//}
}
