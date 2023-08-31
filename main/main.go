package main

import (
	"context"
	cm "observerPolite/common"
	db "observerPolite/mongodb"
	rm "observerPolite/retrymanager"
	wk "observerPolite/worker"
	"sync"
)

func main() {

	// Handle DB connection
	dbConn := db.DBConn{context.Background(), nil, nil, nil}
	dbConn.Connect()
	if cm.GlobalConfig.DBCollection != "" {
		dbConn.NewCollection(cm.GlobalConfig.DBCollection)
	}

	// Read and parse from .txt
	tasks, err := cm.ReadTasksFromInput(cm.GlobalConfig.InputFileName)
	if err != nil {
		panic(err)
	}

	// Add WG: +1 per task
	var wg sync.WaitGroup
	wg.Add(len(tasks))

	// Group and schedule tasks
	workerTaskList := cm.ScheduleTasks(tasks)

	// Make workers
	var workerList []wk.GeneralWorker
	allResults := make(chan cm.Task, 2)
	for i, _ := range workerTaskList {
		worker := wk.GeneralWorker{
			WorkerTasks:   make(chan cm.Task, cm.GlobalConfig.WorkerStress),
			AllResultsRef: &allResults,
		}
		for j, _ := range workerTaskList[i] {
			worker.WorkerTasks <- *workerTaskList[i][j]
		}
		close(worker.WorkerTasks)
		workerList = append(workerList, worker)
	}
	// NOW IT'S SAFE TO DISCARD THE ORI COPY!
	tasks = nil

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
	taskPrints := make(chan cm.TaskPrint, 100000)
	retryManager := rm.RetryManager{
		AllResults:    &allResults,
		TaskPrintsRef: &taskPrints,
	}
	go retryManager.Start()

	// GO! DB GO!
	go func() {
		for taskPrint := range taskPrints {
			wg.Done()
			dbConn.Insert(taskPrint)
		}
	}()

	wg.Wait()
	dbConn.Disconnect()
}
