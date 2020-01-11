package api

import (
	"fmt"
	"godgame/core"
	"iceberg/frame"
	"sort"
	"time"

	"gopkg.in/olivere/elastic.v5"
	"laoyuegou.com/util"
	"laoyuegou.pb/game/pb"
	"laoyuegou.pb/godgame/constants"
	"laoyuegou.pb/godgame/model"
	"laoyuegou.pb/godgame/pb"
	"laoyuegou.pb/imapi/pb"
	"laoyuegou.pb/plorder/pb"
	"laoyuegou.pb/user/pb"
)

// 获取对应appId,action的审核versions
func GetAuditVersions(action int64) (map[string]struct{}, error) {
	versionMap := make(map[string]struct{})
	resp, err := userpb.GetAuditVersions(frame.TODO(), &userpb.GetAuditVersionsReq{
		Action: action,
	})
	if err != nil || resp.GetErrcode() != 0 {
		return nil, err
	}
	if resp.GetData() != nil && len(resp.GetData()) > 0 {
		for _, v := range resp.GetData() {
			versionMap[FormatAppIdVersion(v.AppId, v.Version)] = struct{}{}
		}
	}
	return versionMap, nil
}

func FormatAppIdVersion(appId string, version int64) string {
	return fmt.Sprintf("%s:%d", appId, version)
}

func (gg *GodGame) Vcard(c frame.Context) error {
	var req godgamepb.VcardReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.RetBadRequestError(err.Error())
	}
	god := gg.dao.GetGod(req.GetGodId())
	if god.Status != constants.GOD_STATUS_PASSED {
		return c.RetSuccess("非大神用户", nil)
	}
	v1s, err := gg.dao.GetGodAllGameV1(req.GetGodId())
	if err != nil {
		c.Error(err.Error())
		return c.RetSuccess("大神信息获取异常", nil)
	}
	sort.Sort(v1s)

	appId, version, _ := c.GetClientInfo()
	resMap, err := GetAuditVersions(6)
	if err == nil {
		if _, ok := resMap[FormatAppIdVersion(appId, version)]; ok {
			return c.RetSuccess("success", nil)
		}
	}

	items := make([]*godgamepb.VcardResp_Data, 0, len(v1s))
	var item *godgamepb.VcardResp_Data
	for _, v1 := range v1s {
		if v1.GrabSwitch != constants.GRAB_SWITCH_OPEN {
			continue
		}
		item = new(godgamepb.VcardResp_Data)
		item.GameId = v1.GameID
		item.OrderCnt = v1.AcceptNum
		item.OrderCntDesc = FormatAcceptOrderNumber(v1.AcceptNum)
		item.OrderCntDesc2 = FormatAcceptOrderNumber3(v1.AcceptNum)
		if v1.PriceType == constants.PW_PRICE_TYPE_BY_OM {
			item.Price = FormatPriceV1(v1.PeiWanPrice)
		} else {
			cfgResp, err := gamepb.AcceptCfgV2(c, &gamepb.AcceptCfgV2Req{
				GameId: v1.GameID,
			})
			if err == nil || cfgResp.GetErrcode() == 0 {
				item.Price = FormatPriceV1(cfgResp.GetData().GetPrices()[v1.PriceID])
			}
		}
		if req.GetMore() {
			item.Score = FormatScore(v1.Score)
			if orderPercent, err := plorderpb.OrderFinishPercent(c, &plorderpb.OrderFinishPercentReq{
				GodId: v1.GodID,
				Days:  7,
			}); err == nil && orderPercent.GetErrcode() == 0 {
				item.OrderRate = orderPercent.GetData()
			}
			item.Desc = v1.Desc
		}
		items = append(items, item)
	}
	return c.RetSuccess("success", items)
}

// 获取语聊大神的单价
func (gg *GodGame) GetCallPrice(c frame.Context) error {
	var req godgamepb.GetCallPriceReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_INTERNAL, err.Error(), nil)
	} else if req.GetGodId() == 0 {
		return c.JSON2(ERR_CODE_INTERNAL, "invalid god_id", nil)
	}
	gameInfo, err := gamepb.GetVoiceCall(c, nil)
	if err != nil || gameInfo.GetErrcode() != 0 || gameInfo.GetData() == nil {
		return c.JSON2(ERR_CODE_INTERNAL, "invalid gameinfo", nil)
	}
	godGameV1, err := gg.dao.GetGodSpecialGameV1(req.GetGodId(), gameInfo.GetData().GetGameId())
	if err != nil {
		return c.JSON2(ERR_CODE_INTERNAL, err.Error(), nil)
	}
	var resp godgamepb.GetCallPriceResp_Data
	if godGameV1.PriceType == constants.PW_PRICE_TYPE_BY_OM {
		resp.PriceId = 0
		resp.PriceGl = godGameV1.PeiWanPrice
	} else {
		resp.PriceId = godGameV1.PriceID
		resp.PriceGl = gameInfo.GetData().GetPrices()[godGameV1.PriceID]
	}
	resp.GrabSwitch = godGameV1.GrabSwitch
	return c.JSON2(StatusOK_V3, "", &resp)
}

