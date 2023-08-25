package heartbeat

import (
	"context"
	"fmt"
	"net/smtp"
	cm "observerPolite/common"
	db "observerPolite/mongodb"
	"time"
)

type Heartbeat struct {
	DBConn *db.DBConn
}

type HeartbeatInterface interface {
	Start()
	sendEmail()
}

func (hb *Heartbeat) sendEmail() {
	from := cm.GlobalConfig.HeartbeatEmailFrom
	pass := cm.GlobalConfig.HeartbeatEmailPW
	to := cm.GlobalConfig.HeartbeatEmailTo
	subject := "ALERT form wikiPolite HeartbeatDuration\n"
	body := fmt.Sprintf("Your program is not making any process in the past %.0f seconds", cm.GlobalConfig.HeartbeatDuration.Seconds())

	msg := []byte(subject + "\n" + body)

	err := smtp.SendMail("smtp.gmail.com:587",
		smtp.PlainAuth("", from, pass, "smtp.gmail.com"),
		from, []string{to}, msg)

	if err != nil {
		panic(err)
	}

	println("Email sent successfully!")
}

func (hb *Heartbeat) Start() {
	var prevCount int64 = 0
	for true { // if main exit, this for true breaks
		time.Sleep(cm.GlobalConfig.HeartbeatDuration)
		count, err := hb.DBConn.Collection.EstimatedDocumentCount(context.Background())
		if err != nil {
			//do something
			panic(err)
		}
		//fmt.Println(count)
		if count == prevCount {
			hb.sendEmail()
		}
		prevCount = count
	}
}
