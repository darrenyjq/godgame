package api

import (
	"fmt"
	"github.com/olivere/elastic"
	shence "github.com/sensorsdata/sa-sdk-go"
	"iceberg/frame"
	"iceberg/frame/icelog"
	"laoyuegou.com/httpkit/lyghttp/middleware"
	"laoyuegou.com/keyword"
	"laoyuegou.pb/godgame/model"
	user_pb "laoyuegou.pb/user/pb"
	"play/common/imclient"
	"play/godgame/config"
	"play/godgame/core"
	"strconv"
)

// GodGame God Game服务
type GodGame struct {
	dao           *core.Dao
	cfg           config.Config
	msgSender     *imclient.MsgSender
	esClient      *elastic.Client
	esChan        chan ESParams
	shence        shence.SensorsAnalytics
	keywordFilter *keyword.Filter
}

// NewGodGame new God Game
func NewGodGame(cfg config.Config) *GodGame {
	gg := new(GodGame)
	gg.cfg = cfg
	gg.dao = core.NewDao(cfg)
	shenceConsumer, _ := shence.InitDefaultConsumer(cfg.Shence.URL, cfg.Shence.Timeout)
	gg.shence = shence.InitSensorsAnalytics(shenceConsumer, cfg.Shence.Project, false)
	gg.msgSender = imclient.NewMsgSender(cfg.IM.Addr, cfg.IM.AppID, cfg.IM.AppToken)
	esClient, err := elastic.NewClient(
		elastic.SetURL(cfg.ES.Host...),
		elastic.SetMaxRetries(10))
	if err != nil {
		icelog.Errorf("Init esClient error:%s", err)
	} else {
		gg.esClient = esClient
	}
	gg.esChan = make(chan ESParams, 100)
	go gg.StartLoop()
	return gg
}

func (gg *GodGame) getCurrentUserID(c frame.Context) int64 {
	uid, _ := strconv.ParseInt(c.Header().Get("userId"), 10, 64)
	return uid
}

func (gg *GodGame) getUserAppID(c frame.Context) string {
	_, _, _, appid := middleware.ClientInfo(c.GetHeaderString("Client-Info", ""))
	return appid
}

func (gg *GodGame) getCurrentUser(c frame.Context) model.CurrentUser {
	var currentUser model.CurrentUser
	uid, _ := strconv.ParseInt(c.Header().Get("userId"), 10, 64)
	if uid == 0 {
		return currentUser
	}

	uinfo, err := gg.getSimpleUser(uid)
	if err != nil || uinfo == nil {
		return currentUser
	}
	currentUser.UserID = uinfo.GetUserId()
	currentUser.GouHao = uinfo.GetGouhao()
	currentUser.UserName = uinfo.GetUsername()
	currentUser.Gender = int64(uinfo.GetGender())
	currentUser.Birthday = uinfo.GetBirthday()
	currentUser.UpdateTime = uinfo.GetUpdateTime()
	currentUser.GameIds = uinfo.GetGameIds()
	currentUser.Invalid = uinfo.GetInvalid()
	currentUser.AppVersion = uinfo.GetAppVersion()
	currentUser.Platform = uinfo.GetAppForm()
	currentUser.AppID = uinfo.GetAppId()
	return currentUser
}

func (gg *GodGame) getSimpleUser(userID int64) (*user_pb.UserInfo, error) {
	resp, err := user_pb.Info(frame.TODO(), &user_pb.InfoReq{UserId: userID})
	if err != nil {
		return nil, err
	} else if resp.GetErrcode() != 0 {
		return nil, fmt.Errorf("%s", resp.GetErrmsg())
	} else if resp.GetData() == nil {
		return nil, fmt.Errorf("not found")
	}
	return resp.GetData(), nil
}
