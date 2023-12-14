package mongodb

import (
	"sync"
)

type FutureResult struct {
	resultChan chan interface{}
}

func (f *FutureResult) Await() interface{} {
	result := <-f.resultChan
	return result
}

type DBRequest struct {
	Key        string
	Value      string
	ResultChan chan interface{}
}

func GetOneAsync(key string, val string, readBatch *[]DBRequest, mutex *sync.Mutex) *FutureResult {
	resultChan := make(chan interface{}, 1)

	mutex.Lock()
	*readBatch = append(*readBatch, DBRequest{
		Key: key, Value: val,
		ResultChan: resultChan,
	})
	mutex.Unlock()

	return &FutureResult{resultChan: resultChan}
}
