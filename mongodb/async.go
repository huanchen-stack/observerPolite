package mongodb

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	cm "observerPolite/common"
	"reflect"
	"time"
	"unsafe"
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
	GetBatchRead() []DBRequest
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
		var tmp T
		fmt.Println("[", reflect.TypeOf(tmp), "] len read batch: ", len(batch))

		go func() {
			currentBatchChanMap := make(map[string][]chan interface{}, 0) // concurrent robotstxt lookup to the same host
			for i, _ := range batch {
				currentBatchChanMap[batch[i].Value] = append(currentBatchChanMap[batch[i].Value], batch[i].ResultChan)
			}

			currentBatch := make([]string, len(currentBatchChanMap))
			idx := 0
			for k, _ := range currentBatchChanMap {
				currentBatch[idx] = k
				idx += 1
			}
			if len(currentBatch) == 0 {
				return
			}
			start := time.Now()

			results := BulkRead[T](dbAccess, "url", currentBatch)

			size_ := 0
			for i, _ := range results {
				size_ += int(unsafe.Sizeof(results[i]))
			}
			fmt.Println("[", reflect.TypeOf(tmp), "][ bulk read ] (len:", len(currentBatch), ") (size:", size_, ") Time passed since start", time.Since(start))

			for i, _ := range results {
				for j, _ := range currentBatchChanMap[results[i].GetURL()] {
					currentBatchChanMap[results[i].GetURL()][j] <- results[i]
				}
				delete(currentBatchChanMap, results[i].GetURL())
			}
			for _, v := range currentBatchChanMap {
				var tmp T
				for i, _ := range v {
					v[i] <- tmp
				}
			}
		}()

	}
}

func GetOneAsync(key string, val string, readBatchChan chan DBRequest) *FutureResult {
	resultChan := make(chan interface{}, 1)
	readBatchChan <- DBRequest{
		Key: key, Value: val,
		ResultChan: resultChan,
	}
	return &FutureResult{resultChan: resultChan}
}