// 获取一键匹配语聊大神列表
func (gg *GodGame) GetCallGods(c frame.Context) error {
	gods, err := gg.dao.GetRandCallGods()
	if err != nil {
		c.Errorf("%s", err.Error())
		return c.JSON2(ERR_CODE_INTERNAL, "", nil)
	}
	return c.JSON2(StatusOK_V3, "", &godgamepb.GetCallGodsResp_Data{
		Gods: gods,
	})
}

// 重新计算大神等级
func (gg *GodGame) ReCalcGodLevel(c frame.Context) error {
	var req godgamepb.ReCalcGodLevelReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	err = gg.dao.ReCalcGodLevel(req.GetGodId(), req.GetGameId())
	if err != nil {
		return c.JSON2(ERR_CODE_INTERNAL, err.Error(), nil)
	}
	return c.JSON2(StatusOK_V3, "", nil)
}

// 获取指定游戏的大神列表数据
func (gg *GodGame) SimpleGodGame(c frame.Context) error {
	var req godgamepb.SimpleGodGameReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	} else if req.GetGameId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "invalid game id", nil)
	} else if len(req.GetGodIds()) > 40 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "too many gods. max 40", nil)
	}

	var v1 model.GodGameV1
	var godInfo model.God
	var userInfo *userpb.UserInfo
	var tmpData *godgamepb.SimpleGodGameResp_SimpleGodGame
	var tmpImgs []string
	var tmpCfgV2 *gamepb.AcceptCfgV2Resp
	data := make([]*godgamepb.SimpleGodGameResp_SimpleGodGame, 0, len(req.GetGodIds()))
	for _, godID := range req.GetGodIds() {
		v1, err = gg.dao.GetGodSpecialGameV1(godID, req.GetGameId())
		if err != nil {
			v1, err = gg.dao.GetGodSpecialBlockedGameV1(godID, req.GetGameId())
			if err != nil {
				continue
			}
		}
		godInfo = gg.dao.GetGod(godID)
		if godInfo.UserID != godID {
			continue
		}
		userInfo, err = gg.getSimpleUser(godID)
		if err != nil {
			continue
		}
		if err = json.Unmarshal([]byte(v1.Images), &tmpImgs); len(tmpImgs) == 0 {
			continue
		}
		tmpData = new(godgamepb.SimpleGodGameResp_SimpleGodGame)
		tmpData.GodId = godID
		tmpData.GameId = req.GetGameId()
		tmpData.GodName = userInfo.GetUsername()
		tmpData.Sex = godInfo.Gender
		tmpData.Age = int64(util.Age(userInfo.GetBirthday()))
		tmpData.Avatar = userInfo.GetAvatarSmall()
		tmpData.Voice = v1.Voice
		tmpData.VoiceDuration = v1.VoiceDuration
		tmpData.Aac = v1.Aac
		tmpData.GodImgs = []string{tmpImgs[0] + "/400"}
		if v1.PriceType == constants.PW_PRICE_TYPE_BY_OM {
			tmpData.Uniprice = v1.PeiWanPrice
			tmpData.Gl = FormatRMB2Gouliang(v1.PeiWanPrice)
		} else {
			tmpCfgV2, err = gamepb.AcceptCfgV2(frame.TODO(), &gamepb.AcceptCfgV2Req{
				GameId: req.GetGameId(),
			})
			if err == nil {
				tmpData.Uniprice = tmpCfgV2.GetData().GetPrices()[v1.PriceID]
				tmpData.Gl = FormatRMB2Gouliang(tmpCfgV2.GetData().GetPrices()[v1.PriceID])
			} else {
				continue
			}
		}
		tmpData.OrderCnt = v1.AcceptNum
		tmpData.OrderCntDesc = FormatAcceptOrderNumber(v1.AcceptNum)
		tmpData.Score = fmt.Sprintf("%.1f", float64(v1.Score))
		data = append(data, tmpData)
	}
	return c.JSON2(StatusOK_V3, "", data)
}

