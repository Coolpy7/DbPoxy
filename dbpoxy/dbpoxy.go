package dbpoxy

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Coolpy7/DbPoxy/topic"
	_ "github.com/denisenkom/go-mssqldb"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
	_ "github.com/lib/pq"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.mongodb.org/mongo-driver/x/bsonx"
	"gopkg.in/h2non/filetype.v1"
	"gopkg.in/vmihailenco/msgpack.v2"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type DbPoxy struct {
	Config  *GatewayConfig
	Cmdfig  *CmdConfig
	Mongo   *mongo.Client
	Sqldb   *xorm.Engine
	jsonpre byte
	jsonend byte
	Quit    chan bool
	Choke   chan DbProxyMessage
}

func NewDbPoxy() *DbPoxy {
	poxy := &DbPoxy{
		Quit:  make(chan bool),
		Choke: make(chan DbProxyMessage),
	}
	poxy.jsonpre = byte('{')
	poxy.jsonend = byte('}')
	return poxy
}

func (d *DbPoxy) Close() {
	if d.Mongo != nil {
		_ = d.Mongo.Disconnect(context.Background())
	}
	if d.Sqldb != nil {
		_ = d.Sqldb.Close()
	}
}

func (d *DbPoxy) Run() {
	for {
		select {
		case incoming := <-d.Choke:
			d.mqttHandler(incoming.Client, incoming.Message)
		case <-d.Quit:
			return
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}
}

func (d *DbPoxy) mqttHandler(client MQTT.Client, msg MQTT.Message) {
	var op Operation
	payload := msg.Payload()
	if len(payload) == 0 {
		log.Println("error payload is nil")
		return
	}
	if payload[0] == d.jsonpre && payload[len(payload)-1] == d.jsonend {
		err := json.Unmarshal(payload, &op)
		if err != nil {
			log.Println("error payload json unmarshal", string(payload))
			return
		}
	} else {
		err := msgpack.Unmarshal(payload, &op)
		if err != nil {
			log.Println("error payload msgpack unmarshal", string(payload))
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
		if op.SaveMode == nil {
			d.mgdo(client, op)
		} else if *op.SaveMode && d.Cmdfig.Enable {
			//全局防注入
			if d.Cmdfig.GInject != "" {
				gis := strings.Split(d.Cmdfig.GInject, ",")
				for _, v := range op.Params {
					if vv, ok := v.(string); ok {
						for _, gv := range gis {
							if strings.Contains(vv, gv) {
								d.SendOk(client, &op, nil, 0, errors.New("inject error"))
								return
							}
						}
					}
				}
			}
			//指令转换
			for _, v := range d.Cmdfig.Cmd {
				if op.CmdId == v.CmdId && len(op.Params) == int(v.Pcount) {
					//指令防注入
					if v.Inject != "" {
						gis := strings.Split(v.Inject, ",")
						for _, pv := range op.Params {
							if vv, ok := pv.(string); ok {
								for _, gv := range gis {
									if strings.Contains(vv, gv) {
										d.SendOk(client, &op, nil, 0, errors.New("inject error"))
										return
									}
								}
							}
						}
					}
					op.DbName = v.DbName
					op.TableName = v.TableName
					op.OpName = v.OpName
					op.Value = make(map[string]interface{})
					for k1, v1 := range v.Value {
						if vv, ok := v1.(string); ok {
							op.Value[k1] = op.Params[vv]
						}
					}
					d.mgdo(client, op)
				} else {
					d.SendOk(client, &op, nil, 0, errors.New("save mode params count error"))
					return
				}
			}
		}
	} else if d.Config.DatabaseType == "postgres" || d.Config.DatabaseType == "mssql" || d.Config.DatabaseType == "mysql" {
		if op.SaveMode == nil {
			d.sqldo(client, op)
		} else if *op.SaveMode && d.Cmdfig.Enable {
			//全局防注入
			if d.Cmdfig.GInject != "" {
				err := d.inject(d.Cmdfig.GInject, op.Params)
				if err != nil {
					d.SendOk(client, &op, nil, 0, errors.New("inject error"))
					return
				}
			}
			//指令转换
			for _, v := range d.Cmdfig.Cmd {
				if op.CmdId == v.CmdId { //&& len(op.Params) == int(v.Pcount)
					//指令防注入
					if v.Inject != "" {
						err := d.inject(v.Inject, op.Params)
						if err != nil {
							d.SendOk(client, &op, nil, 0, errors.New("inject error"))
							return
						}
					}
					op.DbName = v.DbName
					op.TableName = v.TableName
					op.OpName = v.OpName
					switch op.OpName {
					case "insert", "update", "delete":
						if vv, ok := v.Value["sql_exec"].(string); ok {
							op.SqlExec = d.rpv(0, vv, op.Params)
						}
					case "work":
						if len(v.SqlWorks) > 0 {
							if pv, ok := op.Params["work"]; ok {
								if lpv, ok := pv.([]interface{}); ok {
									pcount := 0
									for wi, sv := range v.SqlWorks {
										if wi <= len(lpv)-1 {
											if ipvv, ok := lpv[wi].(map[string]interface{}); ok {
												sql := d.rpv(pcount, sv.Work, ipvv)
												op.SqlWorks = append(op.SqlWorks, SqlWork{TableName: sv.TableName, IdAlias: sv.IdAlias, Work: sql})
												pcount += len(ipvv)
											} else {
												d.SendOk(client, &op, nil, 0, errors.New("params work value error"))
												return
											}
										} else {
											op.SqlWorks = append(op.SqlWorks, sv)
										}
									}

								}
							}
						}
					case "query":
						if vv, ok := v.Value["sql_query"].(string); ok {
							op.SqlQuery = d.rpv(0, vv, op.Params)
						}
					default:
						d.SendOk(client, &op, nil, 0, errors.New("op error"))
						return
					}
					d.sqldo(client, op)
				}
			}
		}
	} else if d.Config.DatabaseType == "oss-gridfs" {
		d.ssodo(client, op)
	}
}

func (d *DbPoxy) inject(inject string, params map[string]interface{}) error {
	gis := strings.Split(inject, ",")
	for _, v := range params {
		if reflect.TypeOf(v).Kind() == reflect.Slice {
			for _, mv := range v.([]interface{}) {
				if reflect.TypeOf(mv).Kind() == reflect.Map {
					m := mv.(map[string]interface{})
					for _, sv := range m {
						if vv, ok := sv.(string); ok {
							for _, gv := range gis {
								if strings.Contains(vv, gv) {
									return errors.New("inject error")
								}
							}
						}
					}
				}
			}
		} else {
			if vv, ok := v.(string); ok {
				for _, gv := range gis {
					if strings.Contains(vv, gv) {
						return errors.New("inject error")
					}
				}
			}
		}
	}
	return nil
}

func (d *DbPoxy) rpv(startpindex int, sql string, params map[string]interface{}) string {
	for pk, pv := range params {
		if reflect.TypeOf(pv).Kind() == reflect.String {
			sql = strings.Replace(sql, pk, "'%v'", -1)
		} else {
			sql = strings.Replace(sql, pk, "%v", -1)
		}
	}
	itfs := make([]interface{}, 0)
	for i := 0; i < len(params); i++ {
		idx := strconv.Itoa(i + startpindex)
		itfs = append(itfs, params["{"+idx+"}"])
	}
	return fmt.Sprintf(sql, itfs...)
}

func (d *DbPoxy) ssodo(client MQTT.Client, op Operation) {
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
			fhex, err := base64.StdEncoding.DecodeString(op.OssFileBase64)
			if err != nil {
				d.SendOk(client, &op, nil, 0, err)
				return
			}
			op.OssFileHex = fhex
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
		bucket, err := gridfs.NewBucket(d.Mongo.Database(op.DbName), options.GridFSBucket().SetName(op.TableName))
		if err != nil {
			d.SendOk(client, &op, nil, 0, err)
			return
		}
		rd := bytes.NewReader(op.OssFileHex)
		fid := primitive.NewObjectID()
		meta := bsonx.Doc{
			{"Content-type", bsonx.String(kind.MIME.Value)},
			{"Ext", bsonx.String(kind.Extension)},
		}
		err = bucket.UploadFromStreamWithID(fid, op.OssFileName, rd, options.GridFSUpload().SetMetadata(meta))
		if err != nil {
			d.SendOk(client, &op, nil, 0, err)
			return
		}
		d.SendOk(client, &op, fid, 1, err)
	case "delete":
		findid, err := primitive.ObjectIDFromHex(op.FilterId)
		if err == nil {
			bucket, err := gridfs.NewBucket(d.Mongo.Database(op.DbName), options.GridFSBucket().SetName(op.TableName))
			if err != nil {
				d.SendOk(client, &op, nil, 0, err)
				return
			}
			err = bucket.Delete(findid)
			if err != nil {
				d.SendOk(client, &op, nil, 0, err)
				return
			}
			d.SendOk(client, &op, op.FilterId, 1, err)
		}
	default:
		d.SendOk(client, &op, nil, 0, errors.New("invail op"))
	}
}

func (d *DbPoxy) sqldo(client MQTT.Client, op Operation) {
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
							_ = ss.Rollback()
							haserr = err
							break
						}
						nid, err := ref.LastInsertId()
						if err != nil {
							var nids []int64
							_ = sess.SQL("SELECT MAX(id) FROM " + v.TableName).Find(&nids)
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
							_ = ss.Rollback()
							haserr = err
							break
						}
						changes, _ := ref.RowsAffected()
						res[idx] = changes
					}
				}
			}
			if haserr == nil {
				_ = ss.Commit()
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
}

