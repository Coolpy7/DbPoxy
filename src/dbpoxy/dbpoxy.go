package dbpoxy

import (
	"encoding/base64"
	"errors"
	_ "github.com/denisenkom/go-mssqldb"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
	_ "github.com/lib/pq"
	"github.com/pquerna/ffjson/ffjson"
	"gopkg.in/h2non/filetype.v1"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"gopkg.in/vmihailenco/msgpack.v2"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"reflect"
	"strconv"
	"strings"
	"time"
	"topic"
)

type DbPoxy struct {
	Config *GatewayConfig
	Mongo  *mgo.Session
	Sqldb  *xorm.Engine
}

func (d *DbPoxy) Close() {
	if d.Mongo != nil {
		d.Mongo.Close()
	}
	if d.Sqldb != nil {
		d.Sqldb.Close()
	}
}

func (d *DbPoxy) BrokerLoadHandler(client MQTT.Client, msg MQTT.Message) {
	var op Operation
	err := ffjson.Unmarshal(msg.Payload(), &op)
	if err != nil {
		err = msgpack.Unmarshal(msg.Payload(), &op)
		if err != nil {
			return
		}
	}
	if d.Config.AccessToken != "" {
		if d.Config.AccessToken != op.Token {
			d.SendOk(client, &op, nil, 0, errors.New("token error"))
			return
		}
	}

	if d.Config.DatabaseType == "mongodb" {
		sess := d.Mongo.Copy()
		defer sess.Close()
		switch op.OpName {
		case "insert":
			if op.Value == nil {
				d.SendOk(client, &op, nil, 0, errors.New("value is nil"))
				return
			}
			LoopParseDatetimeOrOid(op.Value)
			op.Value["createat"] = time.Now().Local()
			op.Value["_id"] = bson.NewObjectId()
			err = sess.DB(op.DbName).C(op.TableName).Insert(op.Value)
			d.SendOk(client, &op, op.Value["_id"], 1, err)
		case "update":
			if op.Value == nil {
				d.SendOk(client, &op, nil, 0, errors.New("value is nil"))
				return
			}
			LoopParseDatetimeOrOid(op.Value)
			op.Value["updateat"] = time.Now().Local()
			if op.FilterId != "" && bson.IsObjectIdHex(op.FilterId) {
				err = sess.DB(op.DbName).C(op.TableName).UpdateId(bson.ObjectIdHex(op.FilterId), bson.M{"$set": op.Value})
				d.SendOk(client, &op, op.FilterId, 1, err)
			} else {
				LoopParseDatetimeOrOid(op.Filter)
				info, err := sess.DB(op.DbName).C(op.TableName).UpdateAll(op.Filter, bson.M{"$set": op.Value})
				d.SendOk(client, &op, op.Filter, int(info.Updated), err)
			}
		case "delete":
			if op.FilterId != "" && bson.IsObjectIdHex(op.FilterId) {
				err = sess.DB(op.DbName).C(op.TableName).RemoveId(bson.ObjectIdHex(op.FilterId))
				d.SendOk(client, &op, op.FilterId, 1, err)
			} else {
				LoopParseDatetimeOrOid(op.Filter)
				info, err := sess.DB(op.DbName).C(op.TableName).RemoveAll(op.Filter)
				d.SendOk(client, &op, op.Filter, int(info.Updated), err)
			}
		case "query":
			if op.FilterId != "" && bson.IsObjectIdHex(op.FilterId) {
				var res map[string]interface{}
				err = sess.DB(op.DbName).C(op.TableName).FindId(bson.ObjectIdHex(op.FilterId)).One(&res)
				d.SendOk(client, &op, res, len(res), err)
			} else {
				if op.FilterPipe == nil {
					d.SendOk(client, &op, nil, 0, errors.New("filter pipe is nil"))
					return
				}
				LoopParseDatetimeOrOid(op.FilterPipe)
				var res []map[string]interface{}
				err = sess.DB(op.DbName).C(op.TableName).Pipe(op.FilterPipe).All(&res)
				d.SendOk(client, &op, res, len(res), err)
			}
		default:
			d.SendOk(client, &op, nil, 0, errors.New("invail op"))
		}
	} else if d.Config.DatabaseType == "postgres" || d.Config.DatabaseType == "mssql" || d.Config.DatabaseType == "mysql" {
		sess := d.Sqldb.Table(op.TableName)
		defer sess.Close()
		switch op.OpName {
		case "insert":
			if op.SqlExec == "" {
				d.SendOk(client, &op, nil, 0, errors.New("sqlexec is nil"))
				return
			}
			ref, err := sess.Exec(op.SqlExec)
			if err != nil {
				d.SendOk(client, &op, nil, 0, err)
			}
			nid, err := ref.LastInsertId()
			if err != nil {
				var nids []int64
				err = sess.SQL("SELECT MAX(id) FROM " + op.TableName).Find(&nids)
				if err == nil {
					d.SendOk(client, &op, map[string]interface{}{"id": nids[0]}, 1, nil)
				} else {
					d.SendOk(client, &op, nil, 0, err)
				}
			} else {
				d.SendOk(client, &op, map[string]interface{}{"id": nid}, 1, nil)
			}
		case "update", "delete":
			if op.SqlExec == "" {
				d.SendOk(client, &op, nil, 0, errors.New("sqlexec is nil"))
				return
			}
			ref, err := sess.Exec(op.SqlExec)
			changes, err := ref.RowsAffected()
			d.SendOk(client, &op, nil, int(changes), err)
		case "work":
			if len(op.SqlWorks) > 0 {
				var haserr error
				res := make(map[int]interface{})
				alias := make(map[string]string)
				ss := d.Sqldb.NewSession()
				defer ss.Close()
				for idx, v := range op.SqlWorks {
					if v.Work != "" {
						if strings.HasPrefix(strings.ToLower(v.Work), "insert") {
							ref, err := ss.Exec(v.Work)
							if err != nil {
								res[idx] = err.Error()
								ss.Rollback()
								haserr = err
								break
							}
							nid, err := ref.LastInsertId()
							if err != nil {
								var nids []int64
								sess.SQL("SELECT MAX(id) FROM " + v.TableName).Find(&nids)
								res[idx] = nids[0]
								alias[v.IdAlias] = strconv.FormatInt(nids[0], 10)
							} else {
								res[idx] = nid
								alias[v.IdAlias] = strconv.FormatInt(nid, 10)
							}
						} else {
							work := ""
							for k, vv := range alias {
								work = strings.Replace(v.Work, k, vv, -1)
							}
							ref, err := sess.Exec(work)
							if err != nil {
								res[idx] = err.Error()
								ss.Rollback()
								haserr = err
								break
							}
							changes, _ := ref.RowsAffected()
							res[idx] = changes
						}
					}
				}
				if haserr == nil {
					err = ss.Commit()
				}
				d.SendOk(client, &op, res, len(res), haserr)
			}
		case "query":
			if op.SqlQuery == "" {
				d.SendOk(client, &op, nil, 0, errors.New("sqlquery is nil"))
				return
			}
			var res []map[string]interface{}
			result, err := sess.Query(op.SqlQuery)
			for _, re := range result {
				mm := make(map[string]interface{})
				for k, v := range re {
					nv := string(v)
					nt, err := time.Parse(time.RFC3339, nv)
					if err == nil {
						mm[k] = nt
					} else {
						ff, err := strconv.ParseFloat(nv, 64)
						if err == nil {
							mm[k] = ff
						} else {
							mm[k] = nv
						}
					}
				}
				res = append(res, mm)
			}
			if err == nil {
				d.SendOk(client, &op, res, len(res), nil)
			} else {
				d.SendOk(client, &op, nil, 0, err)
			}
		default:
			d.SendOk(client, &op, nil, 0, errors.New("invail op"))
		}
	} else if d.Config.DatabaseType == "oss-gridfs" {
		sess := d.Mongo.Copy()
		defer sess.Close()
		switch op.OpName {
		case "insert":
			if op.OssFileName == "" {
				d.SendOk(client, &op, nil, 0, errors.New("value is nil"))
				return
			}
			op.OssFileName = strings.ToLower(op.OssFileName)
			if op.OssFileBase64 != "" {
				bfs := strings.Split(op.OssFileBase64, ",")
				if len(bfs) == 2 {
					op.OssFileBase64 = bfs[1]
				}
				op.OssFileHex, err = base64.StdEncoding.DecodeString(op.OssFileBase64)
				if err != nil {
					d.SendOk(client, &op, nil, 0, err)
					return
				}
			}
			if len(op.OssFileHex) < 261 {
				d.SendOk(client, &op, nil, 0, errors.New("content less"))
				return
			}
			//提取http的content-type文件类型
			kind, unkwown := filetype.Match(op.OssFileHex[:261])
			if unkwown != nil {
				kind.MIME.Value = "application/octet-stream"
			}
			fs, err := sess.DB(op.DbName).GridFS(op.TableName).Create(op.OssFileName)
			defer fs.Close()
			fid := bson.NewObjectId()
			if err == nil {
				fs.SetId(fid)
				fs.SetContentType(kind.MIME.Value)
				fs.SetMeta(map[string]interface{}{
					"ext": kind.Extension,
				})
				fs.Write(op.OssFileHex)
			}
			d.SendOk(client, &op, fid, 1, err)
		case "delete":
			if op.FilterId != "" && bson.IsObjectIdHex(op.FilterId) {
				err = sess.DB(op.DbName).GridFS(op.TableName).RemoveId(bson.ObjectIdHex(op.FilterId))
				d.SendOk(client, &op, op.FilterId, 1, err)
			}
		default:
			d.SendOk(client, &op, nil, 0, errors.New("invail op"))
		}
	}
}