// 获取大神和陪玩游戏状态
func (gg *GodGame) GodGameStatus(c frame.Context) error {
	var req godgamepb.GodGameStatusReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	var data godgamepb.GodGameStatusResp_Status
	god := gg.dao.GetGod(req.GetGodId())

	if god.UserID > 0 {
		data.GodStatus = god.Status
	} else {
		data.GodStatus = constants.GOD_STATUS_UNAUTHED
		return c.JSON2(StatusOK_V3, "", &data)
	}
	if req.GetGameId() > 0 {
		godGame := gg.dao.GetGodGame(req.GetGodId(), req.GetGameId())
		if godGame.UserID > 0 {
			data.GameStatus = godGame.Status
			data.GrabStatus = godGame.GrabStatus
		} else {
			data.GameStatus = constants.GOD_GAME_STATUS_UNAUTHED
		}
		data.HighestLevelId = godGame.HighestLevelID
	}
	return c.JSON2(StatusOK_V3, "", &data)
}

// 根据大神ID获取大神信息
func (gg *GodGame) GodInfo(c frame.Context) error {
	var req godgamepb.GodInfoReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	} else if req.GetGodId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "god_id is empty", nil)
	}
	god := gg.dao.GetGod(req.GetGodId())
	if god.UserID == 0 {
		return c.JSON2(StatusOK_V3, "", &godgamepb.GodInfoResp_GodInfo{
			Status: constants.GOD_STATUS_UNAUTHED,
		})
	}
	return c.JSON2(StatusOK_V3, "", &godgamepb.GodInfoResp_GodInfo{
		Realname:   god.RealName,
		IdcardType: god.IDcardtype,
		Idcard:     god.IDcard,
		IdcardUrl:  god.IDcardurl,
		Phone:      god.Phone,
		Gender:     god.Gender,
		Status:     god.Status,
	})
}

// 获取满足订单端位要求的大神集合
func (gg *GodGame) OrderGods(c frame.Context) error {
	var req godgamepb.OrderGodsReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	checkResp, err := gamepb.Check(frame.TODO(), &gamepb.CheckReq{
		GameId:      req.GetGameId(),
		RegionId:    req.GetRegion2(),
		StartLevel1: req.GetStartLevel1(),
		EndLevel1:   req.GetEndLevel1(),
	})
	if err != nil || checkResp == nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	} else if checkResp.GetErrcode() != 0 {
		c.Errorf("check error:%s", checkResp.GetErrmsg())
		return c.JSON2(ERR_CODE_BAD_REQUEST, checkResp.GetErrmsg(), nil)
	}

	gods := gg.dao.GetOrderGods(req.GetGameId(), req.GetRegion2(), req.GetStartLevel1(), req.GetEndLevel1())
	return c.JSON2(StatusOK_V3, "", &godgamepb.OrderGodsResp_Data{
		Gods: gods,
	})
}

// 获取满足条件的即时约大神列表
func (gg *GodGame) JSYOrderGods(c frame.Context) error {
	var req godgamepb.JSYOrderGodsReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	gods := gg.dao.GetJSYOrderGods(req.GetGameId(), req.GetGender())
	return c.JSON2(StatusOK_V3, "", &godgamepb.JSYOrderGodsResp_Data{
		Gods: gods,
	})
}

// 获取大神所有接单设置
func (gg *GodGame) GodOrderSettings(c frame.Context) error {
	var req godgamepb.GodOrderSettingsReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	godGames, err := gg.dao.GetGodAllGameV1(req.GetGodId())
	if err != nil {
		return c.JSON2(ERR_CODE_FORBIDDEN, "", nil)
	}
	settings := make([]*godgamepb.GodOrderSettingsResp_OrderSettings, 0, len(godGames))
	for _, godGame := range godGames {
		settings = append(settings, &godgamepb.GodOrderSettingsResp_OrderSettings{
			GameId:         godGame.GameID,
			Regions:        godGame.Regions,
			Levels:         godGame.Levels,
			HighestLevelId: godGame.HighestLevelID,
			GrabStatus:     godGame.GrabStatus == constants.GRAB_STATUS_YES,
			GrabSwitch:     godGame.GrabSwitch,
			GrabSwitch2:    godGame.GrabSwitch2,
			GrabSwitch3:    godGame.GrabSwitch3,
			GrabSwitch4:    godGame.GrabSwitch4,
		})
	}
	return c.JSON2(StatusOK_V3, "", settings)
}

