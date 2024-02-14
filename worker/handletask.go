package worker

import (
	"fmt"
	"net/http"
	"net/url"
	cm "observerPolite/common"
	"sync"
)

func (wk *Worker) ErrorLog(task *cm.TaskPrint, err error) {
	if task.Retry.Retried {
		task.Retry.Err = cm.PrintErr(err)
	} else {
		task.Err = cm.PrintErr(err)
	}
}

func (wk *Worker) RespLog(task *cm.TaskPrint, resp *http.Response) {
	if task.Retry.Retried {
		task.Retry.Resp = cm.PrintResp(resp)
	} else {
		task.Resp = cm.PrintResp(resp)
	}
}

// HandleTask should check task struct first: use insights from sentinel
func (wk *Worker) HandleTask(task cm.TaskPrint, wg *sync.WaitGroup) {
	var response *http.Response
	var err error

	defer func() {
		wk.RespLog(&task, response)
		wk.ErrorLog(&task, err)
		*wk.AllResultsRef <- task
		(*wg).Done()
	}()

	// excluded hostnames
	parsedURL, _ := url.Parse(task.URL)
	if _, ok := cm.ExcludedList[parsedURL.Hostname()]; ok {
		err = fmt.Errorf("excluded hostname: %v", parsedURL.Hostname())
		return
	}

	// sentinel insights (.Err is empty for all retry tasks (retry is only patching to DB))
	if len(task.Err) >= 11 && task.Err[:11] == "HealthCheck" {
		if task.Err[13:16] == "DNS" || task.Err[13:16] == "TCP" {
			err = fmt.Errorf(task.Err)
			return
		}
		if task.Err[13:16] == "TLS" && parsedURL.Scheme == "https" {
			err = fmt.Errorf(task.Err)
			return
		}
	}
	if task.Err != "" {
		fmt.Println(task.Err)
	}
	task.Err = "" // skip all other errors

	var redirectChain *[]cm.Redirect
	if task.Retry.Retried == false {
		redirectChain = &task.RedirectChain
	} else {
		redirectChain = &task.Retry.RedirectChain
	}
	*redirectChain = (*redirectChain)[:0]

	response, err = wk.HTTPGET(parsedURL, redirectChain, &task.DNSRecords)
}
