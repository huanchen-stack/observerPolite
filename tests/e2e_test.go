package tests

import (
	"fmt"
	cm "observerPolite/common"
	"os"
	"testing"
	"time"
)

func TestDedup(t *testing.T) {
	cm.GlobalConfig.InputFileName = "inputs/dedup.txt"
	cm.GlobalConfig.DBCollection = "test_E2E"
	cm.GlobalConfig.ExpectedRuntime = 10 * time.Second
	cm.GlobalConfig.RetryPoliteness = 3 * time.Second

	err := execTimeThresh(60*time.Second, true)
	if err != nil {
		t.Fatal(err)
	}

	docChan := make(chan cm.TaskPrint)
	go dbQuery("test_E2E", docChan, true)

	count := 0
	for task := range docChan {
		count++
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(task.URL)
		if task.Resp.StatusCode != 200 {
			t.Fatal("statuscode != 200")
		}
		if task.Retry.Retried {
			t.Fatal("retried when shouldn't")
		}
		// ...
	}
	if count != 7 {
		t.Fatal("dedup failed!")
	}
}

func TestConnErrDNS(t *testing.T) {
	cm.GlobalConfig.InputFileName = "inputs/connErrDNS.txt"
	cm.GlobalConfig.DBCollection = "test_E2E"
	cm.GlobalConfig.ExpectedRuntime = 5 * time.Second
	cm.GlobalConfig.RetryPoliteness = 5 * time.Second
	cm.GlobalConfig.Timeout = 10 * time.Second

	err := execTimeThresh(60*time.Second, true)
	if err != nil {
		t.Fatal(err)
	}

	docChan := make(chan cm.TaskPrint)
	go dbQuery("test_E2E", docChan, true)

	count := 0
	for task := range docChan {
		count++
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(task.URL)
		if task.Resp.StatusCode == 200 {
			t.Fatal("statuscode == 200 when it shouldn't")
		}
		if len(task.Err) < 3 || task.Err[:3] != "DNS" {
			t.Fatalf("DNS error message not thrown: %s", task.Err)
		}
		if task.Retry.Retried {
			t.Fatal("retried when shouldn't")
		}
		// ...
	}
	if count != 3 {
		t.Fatal("dblog failed!")
	}
}

func TestConnErrTCP(t *testing.T) {
	cm.GlobalConfig.InputFileName = "inputs/connErrTCP.txt"
	cm.GlobalConfig.DBCollection = "test_E2E"
	cm.GlobalConfig.ExpectedRuntime = 5 * time.Second
	cm.GlobalConfig.RetryPoliteness = 5 * time.Second
	cm.GlobalConfig.Timeout = 10 * time.Second

	err := execTimeThresh(30*time.Second, true)
	if err != nil {
		t.Fatal(err)
	}

	docChan := make(chan cm.TaskPrint)
	go dbQuery("test_E2E", docChan, true)

	count := 0
	for task := range docChan {
		count++
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(task.URL)
		if task.Resp.StatusCode == 200 {
			t.Fatal("statuscode == 200 when it shouldn't")
		}
		if len(task.Err) < 3 || task.Err[:3] != "TCP" {
			t.Fatal("TCP error message not thrown")
		}
		if task.Retry.Retried {
			t.Fatal("retried when shouldn't")
		}
		// ...
	}
	if count != 3 {
		t.Fatal("dblog failed!")
	}
}

func TestConnErrTLS(t *testing.T) {
	cm.GlobalConfig.InputFileName = "inputs/connErrTLS.txt"
	cm.GlobalConfig.DBCollection = "test_E2E"
	cm.GlobalConfig.ExpectedRuntime = 5 * time.Second
	cm.GlobalConfig.RetryPoliteness = 5 * time.Second
	cm.GlobalConfig.Timeout = 10 * time.Second

	err := execTimeThresh(60*time.Second, true)
	if err != nil {
		t.Fatal(err)
	}

	docChan := make(chan cm.TaskPrint)
	go dbQuery("test_E2E", docChan, true)

	count := 0
	for task := range docChan {
		count++
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(task.URL)
		if task.Resp.StatusCode == 200 {
			t.Fatal("statuscode == 200 when it shouldn't")
		}
		if len(task.Err) < 3 || task.Err[:3] != "TLS" {
			t.Fatalf("TLS error message not thrown: %s", task.Err)
		}
		if task.Retry.Retried {
			t.Fatal("retried when shouldn't")
		}
		// ...
	}
	if count != 3 {
		t.Fatal("dblog failed!")
	}
}

