package dbpoxy

type Operation struct {
	DbName         string                 `json:"db_name,omitempty" msgpack:"db_name,omitempty"`
	TableName      string                 `json:"table_name,omitempty" msgpack:"table_name,omitempty"`
	TableChildName string                 `json:"table_child_name,omitempty" msgpack:"table_child_name,omitempty"`
	OpName         string                 `json:"op_name,omitempty" msgpack:"op_name,omitempty"` //insert, update, delete, work, query
	Token          string                 `json:"token,omitempty" msgpack:"token,omitempty"`
	MsgId          uint32                 `json:"msg_id,omitempty" msgpack:"msg_id,omitempty"`
	RefTopic       string                 `json:"ref_topic,,omitempty" msgpack:"ref_topic,omitempty"`
	RefQos         byte                   `json:"ref_qos,omitempty" msgpack:"ref_qos,omitempty"`
	FilterId       string                 `json:"filter_id,omitempty" msgpack:"filter_id,omitempty"`
	Filter         map[string]interface{} `json:"filter,omitempty" msgpack:"filter,omitempty"`
	FilterPipe     []interface{}          `json:"filter_pipe,omitempty" msgpack:"filter_pipe,omitempty"`
	Value          map[string]interface{} `json:"value,omitempty" msgpack:"value,omitempty"`
	ResultOp       *bool                  `json:"result_op,omitempty" msgpack:"result_op,omitempty"`
	ResultChanges  float64                `json:"result_changes,omitempty" msgpack:"result_changes,omitempty"`
	ResultData     interface{}            `json:"result_data,omitempty" msgpack:"result_data,omitempty"`
	ResultErr      string                 `json:"result_err,omitempty" msgpack:"result_err,omitempty"`
	SqlExec        string                 `json:"sql_exec,omitempty" msgpack:"sql_exec,omitempty"`
	SqlQuery       string                 `json:"sql_query,omitempty" msgpack:"sql_query,omitempty"`
	SqlWorks       []SqlWork              `json:"sql_works,omitempty" msgpack:"sql_works,omitempty"`
	OssFileName    string                 `json:"oss_file_name,omitempty" msgpack:"oss_file_name,omitempty"`
	OssFileBase64  string                 `json:"oss_file_base64,omitempty" msgpack:"oss_file_base64,omitempty"` //base, hex
	OssFileHex     []byte                 `json:"oss_file_hex,omitempty" msgpack:"oss_file_hex,omitempty"`
	MongoWorks     []MongoWork            `json:"mongo_works,omitempty" msgpack:"mongo_works,omitempty"`
	//安全模式
	SaveMode *bool                  `json:"save_mode,omitempty" msgpack:"save_mode,omitempty"`
	CmdId    uint32                 `json:"cmd_id,omitempty" msgpack:"cmd_id,omitempty"`
	Params   map[string]interface{} `json:"params,omitempty" msgpack:"params,omitempty"`
}

type SqlWork struct {
	TableName string `json:"table_name,omitempty" msgpack:"table_name,omitempty"`
	Work      string `json:"work,omitempty" msgpack:"work,omitempty"`
	IdAlias   string `json:"id_alias,omitempty" msgpack:"id_alias,omitempty"`
}

type MongoWork struct {
	TableName string                 `json:"table_name,omitempty" msgpack:"table_name,omitempty"`
	Work      map[string]interface{} `json:"work,omitempty" msgpack:"work,omitempty"`
	IdAlias   string                 `json:"id_alias,omitempty" msgpack:"id_alias,omitempty"`
	OpName    string                 `json:"op_name,omitempty" msgpack:"op_name,omitempty"` //insert, update, delete, query, aggregate
}
