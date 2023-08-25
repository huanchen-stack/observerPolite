package main

import (
	"context"
	"fmt"
	cm "observerPolite/common"
	hb "observerPolite/heartbeat"
	db "observerPolite/mongodb"
	rm "observerPolite/retrymanager"
	wk "observerPolite/worker"
	"sync"
	"time"
)

func main() {

	dbConn := db.DBConn{context.Background(), nil, nil, nil}
	dbConn.Connect()
	if cm.GlobalConfig.DBCollection != "" {
		dbConn.NewCollection(cm.GlobalConfig.DBCollection)
	}

	tasks, err := cm.ReadTasksFromInput(cm.GlobalConfig.InputFileName)
	if err != nil {
		panic(err)
	}

	workerMap := make(map[string]*wk.DedicatedWorker)
	allResults := make(chan cm.Task, 2)

	var wg sync.WaitGroup

	for _, task := range tasks {
		// Check if a worker for this domain already exists
		if _, ok := workerMap[task.Domain]; !ok {
			worker := &wk.DedicatedWorker{
				Domain:        task.Domain,
				WorkerTasks:   make(chan cm.Task, 1000),
				AllResultsRef: &allResults,
				WG:            &wg,
			}
			workerMap[task.Domain] = worker
		}
	}

	for _, task := range tasks {
		workerMap[task.Domain].WorkerTasks <- task
	}

	hb := hb.Heartbeat{DBConn: &dbConn}
	go hb.Start()
	wg.Add(len(workerMap))
	for _, wkr := range workerMap {
		close(wkr.WorkerTasks)
		go wkr.Start()
	}

	retryManager := rm.RetryManager{
		BuffTasks: make(chan cm.TaskPrint, 100000),
	}
	go retryManager.Start()

	go func() {
		for result := range allResults {
			taskPrint := cm.PrintTask(result)
			retryManager.BuffTasks <- taskPrint
			dbConn.Insert(taskPrint)
		}
	}()

	wg.Wait()
	fmt.Println("closing allresults channel ...")
	close(allResults)

	time.Sleep(cm.GlobalConfig.RetryPoliteness)
	close(retryManager.BuffTasks)
	fmt.Println("closing retrymanager.bufftasks channel ...")

	dbConn.Disconnect()
}
