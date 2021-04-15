# tdengine 
```
$ sudo tar -xvf TDengine-server-2.0.19.1-Linux-x64.tar.gz
$ sudo ./install.sh
$ sudo systemctl start taosd
$ sudo systemctl status taosd

//dos't local machine client operation
$ sudo cp td/driver/libtaos.so.2.0.19.1 /usr/local/lib
$ sudo cp td/inc/* /usr/local/include
$ sudo ldconfig
```

# DbPoxy tdengine
Coolpy7数据库代理
```
//create table 
{
"db_name":"db_cp7",
"table_name":"tb_cp7",
"table_child_name":"d0",
"op_name":"create",
"token":"password",
"msg_id":1,
"ref_topic":"dbpoxy/mongodb/result",
"ref_qos":0,
"sql_exec":"tags('Beijing', 0);"
}

//插入
{
"db_name":"db_cp7",
"table_child_name":"d0",
"op_name":"insert",
"token":"password",
"msg_id":1,
"ref_topic":"dbpoxy/mongodb/result",
"ref_qos":0,
"sql_exec":"( 1601510401000, 200.994125, 994, 0.112666 ) "
}

//查询
{
"db_name":"db_cp7",
"table_name":"tb_cp7",
"op_name":"query",
"token":"password",
"msg_id":1,
"ref_topic":"dbpoxy/mongodb/result",
"ref_qos":0,
"sql_query":"select * from db_cp7.tb_cp7 limit 3 offset 0;"
}
```