// 获取大神指定品类的接单设置数据
func (gg *GodGame) SpecialGodOrderSetting(c frame.Context) error {
	var req godgamepb.SpecialGodOrderSettingReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	} else if req.GetGodId() == 0 || req.GetGameId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	godGame, err := gg.dao.GetGodSpecialGameV1(req.GetGodId(), req.GetGameId())
	if err != nil {
		return c.JSON2(ERR_CODE_INTERNAL, err.Error(), nil)
	}
	if len(godGame.Regions) == 0 || len(godGame.Levels) == 0 {
		return c.JSON2(ERR_CODE_NOT_FOUND, "", nil)
	}
	return c.JSON2(StatusOK_V3, "", &godgamepb.SpecialGodOrderSettingResp_OrderSetting{
		GameId:         godGame.GameID,
		Regions:        godGame.Regions,
		Levels:         godGame.Levels,
		HighestLevelId: godGame.HighestLevelID,
		Level:          godGame.Level,
		PriceId:        godGame.PriceID,
		PriceType:      godGame.PriceType,
		PeiwanPrice:    godGame.PeiWanPrice,
		GrabSwitch:     godGame.GrabSwitch,
		GrabSwitch2:    godGame.GrabSwitch2,
		GrabSwitch3:    godGame.GrabSwitch3,
		GrabSwitch4:    godGame.GrabSwitch4,
	})
}

// 获取大神提现的会长信息
func (gg *GodGame) GetGodLeader(c frame.Context) error {
	var req godgamepb.GetGodLeaderReq
	if err := c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	god := gg.dao.GetGod(req.GetGodId())
	if god.LeaderSwitch == constants.GOD_LEADER_SWITCH_OPEN && god.LeaderID > 0 {
		leaderInfo := gg.dao.GetGodLeaderByID(god.LeaderID)
		if leaderInfo == nil {
			return c.JSON2(ERR_CODE_NOT_FOUND, "", nil)
		}
		return c.JSON2(StatusOK_V3, "", &godgamepb.GetGodLeaderResp_LeaderInfo{
			LeaderId: leaderInfo.ID,
			Realname: leaderInfo.RealName,
			Idcard:   leaderInfo.IDcard,
			Phone:    leaderInfo.Phone,
			Alipay:   leaderInfo.Alipay,
		})
	}
	return c.JSON2(ERR_CODE_NOT_FOUND, "", nil)
}

// 强制刷新被推荐到首页的，状态为已通过的大神的所有品类数据
// 兔叽，捞月狗iOS2.9.9版本没有陪玩，需要把大神从所有的大神池删除
func (gg *GodGame) RefreshGodAllGame(c frame.Context) error {
	var req godgamepb.RefreshGodAllGameReq
	if err := c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	if req.GetGodId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "参数错误", nil)
	}
	v1s, err := gg.dao.GetGodAllGameV1(req.GetGodId())
	if err != nil || len(v1s) == 0 {
		return c.JSON2(StatusOK_V3, "", nil)
	}
	godInfo := gg.dao.GetGod(req.GetGodId())
	if godInfo.ID == 0 {
		return c.JSON2(ERR_CODE_FORBIDDEN, "", nil)
	}
	userInfo, err := userpb.Info(c, &userpb.InfoReq{UserId: req.GetGodId()})
	if err != nil || userInfo.GetErrcode() != 0 {
		return c.JSON2(ERR_CODE_INTERNAL, "", nil)
	}
	redisConn := gg.dao.GetPlayRedisPool().Get()
	defer redisConn.Close()
	if req.GetAppid() == "1006" || (req.GetAppid() == "1001" && req.GetAppVersion() == "2.9.8") {
		for _, v1 := range v1s {
			redisConn.Do("ZREM", core.RKJSYGods(v1.GameID, godInfo.Gender), req.GetGodId())
			redisConn.Do("ZREM", core.RKJSYPaiDanGods(v1.GameID, godInfo.Gender), req.GetGodId())
			for _, region := range v1.Regions {
				for _, level := range v1.Levels {
					redisConn.Do("ZREM", core.GodsRedisKey3(v1.GameID, region, level), req.GetGodId())
				}
			}
		}
		return c.JSON2(StatusOK_V3, "", nil)
	}
	geoInfo, geoErr := userpb.Location(c, &userpb.LocationReq{
		UserId: req.GetGodId(),
	})
	var esGodGame model.ESGodGameRedefine
	for _, v1 := range v1s {
		if v1.Recommend == constants.RECOMMEND_YES {
			// 被推荐到首页的大神，刷新首页的最后在线时间
			if esGodGame, err = gg.BuildESGodGameData(v1.GodID, v1.GameID); err == nil {
				esGodGame.LTS = time.Now()
				if geoErr == nil && geoInfo.GetErrcode() == 0 {
					esGodGame.City = geoInfo.GetData().GetCity()
					esGodGame.District = geoInfo.GetData().GetDistrict()
					esGodGame.Location2 = elastic.GeoPointFromLatLon(geoInfo.GetData().GetLat(), geoInfo.GetData().GetLng())
				}
				gg.ESAddGodGame(esGodGame)
			}
		}
		if gg.isVoiceCallGame(v1.GameID) {
			// 语聊品类
			if v1.GrabSwitch == constants.GRAB_SWITCH_CLOSE {
				redisConn.Do("ZREM", core.RKVoiceCallGods(), v1.GodID)
			} else if v1.GrabSwitch4 == constants.GRAB_SWITCH4_OPEN {
				// 随机模式开关打开
				redisConn.Do("ZADD", core.RKVoiceCallGods(), 1, v1.GodID)
			} else {
				redisConn.Do("ZADD", core.RKVoiceCallGods(), 2, v1.GodID)
			}
		} else {
			if v1.GrabSwitch2 == constants.GRAB_SWITCH2_OPEN {
				redisConn.Do("ZADD", core.RKJSYGods(v1.GameID, godInfo.Gender), time.Now().Unix(), req.GetGodId())
			}
			if v1.GrabSwitch3 == constants.GRAB_SWITCH3_OPEN {
				redisConn.Do("ZADD", core.RKJSYPaiDanGods(v1.GameID, godInfo.Gender), time.Now().Unix(), req.GetGodId())
			}
		}
	}
	return c.JSON2(StatusOK_V3, "", nil)
}

