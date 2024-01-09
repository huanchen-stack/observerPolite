package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

type RespPrint struct {
	StatusCode int
	Header     map[string][]string
	ETag       string // ETag in http response
	ESelfTag   string // ETag computed by this scanner
	Size       int
} // created for TaskPrint and RetryPrint; db logging helper

type DstChangePrint struct {
	Scheme   bool
	Hostname bool
	Path     bool
	Query    bool
} // created for RetryPrint; redirect analysis helper

type RetryHTTPPrint struct {
	Retried       bool
	RedirectChain []string
	DstChange     DstChangePrint
	Resp          RespPrint
	Err           string
} // created for the TaskPrint struct

type TaskPrint struct {
	Source        string
	Hostname      string
	URL           string
	IP            string
	RedirectChain []string
	DstChange     DstChangePrint
	Resp          RespPrint
	Err           string
	NeedsRetry    bool
	Retry         RetryHTTPPrint
} // stores scan results

// Define a structure for your required data format
type JsonDoc struct {
	Statuscode         int
	Destination        string
	Retried            bool
	RetriedStatuscode  int
	RetriedDestination string
}

type Result struct {
	URL               string
	CollectionResults map[string]JsonDoc
}

func main() {
	// Set client options and connect to MongoDB
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(context.TODO())

	// Define your database and collections
	db := client.Database("wikiPolite")
	collections := []string{"TenMilScan10242023_dedeup", "TenMilScan11052023_dedeup", "TenMilScan11172023_dedeup", "TenMilScan11272023_dedeup", "TenMilScan12082023_dedeup"}

	// Prepare your query
	filter := bson.D{{}} // or your specific filter

	// Processing in parallel using goroutines
	var wg sync.WaitGroup
	results := make(map[string]*Result)
	mutex := &sync.Mutex{}

	go func() {
		// Open a file for writing.
		// This will create the file if it doesn't exist, or truncate it if it does.
		file, err := os.Create("example.txt")
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		defer file.Close() // Ensure the file is closed at the end of the function.
		writer := bufio.NewWriter(file)

		ticker := time.NewTicker(1 * time.Second)
		count := 0
		for range ticker.C {
			mutex.Lock()
			del := make([]string, 0)
			for _, v := range results {
				if len(v.CollectionResults) == len(collections) {
					count++
					jsonData, _ := json.Marshal(v)
					writer.Write(jsonData)
					writer.WriteString("\n")
					writer.Flush()
					del = append(del, v.URL)
				}
			}
			for _, d := range del {
				delete(results, d)
			}
			fmt.Println(count)
			mutex.Unlock()
		}
	}()

	for _, collectionName := range collections {
		wg.Add(1)
		go func(collectionName string) {
			defer wg.Done()
			collection := db.Collection(collectionName)
			fmt.Println("getting cursor for collection", collectionName)
			cursor, err := collection.Find(context.TODO(), filter)
			fmt.Println("Cursor get for collection", collectionName)
			if err != nil {
				log.Fatal(err)
			}
			defer cursor.Close(context.TODO())

			i := 0
			for cursor.Next(context.TODO()) {
				var doc bson.M
				if err := cursor.Decode(&doc); err != nil {
					log.Fatal(err)
				}
				url := doc["url"].(string)
				var taskPrint TaskPrint
				bsonBytes, _ := bson.Marshal(doc)
				bson.Unmarshal(bsonBytes, &taskPrint)

				mutex.Lock()
				if _, ok := results[url]; !ok {
					results[url] = &Result{URL: url, CollectionResults: make(map[string]JsonDoc)}
				}
				thisDoc := JsonDoc{}

				thisDoc.Statuscode = taskPrint.Resp.StatusCode
				if taskPrint.RedirectChain != nil && len(taskPrint.RedirectChain) >= 1 {
					lst := taskPrint.RedirectChain[len(taskPrint.RedirectChain)-1]
					prts := strings.Split(lst, " ")
					thisDoc.Destination = prts[len(prts)-1]
				}
				thisDoc.Retried = taskPrint.Retry.Retried
				thisDoc.RetriedStatuscode = taskPrint.Retry.Resp.StatusCode
				if taskPrint.Retry.RedirectChain != nil && len(taskPrint.Retry.RedirectChain) >= 1 {
					lst := taskPrint.Retry.RedirectChain[len(taskPrint.Retry.RedirectChain)-1]
					prts := strings.Split(lst, " ")
					thisDoc.RetriedDestination = prts[len(prts)-1]
				}
				results[url].CollectionResults[collectionName] = thisDoc

				mutex.Unlock()

				i += 1
				if i%1000 == 0 {
					fmt.Println(collectionName, i)
				}
				if i == 10000 {
					break
				}
			}
		}(collectionName)
	}

	wg.Wait()
	time.Sleep(3 * time.Second)
	//
	//// Write results to JSON file
	//file, err := os.Create("output.json")
	//if err != nil {
	//	log.Fatal(err)
	//}
	//defer file.Close()
	//
	//encoder := json.NewEncoder(file)
	//encoder.SetIndent("", "  ")
	//if err := encoder.Encode(results); err != nil {
	//	log.Fatal(err)
	//}

	fmt.Println("Data processing complete, output written to output.json")
}
