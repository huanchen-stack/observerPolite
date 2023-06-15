package manager

import (
	cm "observerPolite/common"
	wk "observerPolite/worker"
)

type TaskManagerInterface interface {
	Start()
}

type TaskManager struct {
	AllTasks            chan cm.Task
	Feedback            chan cm.Task
	AllResults          chan cm.Task
	DedicatedMap        *map[string]*wk.DedicatedWorker
	SessionManagerAdmin *chan cm.AdminMsg
	Counter             int
}

func (tm *TaskManager) Start() {
	for {
		select {
		case task := <-tm.AllTasks:
			if worker, ok := (*tm.DedicatedMap)[task.Domain]; ok {
				worker.WorkerTasks <- task
			} else {
				panic("Worker for the domain is not found")
			}
		case feedback := <-tm.Feedback:
			if feedback.Resp == nil {
				if feedback.Retries > 0 {
					// to not immediately pass back to workers?
					feedback.Retries--
					tm.AllTasks <- feedback
				} else {
					tm.Counter--
					tm.AllResults <- feedback
				}
			} else {
				tm.Counter--
				tm.AllResults <- feedback
			}
		}

		// If all tasks are processed, send "Ready" message to workers
		if tm.Counter == 0 {
			for _, worker := range *tm.DedicatedMap {
				worker.WorkerAdmin <- cm.AdminMsg{Msg: "Ready"}
			}
			*tm.SessionManagerAdmin <- cm.AdminMsg{Msg: "Ready"}
			break
		}
	}
}