// 给聊天室使用，查看大神卡片的陪玩信息，按照接单数排序
func (gg *GodGame) GetGodVCard(c frame.Context) error {
	var req godgamepb.GetGodVCardReq
	if err := c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	if req.GetGodId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	v1s, err := gg.dao.GetGodAllGameV1(req.GetGodId())
	if err != nil {
		return c.JSON2(ERR_CODE_INTERNAL, err.Error(), nil)
	}
	sort.Sort(v1s)
	rows := make([]*godgamepb.GetGodVCardResp_Row, 0, len(v1s))
	for _, v1 := range v1s {
		if v1.GrabSwitch != constants.GRAB_SWITCH_OPEN {
			continue
		}
		rows = append(rows, &godgamepb.GetGodVCardResp_Row{
			GameId:        v1.GameID,
			AcceptNum:     v1.AcceptNum,
			AcceptNumDesc: FormatAcceptOrderNumber2(v1.AcceptNum),
		})
	}
	return c.JSON2(StatusOK_V3, "", rows)
}

// 获取指定大神所有品类状态
func (gg *GodGame) GetGodAllGameStatus(c frame.Context) error {
	var req godgamepb.GetGodAllGameStatusReq
	if err := c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	if req.GetGodId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	godGames, _ := gg.dao.GetGodAllGameV1(req.GetGodId())
	blockedGodGames, _ := gg.dao.GetGodBlockedGameV1(req.GetGodId())
	if len(blockedGodGames) > 0 {
		godGames = append(godGames, blockedGodGames...)
	}
	data := make([]*godgamepb.GetGodAllGameStatusResp_GameStatus, 0, len(godGames))
	for _, game := range godGames {
		data = append(data, &godgamepb.GetGodAllGameStatusResp_GameStatus{
			GameId: game.GameID,
			Status: game.Status,
		})
	}
	return c.JSON2(StatusOK_V3, "", data)
}

// 根据用户ID查询如果是已通过大神并且有已通过的品类，则返回大神和品类列表；
func (gg *GodGame) GetSpecialGodGames(c frame.Context) error {
	var req godgamepb.GetSpecialGodGamesReq
	if err := c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	if req.GetGodId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "empty godid", nil)
	}
	userInfo, err := gg.getSimpleUser(req.GetGodId())
	if err != nil || userInfo == nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "invalid user", nil)
	} else if userInfo.GetInvalid() != userpb.USER_INVALID_NO {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "invalid user", nil)
	}
	godInfo := gg.dao.GetGod(req.GetGodId())
	if godInfo.Status != constants.GOD_STATUS_PASSED {
		return c.JSON2(StatusOK_V3, "", &godgamepb.GetSpecialGodGamesResp_Data{
			Status: constants.GOD_STATUS_UNAUTHED,
		})
	}
	v1s, err := gg.dao.GetGodAllGameV1(req.GetGodId())
	if err != nil {
		return c.JSON2(ERR_CODE_INTERNAL, err.Error(), nil)
	}
	resp := &godgamepb.GetSpecialGodGamesResp_Data{
		Status: constants.GOD_STATUS_PASSED,
	}
	games := make([]*godgamepb.GetSpecialGodGamesResp_Data_GameInfo, 0, len(v1s))
	for _, v1 := range v1s {
		recordResp, err := gamepb.Record(frame.TODO(), &gamepb.RecordReq{GameId: v1.GameID})
		if err != nil || recordResp.GetErrcode() != 0 {
			continue
		}
		games = append(games, &godgamepb.GetSpecialGodGamesResp_Data_GameInfo{
			GameId:   v1.GameID,
			GameName: recordResp.GetData().GetGameName(),
			Status:   v1.Status,
		})
	}
	resp.Games = games
	return c.JSON2(StatusOK_V3, "", resp)
}

