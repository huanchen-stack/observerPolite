package main

import (
	"container/heap"
	"context"
	"log"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	cm "observerPolite/common"
	db "observerPolite/mongodb"
	rm "observerPolite/retrymanager"
	wk "observerPolite/worker"
	"os"
	"runtime/pprof"
	"sync"
	"time"
)

func periodicHeapDump(filename string, duration time.Duration) {
	ticker := time.NewTicker(duration)
	defer ticker.Stop()

	for range ticker.C {
		f, err := os.Create(filename)
		if err != nil {
			// Handle error
			return
		}
		pprof.WriteHeapProfile(f)
		f.Close()
	}
}

func periodicGoroutineDump(filename string, duration time.Duration) {
	ticker := time.NewTicker(duration)
	defer ticker.Stop()

	for range ticker.C {
		f, err := os.Create(filename)
		if err != nil {
			// Handle error
			return
		}
		pprof.Lookup("goroutine").WriteTo(f, 0)
		f.Close()
	}
}

func main() {

	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil)) // for pprof
	}()
	go func() {
		periodicHeapDump("heap_pprof.out", cm.GlobalConfig.PProfDumpFrequency)
	}()
	go func() {
		periodicGoroutineDump("goroutine_pprof.out", cm.GlobalConfig.PProfDumpFrequency)
	}()

	// Handle DB connection for scan results
	dbConn := db.DBConn{
		Ctx:        context.Background(),
		Client:     nil,
		Database:   nil,
		Collection: nil,
	}
	dbConn.Connect()
	if cm.GlobalConfig.DBCollection != "" { // for debugging
		dbConn.NewCollection(cm.GlobalConfig.DBCollection)
	}

	// Handle DB connection for robots.txt
	rbConn := db.RobotsDBConn{
		Ctx:       context.Background(),
		CacheSize: cm.GlobalConfig.RobotsBuffSize,
	}
	rbConn.Connect()

	// Read and parse from .txt
	taskStrs, err := cm.ReadTaskStrsFromInput(cm.GlobalConfig.InputFileName)
	if err != nil {
		panic(err)
	}

	// Add WG: +1 per task
	var wg sync.WaitGroup
	wg.Add(len(taskStrs))

	// Group tasks
	workerTaskStrList := cm.GroupTasks(taskStrs)

	// Make workers
	var workerList []wk.GeneralWorker
	allResults := make(chan cm.Task, 500000) // all workers write to this
	for i, _ := range workerTaskStrList {    // see generalworker.go for details
		workerTasksHeap := &cm.HeapSlice{}
		heap.Init(workerTasksHeap)
		worker := wk.GeneralWorker{
			WorkerTasksHeap: *workerTasksHeap,
			AllResultsRef:   &allResults,
			RBConn:          &rbConn,
		}
		for j, _ := range workerTaskStrList[i] {
			politeness := time.Duration(cm.GlobalConfig.ExpectedRuntime.Seconds() / float64(len(workerTaskStrList[i][j])))
			hp := cm.TaskStrsByHostname{
				Schedule:   time.Duration(rand.Float64()*float64(politeness)) * time.Second,
				Politeness: politeness * time.Second,
				TaskStrs:   make(chan string, len(workerTaskStrList[i][j])),
			}
			for k, _ := range workerTaskStrList[i][j] {
				hp.TaskStrs <- workerTaskStrList[i][j][k]
			}
			close(hp.TaskStrs)
			heap.Push(&worker.WorkerTasksHeap, hp)
		}
		workerList = append(workerList, worker)
	}
	// NOW IT'S SAFE TO DISCARD THE ORI COPY!
	workerTaskStrList = nil
	taskStrs = nil

	// GO! WORKERS, GO!
	for i, _ := range workerList {
		go workerList[i].Start()
	}

	//// GO! HEARTBEAT, GO!
	//if cm.GlobalConfig.DBCollection != "" {
	//	hb := hb.Heartbeat{DBConn: &dbConn}
	//	go hb.Start()
	//}

	// GO! RETRY MANAGER, GO!
	taskPrints := make(chan cm.TaskPrint, 500000) // all scan results are passed to retry manager
	retryManager := rm.RetryManager{              // see retrymanager.go for details
		AllResults:    &allResults, // 		retry manager decides if a task needs to be retried
		TaskPrintsRef: &taskPrints, // 		retry manager also create printable tasks for db logging
	}
	go retryManager.Start()

	// GO! DB GO!
	go func() {
		var writeBuff []cm.TaskPrint
		var mutex sync.Mutex // for writeBuff

		// Wakes up periodically and flush all printable results from buffer to DB
		go func() {
			ticker := time.NewTicker(cm.GlobalConfig.DBWriteFrequency)
			for range ticker.C {
				mutex.Lock()
				dbConn.BulkWrite(writeBuff)
				for range writeBuff {
					wg.Done()
				}
				writeBuff = writeBuff[:0]
				mutex.Unlock()
			}
		}()

		// Listen for all printable tasks, append to buff
		for taskPrint := range taskPrints {
			mutex.Lock()
			writeBuff = append(writeBuff, taskPrint)
			mutex.Unlock()
		}
	}()

	wg.Wait()                                    // WG: +1 per task assigned to workers; -1 per task logged to DB
	time.Sleep(cm.GlobalConfig.DBWriteFrequency) // in case the program would end before db logging finishes

	rbConn.Disconnect()
	dbConn.Disconnect()
}
