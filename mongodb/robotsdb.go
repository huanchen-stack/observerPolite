package mongodb

import (
	"context"
	"fmt"
	"github.com/temoto/robotstxt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"net/url"
	cm "observerPolite/common"
	"sync"
	"time"
)

type RobotsDBConn struct {
	Ctx         context.Context
	Client      *mongo.Client
	Collection  *mongo.Collection
	ExpireMap   map[string]time.Time
	CacheMap    map[string]*robotstxt.Group // not an actual cache, just naturally here because db logging is in batches
	RespBodyMap map[string]string
	CacheSize   int
	mutex       sync.RWMutex
}

type RobotsDBConnInterface interface {
	Get(hostname string) *robotstxt.Group
	Add(hostname string, expiration time.Time, group *robotstxt.Group) error

	Connect()
	FetchOne(hostname string) *robotstxt.Group
	BulkWrite(buff []cm.RobotsPrint)
	Disconnect()
	Clean()
}

// Connect is similar to mongodb::Connect, but additionally this func
//
//	check exist/create collection, whose name is always "robotstxt"
//	initialize buffer/(cache) for db bulk write
func (rb *RobotsDBConn) Connect() {
	rb.Ctx = context.Background()

	// Use the SetServerAPIOptions() method to set the Stable API version to 1
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(cm.GlobalConfig.DBURI).SetServerAPIOptions(serverAPI)

	// Create a new client and connect to the server
	client, err := mongo.Connect(rb.Ctx, opts)
	if err != nil {
		panic(err)
	}
	rb.Client = client

	// Send a ping to confirm a successful connection
	if err := client.Database("admin").RunCommand(rb.Ctx, bson.D{{"ping", 1}}).Err(); err != nil {
		panic(err)
	}
	fmt.Println("Pinged your deployment. You successfully connected to MongoDB! (robots)")

	database := client.Database("wikiPolite")

	collections, err := database.ListCollectionNames(rb.Ctx, bson.M{})
	if err != nil {
		panic(err)
	}

	var exists bool
	for _, name := range collections {
		if name == "robotstxt" {
			exists = true
		}
	}
	if !exists {
		err := database.CreateCollection(rb.Ctx, "robotstxt")
		if err != nil {
			panic(err)
		}
	}
	rb.Collection = database.Collection("robotstxt")

	// cache mech init
	rb.ExpireMap = make(map[string]time.Time, 0) // need size to indicate how full the map is
	rb.CacheMap = make(map[string]*robotstxt.Group, 0)
	rb.RespBodyMap = make(map[string]string, 0)
}

// FetchOne fetched robotsPrint from db and extract robotstxt.Group when possible.
//
//	If entry is not found in db, or the response expired, return nil to notify Caller (GET).
//	This func is not directly called by user. (should be private but now public for debugging)
func (rb *RobotsDBConn) FetchOne(url_ string) *robotstxt.Group {
	filter := bson.M{"url": url_}
	var robotsPrint cm.RobotsPrint
	err := rb.Collection.FindOne(rb.Ctx, filter).Decode(&robotsPrint)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			//fmt.Println("not found in db")
			return nil // notify there's no cache in db
		} else {
			log.Fatal(err)
		}
	}
	//fmt.Println("found in db")
	if robotsPrint.Expiration.After(time.Now()) {
		robotsData, fromStrErr := robotstxt.FromString(robotsPrint.RespBodyStr)
		if fromStrErr != nil {
			return &robotstxt.Group{} // TODO: decide on this
		}
		return robotsData.FindGroup("*")
	} else {
		_, err := rb.Collection.DeleteOne(rb.Ctx, filter)
		if err != nil {
			log.Fatal(err)
		}
		return nil
	}
}

func (rb *RobotsDBConn) BulkWrite(buff []cm.RobotsPrint) {
	var writes []mongo.WriteModel
	for i, _ := range buff {
		writes = append(writes, mongo.NewInsertOneModel().SetDocument(buff[i]))
	}
	bulkWriteOptions := options.BulkWrite().SetOrdered(false)
	_, err := rb.Collection.BulkWrite(rb.Ctx, writes, bulkWriteOptions)
	if err != nil {
		fmt.Println("do something")
	}
}

func (rb *RobotsDBConn) Clean() {} // not implemented

// Get is the method that users should be using
//
//	Get first check the natual cache (db logging is in batches, that batch is a natual cache)
//	Get then call FetchOne to check if entry exists in db.
func (rb *RobotsDBConn) Get(scheme string, hostname string) *robotstxt.Group {
	parsedURL := url.URL{
		Scheme: scheme,
		Host:   hostname,
		Path:   "/robots.txt",
	}

	rb.mutex.Lock()
	if group, ok := rb.CacheMap[parsedURL.String()]; ok {
		//fmt.Println("robot cache HIT")
		if rb.ExpireMap[parsedURL.String()].After(time.Now()) {
			rb.mutex.Unlock()
			return group
		} else {
			//fmt.Println("robot cache HIT but EXPIRED")
			delete(rb.ExpireMap, parsedURL.String()) // can't write to db since this already expired
			delete(rb.CacheMap, parsedURL.String())
			delete(rb.RespBodyMap, parsedURL.String())
			rb.mutex.Unlock()
			return nil
		}
	}
	rb.mutex.Unlock()

	//fmt.Println("robot cache MISS, checking in db...")
	return rb.FetchOne(parsedURL.String())
}

func (rb *RobotsDBConn) Add(scheme string, hostname string, expiration time.Time, group *robotstxt.Group, respBodyStr string) {
	parsedURL := url.URL{
		Scheme: scheme,
		Host:   hostname,
		Path:   "/robots.txt",
	}

	rb.mutex.Lock()
	rb.ExpireMap[parsedURL.String()] = expiration
	rb.CacheMap[parsedURL.String()] = group
	rb.RespBodyMap[parsedURL.String()] = respBodyStr

	var writeBuff []cm.RobotsPrint
	if len(rb.ExpireMap) > cm.GlobalConfig.RobotsBuffSize {
		for url_, expiration := range rb.ExpireMap {

			respBodyStr, _ := rb.RespBodyMap[url_]

			writeBuff = append(writeBuff, cm.RobotsPrint{
				URL:         url_,
				Expiration:  expiration,
				RespBodyStr: respBodyStr, // respBodyStr instead of robotsGroup is logged to db; robotsGroup can't be logged
			})

			delete(rb.ExpireMap, url_)
			delete(rb.CacheMap, url_)
			delete(rb.RespBodyMap, url_)
		}
	}
	rb.mutex.Unlock()

	if len(writeBuff) > 0 {
		rb.BulkWrite(writeBuff)
		writeBuff = writeBuff[:0]
	}
}

func (rb *RobotsDBConn) Disconnect() {
	if err := rb.Client.Disconnect(rb.Ctx); err != nil {
		panic(err)
	}
}
