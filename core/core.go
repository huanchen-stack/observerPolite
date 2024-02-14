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

	cm.ParseFlags()
	fmt.Println(cm.GlobalConfig.DBCollection, cm.GlobalConfig.DBCollectionComp)

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
		Ctx: context.Background(),
	}
	dbConn.Connect()
	if cm.GlobalConfig.DBCollection != "" { // for debugging
		dbConn.NewCollection(cm.GlobalConfig.DBCollection)
		dbConn.CreateIndex("url")
	}
	defer dbConn.Disconnect()

	// Handle DB connection for robots.txt
	rbConn := db.RobotsDBConn{
		Ctx:       context.Background(),
		CacheSize: cm.GlobalConfig.RobotsBuffSize,
	}
	rbConn.Connect()
	defer rbConn.Disconnect()

	// Handle DB connection for sitemap.xml
	spConn := db.SitemapDBConn{
		Ctx: context.Background(),
	}
	spConn.Connect()
	defer spConn.Disconnect()

	// Read and parse from .txt
	taskStrs, err := cm.ReadTaskStrsFromInput(cm.GlobalConfig.InputFileName)
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup

	// Group tasks
	workerTaskStrList := cm.GroupTasks(taskStrs)

	// Make workers
	var workerList []wk.Worker
	allResults := make(chan cm.TaskPrint, 500000) // all workers write to this
	for i, _ := range workerTaskStrList {         // see generalworker.go for details
		workerTasksHeap := &cm.HeapSlice{}
		heap.Init(workerTasksHeap)
		worker := wk.Worker{
			WorkerTasksHeap: *workerTasksHeap,
			AllResultsRef:   &allResults,
			RBConn:          &rbConn,
			SPConn:          &spConn,
		}
		for hostname, _ := range workerTaskStrList[i] {
			numUnits := len(workerTaskStrList[i][hostname])
			numUnits += 1 // +1 for possible sitemap retrieval
			numUnits += cm.GlobalConfig.SentinelPoliteness
			politeness := time.Duration(cm.GlobalConfig.ExpectedRuntime.Seconds() / float64(numUnits))
			// logic:
			//	if host healthy, then need to find sitemap, work is still distributed evenly
			//  if host healthy, then look for robots, mostly DB lookup, but still need at least one host check
			//	if host not healthy, then might have no work at all
			hp := cm.TaskStrsByHostname{
				Hostname:   hostname,
				Schedule:   time.Duration(rand.Float64()*float64(politeness)) * time.Second,
				Politeness: politeness * time.Second,
				TaskStrs:   make(chan string, len(workerTaskStrList[i][hostname])),
			}
			for k, _ := range workerTaskStrList[i][hostname] {
				hp.TaskStrs <- workerTaskStrList[i][hostname][k]
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
	// 	Wakes up periodically and flush all printable results from buffer to DB
	go func() {
		ticker := time.NewTicker(cm.GlobalConfig.DBWriteFrequency)
		for range ticker.C {
			curLen := len(taskPrints)
			writeBuff := make([]cm.TaskPrint, curLen)
			for i := 0; i < curLen; i++ {
				writeBuff[i] = <-taskPrints
			}

			go func() {
				doneWG, _ := dbConn.BulkWrite(writeBuff)

				fmt.Println("DB LOG:", curLen, " \t| doneWG:", doneWG)

				for i := 0; i < doneWG; i++ {
					wg.Done()
				}
			}()
		}
	}()

	wg.Wait()                                    // WG: +1 per task assigned to workers; -1 per task logged to DB
	time.Sleep(cm.GlobalConfig.DBWriteFrequency) // in case the program would end before db logging finishes
}
