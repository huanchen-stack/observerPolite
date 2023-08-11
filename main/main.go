package main

import (
	"context"
	cm "observerPolite/common"
	db "observerPolite/mongodb"
	wk "observerPolite/worker"
)

func main() {

	dbConn := db.DBConn{context.Background(), nil, nil, nil}
	dbConn.Connect()

	tasks, err := cm.ReadTasksFromInput(cm.GlobalConfig.InputFileName)
	if err != nil {
		panic(err)
	}

	workerMap := make(map[string]*wk.DedicatedWorker)
	allResults := make(chan cm.Task, 100000)

	for _, task := range tasks {
		// Check if a worker for this domain already exists
		if _, ok := workerMap[task.Domain]; !ok {
			worker := &wk.DedicatedWorker{
				Domain:        task.Domain,
				WorkerTasks:   make(chan cm.Task, 1000),
				AllResultsRef: &allResults,
			}
			workerMap[task.Domain] = worker
		}
	}

	for _, task := range tasks {
		workerMap[task.Domain].WorkerTasks <- task
	}

	for _, wkr := range workerMap {
		go wkr.Start()
	}

	var counter = 0
	for result := range allResults {
		counter += 1

		dbConn.Insert(cm.PrintTask(result))

		if counter == len(tasks) {
			break
		}
	}

	dbConn.Disconnect()
}
