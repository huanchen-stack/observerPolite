package mongodb

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	cm "observerPolite/common"
	"time"

	"github.com/yterajima/go-sitemap"
)

type SitemapDBConn struct {
	Ctx        context.Context
	Client     *mongo.Client
	Database   *mongo.Database
	Collection *mongo.Collection
	WriteChan  chan sitemap.Sitemap
}

func (sp *SitemapDBConn) GetCtx() context.Context          { return sp.Ctx }
func (sp *SitemapDBConn) GetCollection() *mongo.Collection { return sp.Collection }
func (sp *SitemapDBConn) GetBatchRead() []DBRequest        { return []DBRequest{} }

// Connect is mostly copied from mongodb website.
//
//	PANIC when error; caller does not need to handle error
func (sp *SitemapDBConn) Connect() {
	// Use the SetServerAPIOptions() method to set the Stable API version to 1
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	clientOptions := options.Client().ApplyURI(cm.GlobalConfig.DBURI)
	clientOptions.SetServerAPIOptions(serverAPI)
	clientOptions.SetMaxPoolSize(500)

	// Create a new client and connect to the server
	client, err := mongo.Connect(sp.Ctx, clientOptions)
	if err != nil {
		panic(err)
	}
	sp.Client = client

	// Send a ping to confirm a successful connection
	if err := client.Database("admin").RunCommand(sp.Ctx, bson.D{{"ping", 1}}).Err(); err != nil {
		panic(err)
	}
	fmt.Println("Pinged your deployment. You successfully connected to MongoDB! (sitemap)")

	database := client.Database("wikiPolite")
	sp.Database = database

	sp.NewCollection("sitemap")
	sp.CreateIndex("url")

	// GO! DB GO!
	// 	Wakes up periodically and flush all printable results from buffer to DB
	sp.WriteChan = make(chan sitemap.Sitemap, 50000)
	go func() {
		ticker := time.NewTicker(cm.GlobalConfig.DBWriteFrequency)
		for range ticker.C {
			curLen := len(sp.WriteChan)
			writeBuff := make([]sitemap.Sitemap, curLen)
			for i := 0; i < curLen; i++ {
				writeBuff[i] = <-sp.WriteChan
			}

			go func() {
				err := sp.BulkWrite(writeBuff)
				if err != nil {
					fmt.Println("do something (sp bulk write err)")
				}
				fmt.Println("--sitemap-- LOG:", curLen)
			}()
		}
	}()
}

func (sp *SitemapDBConn) NewCollection(name string) {
	currentTime := time.Now()
	timeString := currentTime.Format("20060102")
	if len(name) > 4 && name[:4] == "test" { // Provides extra sanity for testing
		timeString = ""
	}
	err := sp.Database.CreateCollection(sp.Ctx, name+timeString)
	if err != nil {
		log.Fatal(err)
	}
	collection := sp.Database.Collection(name + timeString)
	sp.Collection = collection
}

func (sp *SitemapDBConn) BulkWrite(dbDocs []sitemap.Sitemap) error {
	var writes []mongo.WriteModel
	for i, _ := range dbDocs {
		writes = append(writes, mongo.NewInsertOneModel().SetDocument(dbDocs[i]))
	}
	bulkWriteOptions := options.BulkWrite().SetOrdered(false)
	_, err := sp.Collection.BulkWrite(sp.Ctx, writes, bulkWriteOptions)
	if err != nil {
		// TODO: do something
	}
	return nil
}

// CreateIndex first check if index exists, then create one if it doesn't
//
//	This func is crucial for db lookup, which is as frequent as db write,
//	because comparisons to prev scan results are always needed for retry.
//	This func is always called at the beginning of the program, so PANIC when error occurs!
func (sp *SitemapDBConn) CreateIndex(idx string) {
	indexes := sp.Collection.Indexes()
	cursor, err := indexes.List(sp.Ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(sp.Ctx)

	indexExists := false
	for cursor.Next(sp.Ctx) {
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
		if _, err := indexes.CreateOne(sp.Ctx, indexModel); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Index on 'url' field created")
	} else {
		fmt.Println("Index on 'url' field already exists")
	}
}

func (sp *SitemapDBConn) Disconnect() {
	if err := sp.Client.Disconnect(sp.Ctx); err != nil {
		panic(err)
	}
}