func (d *DbPoxy) mgdo(client MQTT.Client, op Operation) {
	switch op.OpName {
	case "insert":
		if op.Value == nil {
			d.SendOk(client, &op, nil, 0, errors.New("value is nil"))
			return
		}
		LoopParseDatetimeOrOid(op.Value, false, "", nil)
		op.Value["createat"] = time.Now().Local()
		res, err := d.Mongo.Database(op.DbName).Collection(op.TableName).InsertOne(context.Background(), op.Value)
		if err != nil {
			d.SendOk(client, &op, nil, 0, err)
		} else {
			d.SendOk(client, &op, res.InsertedID, 1, err)
		}
	case "update":
		if op.Value == nil {
			d.SendOk(client, &op, nil, 0, errors.New("value is nil"))
			return
		}
		LoopParseDatetimeOrOid(op.Value, false, "", nil)
		op.Value["updateat"] = time.Now().Local()
		findid, err := primitive.ObjectIDFromHex(op.FilterId)
		if err == nil {
			filter := bson.M{"_id": findid}
			res, err := d.Mongo.Database(op.DbName).Collection(op.TableName).UpdateOne(context.Background(), filter, bson.M{"$set": op.Value})
			if err != nil {
				d.SendOk(client, &op, nil, 0, err)
			} else {
				d.SendOk(client, &op, nil, int(res.ModifiedCount), err)
			}
		} else {
			LoopParseDatetimeOrOid(op.Filter, false, "", nil)
			res, err := d.Mongo.Database(op.DbName).Collection(op.TableName).UpdateMany(context.Background(), op.Filter, bson.M{"$set": op.Value})
			if err != nil {
				d.SendOk(client, &op, nil, 0, err)
			} else {
				d.SendOk(client, &op, nil, int(res.ModifiedCount), err)
			}
		}
	case "delete":
		findid, err := primitive.ObjectIDFromHex(op.FilterId)
		if err == nil {
			filter := bson.M{"_id": findid}
			res, err := d.Mongo.Database(op.DbName).Collection(op.TableName).DeleteOne(context.Background(), filter)
			if err != nil {
				d.SendOk(client, &op, nil, 0, err)
			} else {
				d.SendOk(client, &op, nil, int(res.DeletedCount), err)
			}
		} else {
			LoopParseDatetimeOrOid(op.Filter, false, "", nil)
			res, err := d.Mongo.Database(op.DbName).Collection(op.TableName).DeleteMany(context.Background(), op.Filter)
			if err != nil {
				d.SendOk(client, &op, nil, 0, err)
			} else {
				d.SendOk(client, &op, nil, int(res.DeletedCount), err)
			}
		}
	case "work":
		if len(op.MongoWorks) > 0 {
			sess, err := d.GetSess()
			if err != nil {
				d.SendOk(client, &op, nil, 0, err)
				return
			}
			db := d.Mongo.Database(op.DbName)
			res := make(map[int]interface{})
			alias := make(map[string]interface{})
			err = mongo.WithSession(context.Background(), sess, func(sessionContext mongo.SessionContext) error {
				err := sessionContext.StartTransaction()
				if err != nil {
					return err
				}
				for idx, v := range op.MongoWorks {
					if v.Work != nil {
						tn, _ := db.ListCollections(context.Background(), bson.M{"name": v.TableName})
						if tn.Next(context.Background()) == false {
							db.RunCommand(context.Background(), bson.D{{"create", v.TableName}})
						}
						col := db.Collection(v.TableName)
						//跨文档事务
						if v.OpName == "insert" {
							LoopParseDatetimeOrOid(v.Work, false, "", nil)
							v.Work["createat"] = time.Now().Local()
							nr, err := col.InsertOne(sessionContext, v.Work)
							if err != nil {
								_ = sessionContext.AbortTransaction(sessionContext)
								return err
							}
							oid, ok := nr.InsertedID.(primitive.ObjectID)
							if !ok {
								_ = sessionContext.AbortTransaction(sessionContext)
								return err
							}
							alias[v.IdAlias] = oid.Hex()
							res[idx] = oid.Hex()
						} else if v.OpName == "update" {
							for k, vv := range alias {
								LoopParseDatetimeOrOid(v.Work, true, k, vv)
							}
							v.Work["updateat"] = time.Now().Local()
							nr, err := col.UpdateMany(sessionContext, v.Work["filter"], v.Work["value"])
							if err != nil {
								_ = sessionContext.AbortTransaction(sessionContext)
								return err
							}
							res[idx] = nr.MatchedCount
						} else if v.OpName == "delete" {
							for k, vv := range alias {
								LoopParseDatetimeOrOid(v.Work, true, k, vv)
							}
							nr, err := col.DeleteMany(sessionContext, v.Work["filter"])
							if err != nil {
								_ = sessionContext.AbortTransaction(sessionContext)
								return err
							}
							res[idx] = nr.DeletedCount
						} else if v.OpName == "query" {
							for k, vv := range alias {
								LoopParseDatetimeOrOid(v.Work, true, k, vv)
							}
							var objs []map[string]interface{}
							nr, err := col.Find(sessionContext, v.Work["filter"])
							if err != nil {
								_ = sessionContext.AbortTransaction(sessionContext)
								return err
							}
							for nr.Next(context.Background()) {
								var m map[string]interface{}
								_ = nr.Decode(&m)
								objs = append(objs, m)
							}
							res[idx] = objs
						}
					}
				}
				err = sessionContext.CommitTransaction(sessionContext)
				if err != nil {
					return err
				}
				d.SendOk(client, &op, res, len(res), err)
				return nil
			})
			if err != nil {
				d.SendOk(client, &op, nil, 0, err)
			}
		}
	case "query":
		findid, err := primitive.ObjectIDFromHex(op.FilterId)
		if err == nil {
			filter := bson.M{"_id": findid}
			var res map[string]interface{}
			err = d.Mongo.Database(op.DbName).Collection(op.TableName).FindOne(context.Background(), filter).Decode(&res)
			if err != nil {
				d.SendOk(client, &op, nil, 0, err)
				return
			}
			d.SendOk(client, &op, res, len(res), err)
		} else {
			if op.FilterPipe == nil {
				d.SendOk(client, &op, nil, 0, errors.New("filter pipe is nil"))
				return
			}
			LoopParseDatetimeOrOid(op.FilterPipe, false, "", nil)
			var res []map[string]interface{}
			cur, err := d.Mongo.Database(op.DbName).Collection(op.TableName).Aggregate(context.Background(), op.FilterPipe)
			if err != nil {
				d.SendOk(client, &op, nil, 0, err)
			}
			defer cur.Close(context.Background())
			for cur.Next(context.Background()) {
				var result map[string]interface{}
				err := cur.Decode(&result)
				if err != nil {
					break
				}
				res = append(res, result)
			}
			if err := cur.Err(); err != nil {
				d.SendOk(client, &op, nil, 0, err)
			}
			d.SendOk(client, &op, res, len(res), err)
		}
	default:
		d.SendOk(client, &op, nil, 0, errors.New("invail op"))
	}
}

