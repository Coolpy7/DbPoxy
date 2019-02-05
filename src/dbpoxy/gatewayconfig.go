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

type CmdConfig struct {
	DatabaseType string    `json:"databasetype"`
	Enable       bool      `json:"enable"`
	GInject      string    `json:"ginject"`
	Cmds         []PureCmd `json:"cmds"`
}

type PureCmd struct {
	Inject string `json:"inject"`
	CmdId  int64  `json:"cmdid"`
	Cmd    string `json:"cmd"`
	Pcount int    `json:"pcount"`
	OpName string `json:"opname"`
}
