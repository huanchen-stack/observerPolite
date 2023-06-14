package worker

import (
	"context"
	"net"
	"net/http"
	cm "observerPolite/common"
	mn "observerPolite/main"
	"time"
	// mgr "observerPolite/manager"
)

type DedicatedWorker struct {
	Domain string

	WorkerTasks chan cm.Task
	WorkerAdmin chan cm.AdminMsg

	TaskManagerFeedback *chan cm.Task
	TaskManagerAdmin    *chan cm.AdminMsg
	SessionManagerAdmin *chan cm.AdminMsg

	Connection *cm.Connection
	LastActive time.Time
	Politeness time.Duration
}

type DedicatedWorkerInterface interface {
	Start()
	HandleTask(task cm.Task)
	HandleAdminMsg(msg cm.AdminMsg)
}

func EstablishTLSConnection(task cm.Task) (*http.Client, error) {
	dailer := &net.Dialer{}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dailer.DialContext(ctx, network, task.IP+":443")
	}
	transport.MaxIdleConns = 0
	transport.IdleConnTimeout = 60 * time.Second

	client := &http.Client{
		Transport: transport,
		Timeout:   mn.GlobalConfig.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= mn.GlobalConfig.MaxRedirects {
				if mn.GlobalConfig.RedirectSucceed {
					return nil
				}
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	return client, nil
}

func SendGETRequest(client *http.Client, task cm.Task) (*http.Response, error) {
	req, err := http.NewRequest("GET", "https://"+task.Domain+task.Endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	return resp, err
}

func (dw *DedicatedWorker) HandleTask(task cm.Task) {
	// TLS handshake
	// 		Reuse connection whenever it's possible
	if dw.Connection.Client == nil || !dw.Connection.SessionAlive {
		mn.SemTLSConn <- struct{}{}
		client, err := EstablishTLSConnection(task)
		<-mn.SemTLSConn

		if err != nil {
			task.Retries--
			*dw.TaskManagerFeedback <- task
			return
		}

		dw.Connection.Client = client
		dw.Connection.SessionAlive = true
		*dw.SessionManagerAdmin <- cm.AdminMsg{Msg: dw.Domain, Value: 0}
	}

	// GET request
	mn.SemGETReq <- struct{}{}
	resp, err := SendGETRequest(dw.Connection.Client, task)
	<-mn.SemGETReq

	if err != nil {
		task.Retries--
		*dw.TaskManagerFeedback <- task
		return
	}

	task.Resp = resp
	*dw.TaskManagerFeedback <- task
}

func (dw *DedicatedWorker) HandleAdminMsg(msg cm.AdminMsg) bool {
	switch msg.Msg {
	case "Ready":
		// If "Ready" message is received, return true to signal to stop the worker!!!
		return true
	case "Clear":
		if time.Since(dw.LastActive) > time.Duration(msg.Value)*time.Second {
			if dw.Connection.SessionAlive && dw.Connection.Client != nil {
				dw.Connection.Client.CloseIdleConnections()
				dw.Connection.SessionAlive = false
			}
		}
	}
	// If message was not "Ready", return false to keep the worker running
	return false
}

func (dw *DedicatedWorker) Start() {
	for {
		select {
		case task := <-dw.WorkerTasks:
			dw.HandleTask(task)
			dw.LastActive = time.Now()
		case adminMsg := <-dw.WorkerAdmin:
			if dw.HandleAdminMsg(adminMsg) {
				return // Stop the worker!!!
			}
		}
	}
}