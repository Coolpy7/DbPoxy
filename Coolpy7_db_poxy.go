package main

import (
	"dbpoxy"
	"errors"
	"flag"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func main() {
	var (
		confFile = flag.String("c", "", "config file path")
	)
	flag.Parse()

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		panic(err)
	}
	if dir == "/" {
		dir += "data"
	} else {
		dir += "/data"
	}
	if _, err := os.Stat(dir); err != nil {
		if err = os.MkdirAll(dir, 0755); err != nil {
			panic(err)
		}
	}

	if *confFile == "" {
		*confFile = dir + "/dbpoxy.yml"
	}

	poxy := &dbpoxy.DbPoxy{}

	// parse config
	err = poxy.ParseConfig(*confFile)
	if err != nil {
		log.Println(err)
		return
	}

	// initialize logger
	err = InitLogger(dir + "/" + poxy.Config.LogFilePath)
	if err != nil {
		panic(err)
	}

	opts := MQTT.NewClientOptions().
		AddBroker("tcp://" + poxy.Config.BrokerHost + ":" + strconv.Itoa(poxy.Config.BrokerPort)).
		SetClientID(poxy.Config.BrokerClientId).SetUsername(poxy.Config.BrokerUser).SetPassword(poxy.Config.BrokerPassword).
		SetAutoReconnect(true).SetMaxReconnectInterval(2 * time.Second).SetCleanSession(poxy.Config.BrokerClearSession)
	c := MQTT.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		log.Println("start error", token.Error())
		return
	}

	if token := c.Subscribe(poxy.Config.OpTopics.Topic, poxy.Config.OpTopics.Qos, poxy.BrokerLoadHandler); token.Wait() && token.Error() != nil {
		log.Println("start error", token.Error())
		return
	} else {
		log.Println(poxy.Config.OpTopics.Topic, "OK, token:", poxy.Config.AccessToken)
	}

	signalChan := make(chan os.Signal, 1)
	cleanupDone := make(chan bool)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for range signalChan {
			if c.IsConnected() {
				c.Disconnect(5)
			}
			poxy.Close()
			log.Println("safe quit")
			cleanupDone <- true
		}
	}()
	<-cleanupDone
}

func InitLogger(filename string) error {
	logfile, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return errors.New("failed to OpenFile.")
	}

	log.SetOutput(io.MultiWriter(logfile, os.Stdout))
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	return nil
}
