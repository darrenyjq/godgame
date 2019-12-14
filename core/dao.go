package core

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	"github.com/jinzhu/gorm"
	"github.com/json-iterator/go"
	"godgame/config"
	"gopkg.in/olivere/elastic.v5"
	"iceberg/frame"
	iconfig "iceberg/frame/config"
	"iceberg/frame/util"
	lyg_util "laoyuegou.com/util"
	user_pb "laoyuegou.pb/user/pb"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// Dao core dao
type Dao struct {
	Cfg          config.Config
	Cpool        *redis.Pool             // 缓存池
	dbr          *gorm.DB                // 读库
	dbw          *gorm.DB                // 写库
	ypClient     *lyg_util.YunPianClient // 云片客户端
	EsClient     *elastic.Client         // ES
	ExitImChan   chan int                // Im消息监控
	ExitChatChan chan int                // Im消息监控
}

// NewDao dao object
func NewDao(cfg config.Config, esClient *elastic.Client) *Dao {
	dao := new(Dao)
	dao.ExitImChan = make(chan int, 1)
	dao.ExitChatChan = make(chan int, 1)
	dao.EsClient = esClient
	dao.Cfg = cfg
	dao.Cpool = util.NewRedisPool(&cfg.Redis)
	dsnr := fmt.Sprintf("%s:%s@(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Mysql.User, cfg.Mysql.Psw, cfg.Mysql.Host.Read, cfg.Mysql.Port, cfg.Mysql.DbName)
	var err error
	dao.dbr, err = gorm.Open("mysql", dsnr)
	if err != nil {
		panic(err.Error())
	}
	dsnw := fmt.Sprintf("%s:%s@(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Mysql.User, cfg.Mysql.Psw, cfg.Mysql.Host.Write, cfg.Mysql.Port, cfg.Mysql.DbName)
	dao.dbw, err = gorm.Open("mysql", dsnw)
	if err != nil {
		panic(err.Error())
	}
	if cfg.Env != iconfig.ENV_PROD {
		dao.dbw.LogMode(true)
		dao.dbr.LogMode(true)
	}
	if cfg.YunPianApiKey != "" {
		dao.ypClient = lyg_util.NewYunPianClient(cfg.YunPianApiKey)
	} else {
		panic("云片ApiKey为空")
	}
	return dao
}

func (dao *Dao) GetPlayRedisPool() *redis.Pool {
	return dao.Cpool
}

type UserInfoV1 struct {
	UserID   int64  `json:"user_id"`
	GouHao   int64  `json:"gouhao"`
	NickName string `json:"nickname"`
	Gender   int64  `json:"gender"`
	Birthday int64  `json:"birthday"`
	LTS      int64  `json:"lts"`
}

func GetUserInfo(req *user_pb.InfoReq) (*user_pb.UserInfo, error) {
	resp, err := user_pb.Info(frame.TODO(), req)
	if err != nil {
		return nil, err
	} else if resp == nil || resp.GetData() == nil {
		return nil, fmt.Errorf("Not Found")
	}
	return resp.GetData(), nil
}

// 根据用户ID查询用户信息，封装user服务
func (dao *Dao) UserV1ByID(userID int64) (UserInfoV1, error) {
	var v1 UserInfoV1
	ret, err := GetUserInfo(&user_pb.InfoReq{UserId: userID})
	if err != nil {
		return v1, err
	}
	v1.UserID = ret.GetUserId()
	v1.GouHao = ret.GetGouhao()
	v1.NickName = ret.GetUsername()
	v1.Gender = int64(ret.GetGender())
	v1.Birthday = ret.GetBirthday()
	v1.LTS = ret.GetLts()
	return v1, nil
}

// 根据狗号查询用户信息，封装user服务
func (dao *Dao) UserV1ByGouHao(gouhao int64) (UserInfoV1, error) {
	var v1 UserInfoV1
	ret, err := GetUserInfo(&user_pb.InfoReq{GouHao: gouhao})
	if err != nil {
		return v1, err
	}
	v1.UserID = ret.GetUserId()
	v1.GouHao = ret.GetGouhao()
	v1.NickName = ret.GetUsername()
	v1.Gender = int64(ret.GetGender())
	return v1, nil
}

func (dao *Dao) GetRedisPool() *redis.Pool {
	return dao.Cpool
}