func LoopParseDatetimeOrOid(inobj interface{}) {
	obj := reflect.ValueOf(inobj)
	if obj.Kind() == reflect.Map {
		o := obj.Interface().(map[string]interface{})
		for k, v := range o {
			t := reflect.ValueOf(v)
			switch t.Kind() {
			case reflect.String:
				nt, err := time.Parse(time.RFC3339, t.Interface().(string))
				if err == nil {
					o[k] = nt
				}
				if bson.IsObjectIdHex(t.Interface().(string)) {
					o[k] = bson.ObjectIdHex(t.Interface().(string))
				}
			case reflect.Map, reflect.Array, reflect.Slice:
				LoopParseDatetimeOrOid(t.Interface())
			default:
				o[k] = v
			}
		}
	} else if obj.Kind() == reflect.Array || obj.Kind() == reflect.Slice {
		o := obj.Interface().([]interface{})
		for i, v := range o {
			t := reflect.ValueOf(v)
			switch t.Kind() {
			case reflect.String:
				nt, err := time.Parse(time.RFC3339, t.Interface().(string))
				if err == nil {
					o[i] = nt
				}
				if bson.IsObjectIdHex(t.Interface().(string)) {
					o[i] = bson.ObjectIdHex(t.Interface().(string))
				}
			case reflect.Map, reflect.Array, reflect.Slice:
				LoopParseDatetimeOrOid(t.Interface())
			default:
				o[i] = v
			}
		}
	}
}

