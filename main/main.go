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
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	go func() {
		periodicHeapDump("heap_pprof.out", cm.GlobalConfig.PProfDumpFrequency)
	}()
	go func() {
		periodicGoroutineDump("goroutine_pprof.out", cm.GlobalConfig.PProfDumpFrequency)
	}()

	// Handle DB connection
	dbConn := db.DBConn{context.Background(), nil, nil, nil}
	dbConn.Connect()
	if cm.GlobalConfig.DBCollection != "" {
		dbConn.NewCollection(cm.GlobalConfig.DBCollection)
	}

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
	allResults := make(chan cm.Task, 1000)

	for i, _ := range workerTaskStrList {
		workerTasksHeap := &cm.HeapSlice{}
		heap.Init(workerTasksHeap)
		worker := wk.GeneralWorker{
			WorkerTasksHeap: *workerTasksHeap,
			AllResultsRef:   &allResults,
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
	taskPrints := make(chan cm.TaskPrint, 1000)
	retryManager := rm.RetryManager{
		AllResults:    &allResults,
		TaskPrintsRef: &taskPrints,
	}
	go retryManager.Start()

	// GO! DB GO!
	go func() {
		var writeBuff []cm.TaskPrint
		go func() {
			ticker := time.NewTicker(cm.GlobalConfig.DBWriteFrequency)
			for range ticker.C {
				dbConn.BulkWrite(writeBuff)
				for range writeBuff {
					wg.Done()
				}
				writeBuff = writeBuff[:0]
			}
		}()
		for taskPrint := range taskPrints {
			writeBuff = append(writeBuff, taskPrint)
		}
	}()

	wg.Wait()
	time.Sleep(cm.GlobalConfig.DBWriteFrequency)
	dbConn.Disconnect()

	//time.Sleep(1000000 * time.Second)
}
