package manager

import (
	cm "observerPolite/common"
	wk "observerPolite/worker"
)

type SessionManager struct {
	AdminChannel chan cm.AdminMsg
	DedicatedMap *map[string]*wk.DedicatedWorker
	Counter      int
}

type SessionManagerInterface interface {
	Start()
}

func (sm *SessionManager) Start() {
	for {
		select {
		case adminMsg := <-sm.AdminChannel:
			switch adminMsg.Msg {
			case "Ready":
				// If "Ready" message is received, signal to stop the manager!!!
				return
			default:
				sm.Counter++
				if sm.Counter >= cm.GlobalConfig.SessionLimit {
					clearMsg := cm.AdminMsg{Msg: "Clear", Value: cm.GlobalConfig.SessionIdleTimeout}
					for _, worker := range *sm.DedicatedMap {
						worker.WorkerAdmin <- clearMsg
					}
					sm.Counter = 0
				}
			}
		}
	}
}
