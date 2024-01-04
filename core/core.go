package core

import (
	"container/heap"
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
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

func CORE() {

	//cm.ParseFlags()

	if cm.GlobalConfig.Debugging {
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil)) // for pprof
		}()
		go func() {
			periodicHeapDump("heap_pprof.out", cm.GlobalConfig.PProfDumpFrequency)
		}()
		go func() {
			periodicGoroutineDump("goroutine_pprof.out", cm.GlobalConfig.PProfDumpFrequency)
		}()
	}

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
		dbConn.CreateIndex("url")
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

	var wg sync.WaitGroup

	// Group tasks
	workerTaskStrList := cm.GroupTasks(taskStrs)

	// Make workers
	var workerList []wk.GeneralWorker
	allResults := make(chan cm.TaskPrint, 500000) // all workers write to this
	for i, _ := range workerTaskStrList {         // see generalworker.go for details
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
				wg.Add(1)
			}
			close(hp.TaskStrs)
			heap.Push(&worker.WorkerTasksHeap, hp)
		}
		workerList = append(workerList, worker)
	}
	// NOW IT'S SAFE TO DISCARD THE ORI COPY!
	workerTaskStrList = nil
	taskStrs = nil

	// Update excluded lists from workers
	go func() {
		ticker := time.NewTicker(600 * time.Second)
		for range ticker.C {
			cm.ExcludedList = cm.ReadExcludedHostnames()
		}
	}()

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

		// Wakes up periodically and flush all printable results from buffer to DB
		go func() {
			ticker := time.NewTicker(cm.GlobalConfig.DBWriteFrequency)
			for range ticker.C {
				curLen := len(taskPrints)
				writeBuff := make([]cm.TaskPrint, curLen)
				for i := 0; i < curLen; i++ {
					writeBuff[i] = <-taskPrints
				}
				doneWG, _ := dbConn.BulkWrite(writeBuff)

				fmt.Println("DB LOG:", curLen, " \t| doneWG:", doneWG)

				for i := 0; i < doneWG; i++ {
					wg.Done()
				}
			}
		}()
	}()

	wg.Wait()                                    // WG: +1 per task assigned to workers; -1 per task logged to DB
	time.Sleep(cm.GlobalConfig.DBWriteFrequency) // in case the program would end before db logging finishes

	rbConn.Disconnect()
	dbConn.Disconnect()
}
