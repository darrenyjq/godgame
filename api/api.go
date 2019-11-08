package api

import (
	"fmt"
	"github.com/json-iterator/go"
	"github.com/olivere/elastic"
	shence "github.com/sensorsdata/sa-sdk-go"
	"godgame/config"
	"godgame/core"
	"godgame/handlers"
	"iceberg/frame"
	"iceberg/frame/icelog"
	"laoyuegou.com/httpkit/lyghttp/middleware"
	"laoyuegou.pb/game/pb"
	"laoyuegou.pb/godgame/constants"
	"laoyuegou.pb/godgame/model"
	"laoyuegou.pb/godgame/pb"
	user_pb "laoyuegou.pb/user/pb"
	"os"
	"strconv"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// GodGame God Game服务
type GodGame struct {
	dao              *core.Dao
	cfg              config.Config
	esClient         *elastic.Client
	esChan           chan ESParams
	esQuickOrderChan chan ESOrderParams
	shence           shence.SensorsAnalytics
	nsqHandler       *handlers.BaseHandler
	exitChan         chan struct{}
}

// NewGodGame new God Game
func NewGodGame(cfg config.Config) *GodGame {
	gg := new(GodGame)
	gg.cfg = cfg
	gg.dao = core.NewDao(cfg)
	shenceConsumer, _ := shence.InitDefaultConsumer(cfg.Shence.URL, cfg.Shence.Timeout)
	gg.shence = shence.InitSensorsAnalytics(shenceConsumer, cfg.Shence.Project, false)
	esOptions := []elastic.ClientOptionFunc{
		elastic.SetURL(cfg.ES.Host...),
		elastic.SetSniff(false),
		elastic.SetMaxRetries(10),
	}
	if cfg.ES.Username != "" && cfg.ES.Password != "" {
		esOptions = append(esOptions, elastic.SetBasicAuth(cfg.ES.Username, cfg.ES.Password))
	}
	esClient, err := elastic.NewClient(esOptions...)
	if err != nil {
		icelog.Errorf("Init esClient error:%s", err)
	} else {
		gg.esClient = esClient
	}
	gg.esChan = make(chan ESParams, 100)
	gg.esQuickOrderChan = make(chan ESOrderParams, 100)
	gg.nsqHandler = handlers.NewBaseHandler(cfg, gg.dao)
	gg.exitChan = make(chan struct{})
	go gg.StartLoop()
	go gg.StartQuickOrderLoop()
	go gg.fill_god_list()
	return gg
}

func (gg *GodGame) Stop(s os.Signal) bool {
	gg.nsqHandler.Stop()
	close(gg.exitChan)
	return true
}

func (gg *GodGame) getCurrentUserID(c frame.Context) int64 {
	uid, _ := strconv.ParseInt(c.Header().Get("userId"), 10, 64)
	return uid
}

func (gg *GodGame) getUserAppID(c frame.Context) string {
	_, _, _, appid := middleware.ClientInfo(c.GetHeaderString("Client-Info", ""))
	return appid
}

func (gg *GodGame) isIOS(c frame.Context) bool {
	p, _, _, _ := middleware.ClientInfo(c.GetHeaderString("Client-Info", ""))
	return p == "iOSApp"
}

func (gg *GodGame) getUserAppVersion(c frame.Context) string {
	_, v, _, _ := middleware.ClientInfo(c.GetHeaderString("Client-Info", ""))
	return v
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
	currentUser.AppVersionNum = uinfo.GetAppVer()
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

func (gg *GodGame) getSimpleUser2(userID int64) (*user_pb.SimpleInfo, error) {
	if userID == 0 {
		return nil, fmt.Errorf("invalid userid")
	}
	resp, err := user_pb.Simple2(frame.TODO(), &user_pb.Simple2Req{UserId: userID})
	if err != nil {
		return nil, err
	} else if resp.GetErrcode() != 0 {
		return nil, fmt.Errorf("%s", resp.GetErrmsg())
	}
	return resp.GetData(), nil
}

// 判断是否为语聊品类
func (gg *GodGame) isVoiceCallGame(gameID int64) bool {
	resp, err := gamepb.VoiceCall(frame.TODO(), &gamepb.VoiceCallReq{
		GameId: gameID,
	})
	if err == nil && resp.GetErrcode() == 0 {
		return resp.GetData()
	}
	return false
}

func (gg *GodGame) AcceptQuickOrder(c frame.Context) error {
	var in godgamepb.AcceptQuickOrderReq
	if err := c.Bind(&in); err != nil || in.GodId == 0 || in.GameId == 0 || in.GrabSwitch == 0 {
		return c.RetBadRequestError("params fails")
	}
	if in.GrabSwitch == constants.GRAB_SWITCH5_OPEN {
		var data model.ESQuickOrder
		data, err := gg.BuildESQuickOrder(in.GodId, in.GameId)
		if err != nil {
			return c.RetBadRequestError(err.Error())
		}
		gg.ESAddQuickOrder(data)
	} else {
		esId := fmt.Sprintf("%d-%d", in.GodId, in.GameId)
		gg.ESDeleteQuickOrder([]string{esId})
	}
	return c.JSON2(StatusOK_V3, "success", nil)
}

func (gg *GodGame) QueryQuickOrder(c frame.Context) error {

	var in godgamepb.QueryQuickOrderReq
	if err := c.Bind(&in); err != nil {
		return c.RetBadRequestError("params fails")
	}

	data := gg.ESQueryQuickOrder(in)
	// data := 1
	return c.JSON2(StatusOK_V3, "success", data)
}

// func (gg *GodGame) 1  {
//
// }