func TestErrHTTP(t *testing.T) {
	cm.GlobalConfig.InputFileName = "inputs/errHTTP.txt"
	cm.GlobalConfig.DBCollection = "test_E2E"
	cm.GlobalConfig.ExpectedRuntime = 5 * time.Second
	cm.GlobalConfig.RetryPoliteness = 5 * time.Second
	cm.GlobalConfig.Timeout = 10 * time.Second

	err := execTimeThresh(30*time.Second, false)
	if err != nil {
		t.Fatal(err)
	}

	docChan := make(chan cm.TaskPrint)
	go dbQuery("test_E2E", docChan, true)

	count := 0
	for task := range docChan {
		count++
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(task.URL)
		if task.Resp.StatusCode == 200 {
			t.Fatal("statuscode == 200 when it shouldn't")
		}
		if len(task.Err) < 4 || task.Err[:4] != "HTTP" {
			t.Fatalf("HTTP error message not thrown: %s", task.Err)
		}
		if task.Retry.Retried {
			t.Fatal("retried when shouldn't")
		}
		// ...
	}
	if count != 3 {
		t.Fatal("dblog failed!")
	}
}

func TestRetry(t *testing.T) {
	// test worker stress and db write along the way
	cm.GlobalConfig.InputFileName = "inputs/retryAllSc.txt"
	cm.GlobalConfig.DBCollection = "test_E2E_comp1"
	cm.GlobalConfig.ExpectedRuntime = 10 * time.Second
	cm.GlobalConfig.RetryPoliteness = 5 * time.Second
	cm.GlobalConfig.WorkerStress = 5
	cm.GlobalConfig.DBWriteFrequency = 5 * time.Second

	err := execTimeThresh(60*time.Second, true)
	if err != nil {
		t.Fatal(err)
	}

	docChan := make(chan cm.TaskPrint)
	go dbQuery("test_E2E_comp1", docChan, false)

	count := 0
	for task := range docChan {
		count++
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(task.URL)
		if task.Resp.StatusCode != 200 {
			t.Fatal("statuscode != 200")
		}
		if task.Retry.Retried {
			t.Fatal("retried when shouldn't")
		}
		// ...
	}
	if count != 16 {
		t.Fatal("db log failed!")
	}

	dbMess("test_E2E_comp1", false)
	cm.GlobalConfig.DBCollection = "test_E2E_comp2"
	cm.GlobalConfig.DBCollectionComp = "test_E2E_comp1"

	err = execTimeThresh(90*time.Second, true)
	if err != nil {
		t.Fatal(err)
	}

	docChan = make(chan cm.TaskPrint)
	go dbQuery("test_E2E_comp2", docChan, false)

	count = 0
	for task := range docChan {
		count++
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(task.URL)
		if task.Resp.StatusCode != 200 {
			t.Fatal("statuscode != 200")
		}
		if !task.Retry.Retried {
			t.Fatal("not retrying when need")
		}
		// ...
	}
	if count != 16 {
		t.Fatal("db log failed!")
	}

	dbMess("test_E2E_comp2", true)
	cm.GlobalConfig.DBCollection = "test_E2E_comp3"
	cm.GlobalConfig.DBCollectionComp = "test_E2E_comp2"

	err = execTimeThresh(90*time.Second, true)
	if err != nil {
		t.Fatal(err)
	}

	docChan = make(chan cm.TaskPrint)
	go dbQuery("test_E2E_comp3", docChan, false)

	count = 0
	retried := 0
	for task := range docChan {
		count++
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(task.URL)
		if task.Resp.StatusCode != 200 {
			t.Fatal("statuscode != 200")
		}
		if !task.Retry.Retried {
			retried++
		}
		// ...
	}
	if count != 16 {
		t.Fatal("db log failed!")
	}
	if retried != 8 {
		t.Fatal("retry comparison not handled properly")
	}
}

func TestRobotsDisallow(t *testing.T) {
	// also tests robots fetching and robots db logging
	cm.GlobalConfig.InputFileName = "inputs/robotsDisallow.txt"
	cm.GlobalConfig.DBCollection = "test_E2E"
	cm.GlobalConfig.ExpectedRuntime = 5 * time.Second
	cm.GlobalConfig.RetryPoliteness = 5 * time.Second
	cm.GlobalConfig.Timeout = 10 * time.Second
	cm.GlobalConfig.RobotsBuffSize = 2

	err := execTimeThresh(20*time.Second, false)
	if err != nil {
		t.Fatal(err)
	}

	docChan := make(chan cm.TaskPrint)
	go dbQuery("test_E2E", docChan, true)

	count := 0
	for task := range docChan {
		count++
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(task.URL)
		if task.Resp.StatusCode == 200 {
			t.Fatal("statuscode == 200 when it shouldn't")
		}
		if len(task.Err) < 4 || task.Err[:4] != "path" {
			t.Fatalf("path error message not thrown: %s", task.Err)
		}
		if task.Retry.Retried {
			t.Fatal("retried when shouldn't")
		}
		// ...
	}
	if count != 10 {
		t.Fatal("dblog failed!")
	}
}

func TestMain(m *testing.M) {
	exitVal := m.Run()
	os.Exit(exitVal)
}
