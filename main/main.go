package main

import (
	"encoding/json"
	"fmt"
	cm "observerPolite/common"
	wk "observerPolite/worker"
)

func main() {

	tasks, err := cm.ReadTasksFromInput("input.txt")
	if err != nil {
		panic(err)
	}

	workerMap := make(map[string]*wk.DedicatedWorker)
	allResults := make(chan cm.Task, 50000)

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

	for _, wk := range workerMap {
		go wk.Start()
	}

	fmt.Printf("[") // for formatting
	var counter = 0
	for result := range allResults {
		counter += 1

		jsonOut, _ := json.Marshal(cm.ResultPrint(result))
		fmt.Println(string(jsonOut))
		fmt.Printf(",") // for formatting

		if counter == len(tasks) {
			break
		}
	}
	fmt.Println("]") // for formatting
}
