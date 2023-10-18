package tests

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	cm "observerPolite/common"
	"observerPolite/core"
	"time"
)

func execTimeThresh(expectedTestTime time.Duration, lt bool) error {
	errChan := make(chan error, 1)

	//go func() {
	//	flagArgs := make([]string, len(flags))
	//	idx := 0
	//	for k, v := range flags {
	//		flagArgs[idx] = "--" + k + "=" + v
	//		idx++
	//	}
	//	cmd := exec.Command("./../wikiPolite", flagArgs...)
	//
	//	var stdout, stderr bytes.Buffer
	//	cmd.Stdout = &stdout
	//	cmd.Stderr = &stderr
	//	err := cmd.Run()
	//	if err != nil {
	//		err = fmt.Errorf("Running main logic failed: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
	//	}
	//	errChan <- err
	//}()

	go func() {
		core.CORE()
		errChan <- nil
	}()

	var err error
	if lt {
		select {
		case err = <-errChan:
		case <-time.After(expectedTestTime):
			err = fmt.Errorf("finished after %d seconds", int(expectedTestTime.Seconds()))
		}
	} else {
		select {
		case err = <-errChan:
			err = fmt.Errorf("finished before %d seconds", int(expectedTestTime.Seconds()))
		case <-time.After(expectedTestTime):
			err = <-errChan
		}
	}

	return err
}

func dbQuery(collectionName string, docChan chan cm.TaskPrint, discard bool) {
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(cm.GlobalConfig.DBURI).SetServerAPIOptions(serverAPI)

	client, err := mongo.Connect(context.Background(), opts)
	if err != nil {
		panic(err)
	}
	collection := client.Database("wikiPolite").Collection(collectionName)
	cursor, _ := collection.Find(context.TODO(), bson.D{})

	defer func() {
		close(docChan)
		cursor.Close(context.TODO())
		if discard {
			collection.Drop(context.TODO())
		}
		client.Disconnect(context.TODO())
	}()

	for cursor.Next(context.TODO()) {
		var elem cm.TaskPrint
		err = cursor.Decode(&elem)
		docChan <- elem
	}

}

func dbMess(collectionName string, messRetry bool) {
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(cm.GlobalConfig.DBURI).SetServerAPIOptions(serverAPI)

	client, err := mongo.Connect(context.Background(), opts)
	if err != nil {
		panic(err)
	}
	collection := client.Database("wikiPolite").Collection(collectionName)
	cursor, _ := collection.Find(context.TODO(), bson.D{})

	defer func() {
		cursor.Close(context.TODO())
		client.Disconnect(context.TODO())
	}()

	idx := 0
	updates := []bson.M{
		{"$set": bson.M{"resp.statuscode": -1}},
		{"$set": bson.M{"redirectchain": []string{"-1 mockAddr"}}},
	}
	if messRetry {
		retryUpdates := []bson.M{
			{"$set": bson.M{"retry.resp.statuscode": -1}},
			{"$set": bson.M{"retry.redirectchain": []string{"-1 mockAddr"}}},
		}
		updates = append(updates, retryUpdates...)
	}
	for cursor.Next(context.TODO()) {
		var elem cm.TaskPrint
		err = cursor.Decode(&elem)
		filter := bson.M{"url": elem.URL}
		collection.UpdateOne(context.TODO(), filter, updates[idx])

		idx = (idx + 1) % len(updates)
	}

}
