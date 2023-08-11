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
)

type DBConn struct {
	Ctx        context.Context
	Client     *mongo.Client
	Database   *mongo.Database
	Collection *mongo.Collection
}

type DBConnInterface interface {
	Connect() // panic instead of returning error (FATAL)
	Insert(task cm.Task) error
	Disconnect() // panic instead of return error (END OF PROGRAM)
}

func (db *DBConn) Connect() {
	// DISABLE DB FOR DEBUG MODE
	if !cm.GlobalConfig.DBlogging {
		return
	}
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
	//TODO: ALWAYS MAKE NEW COLLECTIONS
	currentTime := time.Now()
	timeString := currentTime.Format("01022006")
	err = database.CreateCollection(db.Ctx, cm.GlobalConfig.DBCollection+timeString)
	if err != nil {
		log.Fatal(err)
	}
	collection := database.Collection(cm.GlobalConfig.DBCollection + timeString)
	db.Collection = collection
}

func (db *DBConn) Insert(dbDoc cm.TaskPrint) error {
	if !cm.GlobalConfig.DBlogging {
		fmt.Println(dbDoc)
		return nil
	}
	_, err := db.Collection.InsertOne(db.Ctx, dbDoc)
	if err != nil {
		log.Println(err) //TODO: do something
	}
	return err
}

func (db *DBConn) Disconnect() {
	if !cm.GlobalConfig.DBlogging {
		return
	}
	if err := db.Client.Disconnect(db.Ctx); err != nil {
		panic(err)
	}
}
