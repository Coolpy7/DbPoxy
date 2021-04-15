package main

import (
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"strconv"
	"testing"
	"time"
)

func BenchmarkPub(b *testing.B) {
	opts := MQTT.NewClientOptions()
	opts.AddBroker("tcp://127.0.0.1:1883")
	opts.SetClientID(strconv.FormatInt(time.Now().UnixNano(), 10))
	opts.SetCleanSession(true)
	opts.SetKeepAlive(60)

	client := MQTT.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	bigData := []byte(`{"db_name":"db_cp7","table_child_name":"d0","op_name":"insert","token":"password","msg_id":1,"ref_topic":"dbpoxy/mongodb/result","ref_qos":0,"sql_exec":"( 1601510401000, 200.994125, 994, 0.112666 ) "}`)
	for i := 0; i < 10000; i++ {
		token := client.Publish("dbpoxy/mongodb/get", byte(0), false, bigData)
		if token.Wait() && token.Error() != nil {
			b.Fail()
		}
	}
}
