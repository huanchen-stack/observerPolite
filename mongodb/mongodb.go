package mongodb

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	cm "observerPolite/common"
	"sync"
	"time"
)

type DBConn struct {
	Ctx        context.Context
	Client     *mongo.Client
	Database   *mongo.Database
	Collection *mongo.Collection
	ReadBatch  []DBRequest
	Mutex      sync.Mutex
}

func (db *DBConn) GetCtx() context.Context          { return db.Ctx }
func (db *DBConn) GetCollection() *mongo.Collection { return db.Collection }
func (db *DBConn) GetBatchRead() *[]DBRequest       { return &db.ReadBatch }
func (db *DBConn) GetBatchMutex() *sync.Mutex       { return &db.Mutex }

// Connect is mostly copied from mongodb website.
//
//	PANIC when error; caller does not need to handle error
func (db *DBConn) Connect() {
	// Use the SetServerAPIOptions() method to set the Stable API version to 1
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(cm.GlobalConfig.DBURI).SetServerAPIOptions(serverAPI)

	// Create a new client and connect to the server
	client, err := mongo.Connect(db.Ctx, opts)
	if err != nil {
		panic(err)
	}
	db.Client = client

	// Send a ping to confirm a successful connection
	if err := client.Database("admin").RunCommand(db.Ctx, bson.D{{"ping", 1}}).Err(); err != nil {
		panic(err)
	}
	fmt.Println("Pinged your deployment. You successfully connected to MongoDB!")

	database := client.Database("wikiPolite")
	db.Database = database
}

func (db *DBConn) NewCollection(name string) {
	currentTime := time.Now()
	timeString := currentTime.Format("01022006")
	if len(name) > 4 && name[:4] == "test" { // Provides extra sanity for testing
		timeString = ""
	}
	err := db.Database.CreateCollection(db.Ctx, name+timeString)
	if err != nil {
		log.Fatal(err)
	}
	collection := db.Database.Collection(name + timeString)
	db.Collection = collection
}

//func (db *DBConn) BulkRead(key string, vals []string) []cm.TaskPrint {
//	filter := bson.M{key: bson.M{"$in": vals}}
//	cursor, err := db.Collection.Find(db.Ctx, filter)
//	if err != nil {
//		if err == mongo.ErrNoDocuments {
//			return make([]cm.TaskPrint, 0)
//		} else {
//			fmt.Println("DB Bulk Read Err") // TODO: fix this
//			return make([]cm.TaskPrint, 0)
//		}
//	}
//	defer cursor.Close(db.Ctx)
//
//	var results []cm.TaskPrint
//	if err := cursor.All(db.Ctx, &results); err != nil {
//		fmt.Println("DB Bulk Read Decode Err")
//	}
//
//	return results
//}

func (db *DBConn) BulkWrite(dbDocs []cm.TaskPrint) (int, error) {
	if !cm.GlobalConfig.DBlogging {
		panic("app won't work without dblogging for now!")
		for _, dbDoc := range dbDocs {
			fmt.Println(dbDoc.URL)
		}
		return 0, nil
	}

	var writes []mongo.WriteModel
	doneWG := 0
	for i, dbDoc := range dbDocs {
		if dbDoc.Retry.Retried {
			writes = append(writes, mongo.NewUpdateOneModel().SetFilter(
				bson.M{"url": dbDoc.URL},
			).SetUpdate(
				bson.M{"$set": bson.M{"retry": dbDoc.Retry}},
			))
			doneWG++
		} else {
			if !dbDoc.NeedsRetry {
				doneWG++
			} else {
			}
			writes = append(writes, mongo.NewInsertOneModel().SetDocument(dbDocs[i]))
		}
	}
	bulkWriteOptions := options.BulkWrite().SetOrdered(false)
	_, err := db.Collection.BulkWrite(db.Ctx, writes, bulkWriteOptions)
	if err != nil {
		// TODO: do something
	}
	return doneWG, nil
}

// CreateIndex first check if index exists, then create one if it doesn't
//
//	This func is crucial for db lookup, which is as frequent as db write,
//	because comparisons to prev scan results are always needed for retry.
//	This func is always called at the beginning of the program, so PANIC when error occurs!
func (db *DBConn) CreateIndex(idx string) {
	indexes := db.Collection.Indexes()
	cursor, err := indexes.List(db.Ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(db.Ctx)

	indexExists := false
	for cursor.Next(db.Ctx) {
		var index map[string]interface{}
		if err := cursor.Decode(&index); err != nil {
			log.Fatal(err)
		}
		if key, ok := index["key"]; ok {
			if keyMap, ok := key.(map[string]interface{}); ok {
				if _, ok := keyMap[idx]; ok {
					indexExists = true
					break
				}
			}
		}
	}

	if !indexExists {
		indexModel := mongo.IndexModel{
			Keys: map[string]interface{}{idx: 1}, // Ascending index
		}
		if _, err := indexes.CreateOne(db.Ctx, indexModel); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Index on 'url' field created")
	} else {
		fmt.Println("Index on 'url' field already exists")
	}
}

func (db *DBConn) GetOne(key string, val string) cm.TaskPrint {
	filter := bson.M{key: val}
	var result cm.TaskPrint
	err := db.Collection.FindOne(db.Ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return cm.TaskPrint{
				URL: "",
			}
		} else {
			fmt.Println("do something") // TODO: fix this
		}
	}
	return result
}

//func (db *DBConn) BatchProcessor() {
//	ticker := time.NewTicker(cm.GlobalConfig.DBWriteFrequency)
//
//	for range ticker.C {
//		fmt.Println("len db readbatch: ", len(db.ReadBatch))
//		db.Mutex.Lock()
//		currentBatch := make([]string, len(db.ReadBatch))
//		//currentBatchStatusMap := make(map[string]bool, 0)
//		currentBatchChanMap := make(map[string]chan interface{}, 0)
//		for i, _ := range db.ReadBatch {
//			currentBatch[i] = db.ReadBatch[i].Value
//			currentBatchChanMap[db.ReadBatch[i].Value] = db.ReadBatch[i].ResultChan
//		}
//		if len(currentBatchChanMap) != len(currentBatch) {
//			panic("duplicate url input!!!")
//		}
//		db.ReadBatch = db.ReadBatch[:0]
//		db.Mutex.Unlock()
//
//		if len(currentBatch) == 0 {
//			continue
//		}
//		results := BulkRead[cm.TaskPrint](db, "url", currentBatch)
//
//		for i, _ := range results {
//			currentBatchChanMap[results[i].URL] <- results[i]
//			delete(currentBatchChanMap, results[i].URL)
//		}
//		for _, v := range currentBatchChanMap {
//			v <- cm.TaskPrint{}
//		}
//	}
//}

func (db *DBConn) GetOneAsync(key string, val string) cm.TaskPrint {
	resultFuture := GetOneAsync(key, val, &db.ReadBatch, &db.Mutex)
	//time.Sleep(10 * time.Second)
	result := resultFuture.Await()
	typedResult, ok := result.(cm.TaskPrint)
	if !ok {
		fmt.Println("db GetOneAsync type cast error")
	}
	return typedResult
}

func (db *DBConn) Disconnect() {
	if err := db.Client.Disconnect(db.Ctx); err != nil {
		panic(err)
	}
}
