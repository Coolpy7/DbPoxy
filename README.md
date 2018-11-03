# DbPoxy
Coolpy7数据库代理
```
//插入
{
"db_name":"kyb-log-err",
"table_name":"test",
"op_name":"insert",
"token":"password",
"msg_id":1,
"ref_topic":"dbpoxy/mongodb/result",
"ref_qos":0,
"value":{"datetime":"2018-10-12T10:10:12+08:00","number":12345.12345655889977,"string":"djkfjdkfj" }
}
//更新by id
{
"db_name":"kyb-log-err",
"table_name":"test",
"op_name":"update",
"token":"password",
"msg_id":1,
"ref_topic":"dbpoxy/mongodb/result",
"ref_qos":0,
"filter_id":"5b03dde450d8e8bc6949e412",
"value":{"value":54321}
}
//更新按条件
{
"db_name":"kyb-log-err",
"table_name":"test",
"op_name":"update",
"token":"password",
"msg_id":1,
"ref_topic":"dbpoxy/mongodb/result",
"ref_qos":0,
"filter":{"$and":[{"datetime": {"$gte": "2018-10-16T10:10:12+08:00"}},{"datetime": {"$lte": "2018-10-18T10:10:12+08:00"}}]},
"value":{"value":111111}
}
//删除by id
{
"db_name":"kyb-log-err",
"table_name":"test",
"op_name":"delete",
"token":"password",
"msg_id":1,
"ref_topic":"dbpoxy/mongodb/result",
"ref_qos":0,
"filter_id":"5b2cbe3e6e53f71848ab3b24"
}
//删除 按条件
{
"db_name":"kyb-log-err",
"table_name":"test",
"op_name":"delete",
"token":"password",
"msg_id":1,
"ref_topic":"dbpoxy/mongodb/result",
"ref_qos":0,
"filter":{"datetime":"2018-10-12T10:10:12+08:00"}
}
//查询按条件
{
"db_name":"kyb-log-err",
"table_name":"test",
"op_name":"query",
"token":"password",
"msg_id":1,
"ref_topic":"dbpoxy/mongodb/result",
"ref_qos":0,
"filter_pipe":[
    {
        "$match": {
            "$and": [
                { "datetime": { "$gte": "2018-10-16T10:10:12+08:00" } },
                { "datetime": { "$lte": "2018-10-18T10:10:12+08:00" } }
            ]
        }
    },
    {"$skip":1},
    {"$limit":2}
]}
//查询by id
{
"db_name":"kyb-log-err",
"table_name":"test",
"op_name":"query",
"token":"password",
"msg_id":1,
"ref_topic":"dbpoxy/mongodb/result",
"ref_qos":0,
"filter_id":"5b03e04250d8e8bc694a3b2f"
}

====================================================================================
//sql数据库
//查询
{
"table_name":"users",
"op_name":"query",
"token":"password",
"msg_id":1,
"ref_topic":"dbpoxy/mongodb/result",
"ref_qos":0,
"sql_query":"SELECT * FROM users;"
}
//更新
{
"table_name":"users",
"op_name":"update",
"token":"password",
"msg_id":1,
"ref_topic":"dbpoxy/mongodb/result",
"ref_qos":0,
"sql_exec":"UPDATE users SET age=8, name='coolpy7'  WHERE id=2;"
}
//插入
{
"table_name":"users",
"op_name":"insert",
"token":"password",
"msg_id":1,
"ref_topic":"dbpoxy/mongodb/result",
"ref_qos":0,
"sql_exec":"INSERT INTO users(age, name, num) VALUES (100, 'ccc', 77)"
}
//删除
{
"table_name":"users",
"op_name":"insert",
"token":"password",
"msg_id":1,
"ref_topic":"dbpoxy/mongodb/result",
"ref_qos":0,
"sql_exec":"DELETE FROM users WHERE id=3;"
}
//事务 id_alias是插入成功后的新记录id
{
"table_name":"users",
"op_name":"work",
"token":"password",
"msg_id":1,
"ref_topic":"dbpoxy/mongodb/result",
"ref_qos":0,
"sql_works":[
{"table_name":"users", "work":"INSERT INTO users(age, name, num) VALUES (100, 'ccc', 77)","id_alias":"insertid"},
{"table_name":"users", "work":"UPDATE users SET age=8, name='coolpy7'  WHERE id=insertid"},
{"table_name":"users", "work":"DELETE FROM users WHERE id=insertid"}
]}
==================================================================
//oss
//上传文件
{
"db_name":"gridfs",
"table_name":"fs",
"op_name":"insert",
"token":"password",
"msg_id":1,
"ref_topic":"dbpoxy/mongodb/result",
"ref_qos":0,
"oss_file_name":文件名,
"oss_file_base64":以base64形式的文件流
// "oss_file_hex":以hex形式的文件流、此标注不能与oss_file_base64同时使用
}
//删除文件
{
"db_name":"gridfs",
"table_name":"fs",
"op_name":"delete",
"token":"password",
"msg_id":1,
"ref_topic":"dbpoxy/mongodb/result",
"ref_qos":0,
"filter_id":"5b0a610853c613cd1c4d9840"
}
```
