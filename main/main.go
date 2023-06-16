package main

import (
	"flag"
	"fmt"
	cm "observerPolite/common"
	mgr "observerPolite/manager"
	wk "observerPolite/worker"
	"time"
)

func Init() {
	flag.DurationVar(&cm.GlobalConfig.Timeout, "timeout", cm.GlobalConfig.Timeout, "TLS establishment timeout")
	flag.IntVar(&cm.GlobalConfig.MaxRedirects, "max-redirects", cm.GlobalConfig.MaxRedirects, "Maximum number of redirects")
	flag.BoolVar(&cm.GlobalConfig.RedirectSucceed, "redirect-succeed", cm.GlobalConfig.RedirectSucceed, "Mark all redirects as success")
	flag.IntVar(&cm.GlobalConfig.Retries, "retries", cm.GlobalConfig.Retries, "Maximum number of retries for each task")
	flag.IntVar(&cm.GlobalConfig.SessionLimit, "session-limit", cm.GlobalConfig.SessionLimit, "Maximum number of concurrent sessions")
	flag.IntVar(&cm.GlobalConfig.SessionIdleTimeout, "session-idle-timeout", cm.GlobalConfig.SessionIdleTimeout, "Threshold for killing idle sessions")
	flag.DurationVar(&cm.GlobalConfig.Politeness, "politeness", cm.GlobalConfig.Politeness, "Duration between scans of the same Domain")

	var TLSConnLimit, GETReqLimit int
	flag.IntVar(&TLSConnLimit, "tls-conn-limit", 1000, "Concurrency limit on TLS establishments")
	flag.IntVar(&GETReqLimit, "get-req-limit", cm.GlobalConfig.SessionLimit, "Concurrency limit on GET requests")

	flag.Parse()

	cm.SemTLSConn = make(chan struct{}, TLSConnLimit)
	cm.SemGETReq = make(chan struct{}, GETReqLimit)
}

func main() {
	Init()

	tasks, err := cm.ReadTasksFromCsv("input.csv")
	if err != nil {
		panic(err)
	}

	workerMap := make(map[string]*wk.DedicatedWorker)
	allResults := make(chan cm.Task, 50000)

	// Initialize TaskManager
	tm := &mgr.TaskManager{
		AllTasks:            make(chan cm.Task, 50000),
		Feedback:            make(chan cm.Task, 50000),
		AllResults:          &allResults,
		SessionManagerAdmin: nil, // set after session manager is created
		DedicatedMap:        &workerMap,
		Counter:             len(tasks),
	}

	// Initialize SessionManager
	sm := &mgr.SessionManager{
		AdminChan:    make(chan cm.AdminMsg, 50000),
		DedicatedMap: &workerMap,
		Counter:      len(tasks),
	}

	tm.SessionManagerAdmin = &sm.AdminChan

	for _, task := range tasks {
		// Check if a worker for this domain already exists
		if _, ok := workerMap[task.Domain]; !ok {
			worker := &wk.DedicatedWorker{
				Domain:              task.Domain,
				WorkerTasks:         make(chan cm.Task, 1000),
				WorkerAdmin:         make(chan cm.AdminMsg, 10),
				TaskManagerFeedback: &tm.Feedback,
				SessionManagerAdmin: &sm.AdminChan,
				Connection: cm.Connection{
					Client:       nil,
					SessionAlive: false,
				},
				LastActive: time.Now().Add(-cm.GlobalConfig.Politeness),
				Politeness: cm.GlobalConfig.Politeness,
			}
			workerMap[task.Domain] = worker
		}
	}

	fmt.Println("All initialized")
	for _, wk := range workerMap {
		go wk.Start()
	}
	go sm.Start()
	go tm.Start()

	go func() {
		for _, task := range tasks {
			fmt.Println(task)
			tm.AllTasks <- task
		}
	}()

	// Read from the AllResults channel of the TaskManager
	var counter = 0
	for result := range allResults {
		counter += 1
		// Handle results
		fmt.Println("bla", result)
		fmt.Println(counter)
		if counter == len(tasks) {
			break
		}
	}
}
