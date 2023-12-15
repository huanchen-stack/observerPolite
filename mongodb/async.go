package mongodb

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	cm "observerPolite/common"
	"reflect"
	"sync"
	"time"
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

type DBDoc interface {
	GetURL() string
}

type DBAccess interface {
	GetCtx() context.Context
	GetCollection() *mongo.Collection
	GetBatchRead() *[]DBRequest
	GetBatchMutex() *sync.Mutex
}

func BulkRead[T DBDoc](dbAccess DBAccess, key string, vals []string) []T {
	collection := dbAccess.GetCollection()
	ctx := dbAccess.GetCtx()
	filter := bson.M{key: bson.M{"$in": vals}}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return make([]T, 0)
		} else {
			fmt.Println("DB Bulk Read Err") // TODO: fix this
			return make([]T, 0)
		}
	}
	defer cursor.Close(ctx)

	var results []T
	if err := cursor.All(ctx, &results); err != nil {
		fmt.Println("DB Bulk Read Decode Err")
	}

	return results
}

func BatchProcessor[T DBDoc](dbAccess DBAccess) {
	ticker := time.NewTicker(cm.GlobalConfig.DBWriteFrequency)

	for range ticker.C {
		batch := dbAccess.GetBatchRead()
		mutex := dbAccess.GetBatchMutex()
		var tmp T
		fmt.Println(reflect.TypeOf(tmp), "len readbatch: ", len(*batch))
		mutex.Lock()
		currentBatch := make([]string, len(*batch))
		//currentBatchStatusMap := make(map[string]bool, 0)
		currentBatchChanMap := make(map[string]chan interface{}, 0)
		for i, _ := range *batch {
			currentBatch[i] = (*batch)[i].Value
			currentBatchChanMap[(*batch)[i].Value] = (*batch)[i].ResultChan
		}
		if len(currentBatchChanMap) != len(currentBatch) {
			panic("duplicate url input!!!")
		}
		*batch = (*batch)[:0]
		mutex.Unlock()

		if len(currentBatch) == 0 {
			continue
		}
		results := BulkRead[T](dbAccess, "url", currentBatch)

		for i, _ := range results {
			currentBatchChanMap[results[i].GetURL()] <- results[i]
			delete(currentBatchChanMap, results[i].GetURL())
		}
		for _, v := range currentBatchChanMap {
			v <- tmp
		}
	}
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
