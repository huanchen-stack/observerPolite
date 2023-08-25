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
	//if !cm.GlobalConfig.DBlogging {
	//	return
	//}

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
	err := db.Database.CreateCollection(db.Ctx, cm.GlobalConfig.DBCollection+timeString)
	if err != nil {
		log.Fatal(err)
	}
	collection := db.Database.Collection(name + timeString)
	db.Collection = collection
}

func (db *DBConn) Insert(dbDoc cm.TaskPrint) error {
	if !cm.GlobalConfig.DBlogging {
		//fmt.Println(dbDoc)
		return nil
	}
	_, err := db.Collection.InsertOne(db.Ctx, dbDoc)
	if err != nil {
		log.Println(err) //TODO: do something
	}
	return err
}

func (db *DBConn) CreateIndex(idx string) error {
	indexes := db.Collection.Indexes()
	cursor, err := indexes.List(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(context.Background())

	indexExists := false
	for cursor.Next(context.Background()) {
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
		if _, err := indexes.CreateOne(context.Background(), indexModel); err != nil {
			log.Fatal(err)
			return err
		}
		fmt.Println("Index on 'url' field created")
	} else {
		fmt.Println("Index on 'url' field already exists")
	}
	return nil
}

func (db *DBConn) GetOne(key string, val string) cm.TaskPrint {
	filter := bson.M{key: val}
	var result cm.TaskPrint
	err := db.Collection.FindOne(context.Background(), filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return cm.TaskPrint{
				URL: "",
			}
		} else {
			log.Fatal(err)
		}
	}
	return result
}

func (db *DBConn) Disconnect() {
	//if !cm.GlobalConfig.DBlogging {
	//	return
	//}
	if err := db.Client.Disconnect(db.Ctx); err != nil {
		panic(err)
	}
}