// 根据传入的大神+游戏ID，批量查询对应的正常状态的大神和陪玩信息
// 用户状态：非拉黑 大神状态：已通过 游戏状态：已通过
func (gg *GodGame) BatchGetSpecialGodGame(c frame.Context) error {
	var req godgamepb.BatchGetSpecialGodGameReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	if len(req.GetItems()) == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "empty params", nil)
	}
	items := make([]*godgamepb.BatchGetSpecialGodGameResp_Data, 0, len(req.GetItems()))
	var userInfo *userpb.UserInfo
	var v1 model.GodGameV1
	var godInfo model.God
	var resp *gamepb.AcceptCfgV2Resp
	var uniprice int64
	for _, item := range req.GetItems() {
		userInfo, err = gg.getSimpleUser(item.GetGodId())
		if err != nil || userInfo == nil {
			continue
		} else if userInfo.GetInvalid() != userpb.USER_INVALID_NO {
			continue
		}
		godInfo = gg.dao.GetGod(item.GetGodId())
		if godInfo.Status != constants.GOD_STATUS_PASSED {
			continue
		}
		v1, err = gg.dao.GetGodSpecialGameV1(item.GetGodId(), item.GetGameId())
		if err != nil || v1.GrabSwitch != constants.GRAB_SWITCH_OPEN {
			continue
		}
		uniprice = 0
		if v1.PriceType == constants.PW_PRICE_TYPE_BY_OM {
			uniprice = v1.PeiWanPrice
		} else {
			resp, err = gamepb.AcceptCfgV2(c, &gamepb.AcceptCfgV2Req{
				GameId: item.GetGameId(),
			})
			if err == nil && resp.GetErrcode() == 0 {
				uniprice = resp.GetData().GetPrices()[v1.PriceID]
			}
		}
		items = append(items, &godgamepb.BatchGetSpecialGodGameResp_Data{
			GodId:         v1.GodID,
			Avatar:        userInfo.GetAvatarBig(),
			GodName:       userInfo.GetUsername(),
			Gender:        godInfo.Gender,
			GameId:        v1.GameID,
			Voice:         v1.Voice,
			VoiceDuration: v1.VoiceDuration,
			Aac:           v1.Aac,
			Status:        v1.Status,
			OrderCnt:      v1.AcceptNum,
			Uniprice:      uniprice,
		})
	}
	return c.JSON2(StatusOK_V3, "", items)
}

func (gg *GodGame) GetGodWeight(c frame.Context) error {
	var req godgamepb.GetGodWeightReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	if req.GetGodId() == 0 || req.GetGameId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "empty id", nil)
	}
	return c.JSON2(StatusOK_V3, "", gg.dao.GetGodGameWeight(req.GetGodId(), req.GetGameId()))
}

func (gg *GodGame) IsGod2(c frame.Context) error {
	var req godgamepb.IsGod2Req
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	if gg.dao.IsGod(req.GetGodId()) {
		return c.JSON2(StatusOK_V3, "", nil)
	}
	return c.JSON2(ERR_CODE_NOT_FOUND, "", nil)
}

func (gg *GodGame) Paidan(c frame.Context) error {
	var req godgamepb.PaidanReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	} else if req.GetGameId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "参数错误[1]", nil)
	} else if req.GetRoomId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "参数错误[2]", nil)
	} else if req.GetSeq() == "" {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "参数错误[3]", nil)
	} else if req.GetRoomTemplate() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "参数错误[4]", nil)
	}
	gods := gg.dao.GetJSYPaiDanGods(req.GetGameId(), req.GetGender())
	if len(gods) == 0 {
		return c.JSON2(StatusOK_V3, "暂无空闲大神", 0)
	}
	var gameName string
	if gameInfo, err := gamepb.Record(c, &gamepb.RecordReq{GameId: req.GetGameId()}); err == nil && gameInfo.GetErrcode() == 0 {
		gameName = gameInfo.GetData().GetGameName()
	}
	gender := constants.GENDER_DESC[req.GetGender()]
	if gender == "" {
		gender = "不限"
	}
	title := "收到新的" + gameName + "派单"
	msg := map[string]interface{}{
		"room_id":    req.GetRoomId(),
		"game_id":    req.GetGameId(),
		"title":      title,
		"gender":     gender,
		"desc":       req.GetDesc(),
		"pd_id":      req.GetSeq(),
		"room_title": req.GetRoomTitle(),
		"template":   req.GetRoomTemplate(),
	}
	bs, err := json.Marshal(msg)
	if err != nil {
		c.Errorf("%s", err.Error())
		return c.JSON2(ERR_CODE_INTERNAL, "内部错误[3]", nil)
	}
	apn := imapipb.PushNotification{
		Desc:  title,
		Sound: "default",
	}
	apnBs, _ := json.Marshal(apn)
	resp, err := imapipb.BatchSendSystemNotify(c, &imapipb.BatchSendSystemNotifyReq{
		Subtype: 9026,
		Message: string(bs),
		Apn:     string(apnBs),
		Ttl:     3600,
		Bt:      imapipb.BatchType_BT_BY_IDS,
		Fanout:  gods,
	})
	if err != nil {
		c.Errorf("%s", err.Error())
		return c.JSON2(ERR_CODE_INTERNAL, "内部错误[4]", nil)
	} else if resp.GetErrcode() != 0 {
		c.Errorf("%s", resp.GetErrmsg())
		return c.JSON2(ERR_CODE_INTERNAL, "内部错误[5]", nil)
	}
	return c.JSON2(StatusOK_V3, "", &godgamepb.PaidanResp_Data{
		Gods:  gods,
		Count: int64(len(gods)),
	})
}

