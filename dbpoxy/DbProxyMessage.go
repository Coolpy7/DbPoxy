package dbpoxy

import MQTT "github.com/eclipse/paho.mqtt.golang"

type DbProxyMessage struct {
	Client  MQTT.Client
	Message MQTT.Message
}
