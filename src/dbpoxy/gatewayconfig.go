package dbpoxy

type GatewayConfig struct {
	DatabaseType             string  `yaml:"DatabaseType"`
	DatabaseConnectionString string  `yaml:"DatabaseConnectionString"`
	BrokerHost               string  `yaml:"BrokerHost"`
	BrokerPort               int     `yaml:"BrokerPort"`
	BrokerClientId           string  `yaml:"BrokerClientId"`
	BrokerUser               string  `yaml:"BrokerUser"`
	BrokerPassword           string  `yaml:"BrokerPassword"`
	BrokerClearSession       bool    `yaml:"BrokerClearSession"`
	LogFilePath              string  `yaml:"LogFilePath"`
	AccessToken              string  `yaml:"AccessToken"`
	OpTopics                 OpTopic `yaml:"OpTopics"`
}

type OpTopic struct {
	Topic string `yaml:"Topic"`
	Qos   byte   `yaml:"Qos"`
}