func (gg *GodGame) GetGodGameInfo(c frame.Context) error {
	var req godgamepb.GetGodGameInfoReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	} else if req.GetGodId() == 0 || req.GetGameId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "参数错误[1]", nil)
	}
	god := gg.dao.GetGod(req.GetGodId())
	if god.Status != constants.GOD_STATUS_PASSED {
		return c.JSON2(StatusOK_V3, "1", nil)
	}
	v1, err := gg.dao.GetGodSpecialGameV1(req.GetGodId(), req.GetGameId())
	if err != nil {
		c.Errorf("%s", err.Error())
		return c.JSON2(StatusOK_V3, "2", nil)
	}
	if v1.GrabSwitch != constants.GRAB_SWITCH_OPEN {
		return c.JSON2(StatusOK_V3, "3", nil)
	}

	acceptResp, err := gamepb.Accept(c, &gamepb.AcceptReq{
		GameId:   v1.GameID,
		AcceptId: v1.HighestLevelID,
	})
	if err != nil || acceptResp.GetErrcode() != 0 {
		return c.JSON2(StatusOK_V3, "4", nil)
	}

	var uniprice int64
	if v1.PriceType == constants.PW_PRICE_TYPE_BY_OM {
		uniprice = v1.PeiWanPrice
	} else {
		cfgResp, err := gamepb.AcceptCfgV2(frame.TODO(), &gamepb.AcceptCfgV2Req{
			GameId: v1.GameID,
		})
		if err != nil || cfgResp.GetErrcode() != 0 {
			return c.JSON2(StatusOK_V3, "5", nil)
		}
		uniprice = cfgResp.GetData().GetPrices()[v1.PriceID]
	}
	resp := &godgamepb.GetGodGameInfoResp_Data{
		Gl:       FormatRMB2Gouliang(uniprice),
		PwPrice:  FormatPriceV1(uniprice),
		GameName: acceptResp.GetData().GetGameName(),
	}
	return c.JSON2(StatusOK_V3, "", resp)
}

