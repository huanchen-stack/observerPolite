package worker

import (
	"container/heap"
	"math/rand"
	"net/url"
	cm "observerPolite/common"
	db "observerPolite/mongodb"
	"strings"
	"sync"
	"time"
)

type Worker struct {
	WorkerTasksHeap cm.HeapSlice       // for first try
	WorkerTasksStrs chan string        // for retry
	AllResultsRef   *chan cm.TaskPrint // write scan results, including DNS records
	RBConn          *db.RobotsDBConn   // read/write robots.txt
	SPConn          *db.SitemapDBConn  // write sitemap.xml -> txt
}

func (wk *Worker) FetchTask() (cm.TaskPrint, time.Duration) {
	taskStrsByHostname := heap.Pop(&wk.WorkerTasksHeap).(cm.TaskStrsByHostname)
	// taskStrsByHostname might be empty due to robots filter by sentinel: fetch again (and again)
	//	prior logic accommodate as well (make program cleaner)
	for len(taskStrsByHostname.TaskStrs) == 0 {
		if len(wk.WorkerTasksHeap) == 0 {
			return cm.TaskPrint{}, 0
		}
		taskStrsByHostname = heap.Pop(&wk.WorkerTasksHeap).(cm.TaskStrsByHostname)
	}
	defer heap.Push(&wk.WorkerTasksHeap, taskStrsByHostname)
	curTaskSchedule := taskStrsByHostname.Schedule

	if !taskStrsByHostname.Sentinel.Handled {
		go func() {
			randFactor := rand.Intn(cm.GlobalConfig.SentinelPoliteness)
			time.Sleep(time.Duration(randFactor) * taskStrsByHostname.Politeness)
			wk.HandleSentinel(&taskStrsByHostname)
		}()
		taskStrsByHostname.Schedule += time.Duration(cm.GlobalConfig.SentinelPoliteness) * taskStrsByHostname.Politeness
		taskStrsByHostname.Sentinel.Handled = true
		return cm.TaskPrint{}, 0
	}

	taskStr := <-taskStrsByHostname.TaskStrs
	if taskStr == "sitemap" {
		go wk.FetchSitemap("https", taskStrsByHostname.Hostname)
		taskStrsByHostname.Schedule += taskStrsByHostname.Politeness
		return cm.TaskPrint{}, 0
	}

	line := strings.TrimSpace(taskStr)
	strL := strings.Split(line, ",")
	URL := strings.TrimSpace(strL[0])
	parsedURL, _ := url.Parse(URL)

	task := cm.TaskPrint{
		Source:   strings.TrimSpace(strL[1]),
		Hostname: parsedURL.Hostname(),
		URL:      URL,
	}
	// benefits from handling sentinel: DNS+ pre warn & DNS records sentinel version
	for i, _ := range taskStrsByHostname.Sentinel.DNSRecords {
		task.DNSRecords = append(task.DNSRecords, taskStrsByHostname.Sentinel.DNSRecords[i])
	}
	if taskStrsByHostname.Sentinel.HealthErrMsg != "" {
		task.Err = "HealthCheck: " + taskStrsByHostname.Sentinel.HealthErrMsg
	}

	taskStrsByHostname.Schedule += taskStrsByHostname.Politeness
	return task, curTaskSchedule
}

func (wk *Worker) Start() {
	var wg sync.WaitGroup
	start := time.Now()

	for len(wk.WorkerTasksHeap) != 0 {
		task, schedule := wk.FetchTask()
		if task.URL == "" {
			continue
		}
		// if task.URL is empty, then it's either sentinel or sitemap, doesn't count as a task
		//	if not continued, then it must be a regular task
		time.Sleep(schedule - time.Since(start))
		wg.Add(1)
		go wk.HandleTask(task, &wg)
	}

	wg.Wait()
}

// StartRetry is a special start funtion made for retry. This func is called by retry manager only.
//
//	If a worker is started with this func, there's no need to create tasks from strings or perform robots check.
func (wk *Worker) StartRetry(politeness time.Duration) {
	var wg sync.WaitGroup

	for urlStr := range wk.WorkerTasksStrs {
		tmpTask := &cm.TaskPrint{
			URL: urlStr,
		}
		tmpTask.Retry.Retried = true

		wg.Add(1)
		go wk.HandleTask(*tmpTask, &wg)

		time.Sleep(politeness)
	}
	wg.Wait()
}