func (d *DbPoxy) SendOk(client MQTT.Client, op *Operation, data interface{}, changs int, inerr error) {
	if op.MsgId == 0 {
		return
	}
	tp, err := topic.Parse(op.RefTopic, false)
	if err != nil {
		return
	}
	ro := false
	var nop Operation
	if inerr != nil {
		nop = Operation{
			MsgId:         op.MsgId,
			ResultOp:      &ro,
			ResultChanges: float64(changs),
			ResultErr:     inerr.Error(),
		}
	} else if data != nil {
		resValue := reflect.ValueOf(data)
		var res interface{}
		if resValue.Kind() == reflect.Array || resValue.Kind() == reflect.Slice {
			res = data
		} else {
			res = []interface{}{data}
		}
		ro = true
		nop = Operation{
			MsgId:         op.MsgId,
			ResultOp:      &ro,
			ResultChanges: float64(changs),
			ResultData:    res,
		}
	} else {
		ro = true
		nop = Operation{
			MsgId:         op.MsgId,
			ResultOp:      &ro,
			ResultChanges: float64(changs),
		}
	}
	if op.RefQos < 0 || op.RefQos > 2 {
		op.RefQos = 0
	}
	bts, _ := ffjson.Marshal(&nop)
	client.Publish(tp, op.RefQos, false, bts)
}

func (d *DbPoxy) ParseConfig(filename string) error {
	var config GatewayConfig
	ymlStr, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(ymlStr, &config)
	if err != nil {
		return err
	}
	d.Config = &config

	if d.Config.DatabaseType == "mongodb" || d.Config.DatabaseType == "oss-gridfs" {
		d.Mongo, err = mgo.Dial(config.DatabaseConnectionString)
		if err != nil {
			return err
		}
		d.Mongo.SetMode(mgo.Monotonic, true)
		if err = d.Mongo.Ping(); err != nil {
			log.Println(d.Config.DatabaseType, "ping err", err.Error())
		} else {
			log.Println(d.Config.DatabaseType, "ping ok")
		}
	} else if d.Config.DatabaseType == "postgres" || d.Config.DatabaseType == "mssql" || d.Config.DatabaseType == "mysql" {
		d.Sqldb, err = xorm.NewEngine(d.Config.DatabaseType, d.Config.DatabaseConnectionString)
		if err != nil {
			return err
		}
		d.Sqldb.TZLocation, _ = time.LoadLocation("Asia/Shanghai")
		d.Sqldb.DB().SetMaxIdleConns(400)
		d.Sqldb.DB().SetMaxOpenConns(4000)
		if err = d.Sqldb.DB().Ping(); err != nil {
			log.Println(d.Config.DatabaseType, "ping err", err.Error())
		} else {
			log.Println(d.Config.DatabaseType, "ping ok")
		}
	} else {
		log.Println(d.Config.DatabaseType, "not suport database type")
	}

	return nil
}