func (gg *GodGame) InternalGodGame(c frame.Context) error {
	var req godgamepb.InternalGodGameReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	godInfo := gg.dao.GetGod(req.GetGodId())
	if godInfo.Status != constants.GOD_STATUS_PASSED {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	v1, err := gg.dao.GetGodSpecialGameV1(req.GetGodId(), req.GetGameId())
	if err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	} else if v1.Status != constants.GOD_GAME_STATUS_PASSED {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	} else if v1.GrabSwitch != constants.GRAB_SWITCH_OPEN {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	return c.JSON2(StatusOK_V3, "", &godgamepb.InternalGodGameResp_Data{
		GodStatus:  godInfo.Status,
		GodGender:  godInfo.Gender,
		GameStatus: v1.Status,
		GrabSwitch: v1.GrabSwitch,
	})
}

func (gg *GodGame) InternalApplyGames(c frame.Context) error {
	var req godgamepb.InternalApplyGamesReq
	if err := c.Bind(&req); err != nil {
		return c.RetBadRequestError(err.Error())
	}
	listResp, err := gamepb.ListV2(c, &gamepb.ListV2Req{})
	if err != nil || listResp.GetErrcode() != 0 {
		return c.RetInternalError("")
	}
	map1 := gg.dao.GetGodGameStatus(req.GetGodId())
	data := make([]*godgamepb.InternalApplyGamesResp_Data, 0, len(listResp.GetData()))
	var ok bool
	var status int64
	for _, game := range listResp.GetData() {
		if status, ok = map1[game.GetGameId()]; !ok {
			data = append(data, &godgamepb.InternalApplyGamesResp_Data{
				GameId:     game.GetGameId(),
				GameName:   game.GetGameName(),
				GameAvatar: game.GetGameAvatar(),
				Status:     constants.GOD_GAME_STATUS_UNAUTHED,
			})
		} else {
			data = append(data, &godgamepb.InternalApplyGamesResp_Data{
				GameId:     game.GetGameId(),
				GameName:   game.GetGameName(),
				GameAvatar: game.GetGameAvatar(),
				Status:     status,
			})
		}

	}
	return c.RetSuccess("", data)
}

func (gg *GodGame) SimpleGodGames(c frame.Context) error {
	var req godgamepb.SimpleGodGamesReq
	if err := c.Bind(&req); err != nil || req.GetGodId() <= 0 {
		return c.RetBadRequestError("")
	}
	return c.RetSuccess("success", gg.dao.SimpleGodGames(req.GetGodId(), req.GetHidePrice()))
}

// SimpleGodGameIds 返回大神已通过的品类ID列表，按品类ID升序
func (gg *GodGame) SimpleGodGameIds(c frame.Context) error {
	var req godgamepb.SimpleGodGameIdsReq
	if err := c.Bind(&req); err != nil || req.GetGodId() <= 0 {
		return c.RetBadRequestError("")
	}
	godInfo := gg.dao.GetGod(req.GetGodId())
	return c.RetSuccess("success", &godgamepb.SimpleGodGameIdsResp_Data{
		Gender:  godInfo.Gender,
		GameIds: gg.dao.SimpleGodGameIds(req.GetGodId()),
	})
}

// 大神定向单接单设置数据查询   php一元购活动专用  后面去掉该接口
func (gg *GodGame) DxdInternal(c frame.Context) error {
	var req godgamepb.DxdReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	} else if req.GetGodId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	v1s, _ := gg.dao.GetGodAllGameV1(req.GetGodId())
	if len(v1s) == 0 {
		return c.JSON2(ERR_CODE_GOD_ACCEPT_SETTING_LOAD_FAIL, errGodAcceptSettingLoadFail, nil)
	}
	games := make([]map[string]interface{}, 0, len(v1s))
	var game map[string]interface{}
	var dxdResp *gamepb.DxdResp
	// isIOS := gg.isIOS(c)
	for _, v1 := range v1s {
		if v1.GrabSwitch != constants.GRAB_SWITCH_OPEN {
			continue
		} else if gg.isVoiceCallGame(v1.GameID) {
			// 语聊品类不展示在下定向单页面
			continue
		}
		dxdResp, err = gamepb.Dxd(c, &gamepb.DxdReq{
			GameId:  v1.GameID,
			Region2: v1.Regions,
		})
		if err != nil || dxdResp.GetErrcode() != 0 {
			continue
		}
		game = make(map[string]interface{})
		game["game_id"] = v1.GameID
		game["highest_level_score"] = dxdResp.GetData().GetHighestLevelScore()
		game["service_type"] = dxdResp.GetData().GetServiceId()
		game["service_name"] = dxdResp.GetData().GetServiceName()
		game["regions"] = v1.Regions
		game["region1"] = dxdResp.GetData().GetRegion1()

		games = append(games, game)
	}
	if len(games) == 0 {
		return c.JSON2(ERR_CODE_GOD_ACCEPT_SETTING_LOAD_FAIL, errGodAcceptSettingLoadFail, nil)
	}
	return c.JSON2(StatusOK_V3, "", map[string]interface{}{
		"god_id": req.GetGodId(),
		"games":  games,
	})
}

// 获取大神接单最多的语音介绍和时长
func (gg *GodGame) GodMostOrderVoice(c frame.Context) error {
	var req godgamepb.GodMostOrderVoiceReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.RetBadRequestError(err.Error())
	}
	god := gg.dao.GetGod(req.GetGodId())
	if god.Status != constants.GOD_STATUS_PASSED {
		return c.RetSuccess("非大神用户", nil)
	}
	v1s, err := gg.dao.GetGodAllGames(req.GetGodId())
	if err != nil {
		c.Error(err.Error())
		return c.RetSuccess("大神信息获取异常", nil)
	}
	var resp godgamepb.GodMostOrderVoiceResp
	if len(v1s) > 0 {
		sort.Slice(v1s, func(i, j int) bool {
			return v1s[i].AcceptNum > v1s[j].AcceptNum
		})
		for _, v := range v1s {
			if v.GrabSwitch == 1 {
				resp.Data = &godgamepb.GodMostOrderVoiceResp_Data{
					Voice:         v.Aac,
					VoiceDuration: v.VoiceDuration,
				}
				return c.RetSuccess("success", resp.Data)
			}
		}
	}
	return c.RetSuccess("success", nil)
}
