package dbpoxy

type GatewayConfig struct {
	DatabaseType             string  `yaml:"DatabaseType"`
	DatabaseInitTable        string  `yaml:"DatabaseInitTable"`
	DatabaseKeep             int     `yaml:"DatabaseKeep"`
	DatabaseDays             int     `yaml:"DatabaseDays"`
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
	DatabaseType string    `json:"database_type"`
	Enable       bool      `json:"enable"`
	GInject      string    `json:"inject"`
	Cmd          []PureCmd `json:"cmd"`
}

type PureCmd struct {
	DbName    string                 `json:"db_name,omitempty" msgpack:"db_name,omitempty"`
	TableName string                 `json:"table_name,omitempty" msgpack:"table_name,omitempty"`
	OpName    string                 `json:"op_name,omitempty" msgpack:"op_name,omitempty"`
	Inject    string                 `json:"inject"`
	CmdId     uint32                 `json:"cmd_id"`
	Value     map[string]interface{} `json:"value"`
	SqlWorks  []SqlWork              `json:"sql_works,omitempty" msgpack:"sql_works,omitempty"`
	Pcount    uint32                 `json:"param_count"`
}