func LoopParseDatetimeOrOid(inobj interface{}, isReplace bool, rkey string, rval interface{}) {
	obj := reflect.ValueOf(inobj)
	if obj.Kind() == reflect.Map {
		o := obj.Interface().(map[string]interface{})
		for k, v := range o {
			if isReplace {
				if v == rkey {
					v = rval
				}
			}
			t := reflect.ValueOf(v)
			switch t.Kind() {
			case reflect.String:
				nt, err := time.Parse(time.RFC3339, t.Interface().(string))
				if err == nil {
					o[k] = nt
				}
				id, err := primitive.ObjectIDFromHex(t.Interface().(string))
				if err == nil {
					o[k] = id
				}
			case reflect.Map, reflect.Array, reflect.Slice:
				LoopParseDatetimeOrOid(t.Interface(), isReplace, rkey, rval)
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
				id, err := primitive.ObjectIDFromHex(t.Interface().(string))
				if err == nil {
					o[i] = id
				}
			case reflect.Map, reflect.Array, reflect.Slice:
				LoopParseDatetimeOrOid(t.Interface(), isReplace, rkey, rval)
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
	bts, _ := json.Marshal(&nop)
	client.Publish(tp, op.RefQos, false, bts)
}

func (d *DbPoxy) GetSess() (mongo.Session, error) {
	session, err := d.Mongo.StartSession(options.Session().SetDefaultReadPreference(readpref.Primary()))
	if err != nil {
		return nil, err
	}
	return session, nil
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
		ops := options.Client().ApplyURI(config.DatabaseConnectionString)
		p := uint64(runtime.NumCPU() * 2)
		ops.MaxPoolSize = &p
		ops.WriteConcern = writeconcern.New(writeconcern.J(true), writeconcern.W(1))
		ops.ReadPreference = readpref.PrimaryPreferred()
		d.Mongo, err = mongo.NewClient(ops)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err = d.Mongo.Connect(ctx)
		if err != nil {
			return err
		}
		err = d.Mongo.Ping(ctx, readpref.PrimaryPreferred())
		if err != nil {
			log.Println(d.Config.DatabaseType, "ping err", err.Error())
			return err
		} else {
			log.Println(d.Config.DatabaseType, "ping ok")
		}
	} else if d.Config.DatabaseType == "postgres" || d.Config.DatabaseType == "mssql" || d.Config.DatabaseType == "mysql" {
		d.Sqldb, err = xorm.NewEngine(d.Config.DatabaseType, d.Config.DatabaseConnectionString)
		if err != nil {
			return err
		}
		d.Sqldb.TZLocation, _ = time.LoadLocation("Asia/Shanghai")
		d.Sqldb.DB().SetMaxIdleConns(runtime.NumCPU() * 2)
		d.Sqldb.DB().SetMaxOpenConns(runtime.NumCPU() * 2)
		if err = d.Sqldb.DB().Ping(); err != nil {
			log.Println(d.Config.DatabaseType, "ping err", err.Error())
			return err
		} else {
			log.Println(d.Config.DatabaseType, "ping ok")
		}
	} else {
		log.Println(d.Config.DatabaseType, "not suport database type")
	}

	return nil
}

func (d *DbPoxy) ParseCmdConfig(filename string) error {
	var config CmdConfig
	jsonStr, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	err = json.Unmarshal(jsonStr, &config)
	if err != nil {
		return err
	}
	if config.Enable {
		d.Cmdfig = &config
		if d.Cmdfig.DatabaseType != d.Config.DatabaseType {
			return errors.New("database type dbpoxy.yml and cmd.json not equal")
		}
	}

	return nil
}
