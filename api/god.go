package api

import (
	"context"
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/olivere/elastic"
	"godgame/core"
	"iceberg/frame"
	"iceberg/frame/icelog"
	lyg_util "laoyuegou.com/util"
	"laoyuegou.pb/chatroom/pb"
	"laoyuegou.pb/follow/pb"
	game_const "laoyuegou.pb/game/constants"
	"laoyuegou.pb/game/pb"
	"laoyuegou.pb/godgame/constants"
	"laoyuegou.pb/godgame/model"
	"laoyuegou.pb/godgame/pb"
	"laoyuegou.pb/imapi/pb"
	"laoyuegou.pb/keyword/pb"
	"laoyuegou.pb/lfs/pb"
	"laoyuegou.pb/live/pb"
	plcommentpb "laoyuegou.pb/plcomment/pb"
	order_const "laoyuegou.pb/plorder/constants"
	"laoyuegou.pb/plorder/pb"
	"laoyuegou.pb/pumpkin/pb"
	user_pb "laoyuegou.pb/user/pb"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	page_size = 100
)

func (gg *GodGame) RandCall(c frame.Context) error {
	p := c.GetInt("p", 1)
	var items []map[string]interface{}
	data := map[string]interface{}{
		"count": 0,
		"items": items,
	}
	start := (p - 1) * page_size
	stop := start + page_size - 1
	gameInfo, err := gamepb.GetVoiceCall(c, nil)
	if err != nil || gameInfo.GetErrcode() != 0 || gameInfo.GetData() == nil || gameInfo.GetData().GetPrices() == nil {
		c.Errorf("获取语聊品类信息失败 %v %v", gameInfo, err)
		return c.JSON2(StatusOK_V3, "", data)
	}
	gods, err := gg.dao.GetRandCallGods2(start, stop)
	if err != nil {
		c.Error(err.Error())
		return c.JSON2(StatusOK_V3, "", data)
	} else if len(gods) == 0 {
		return c.JSON2(StatusOK_V3, "", data)
	}
	gameID := gameInfo.GetData().GetGameId()
	var userInfo *user_pb.GetUserResp
	var lts *pumpkinpb.UserStatusResp
	var godGameV1 model.GodGameV1
	onlineItems := make([]map[string]interface{}, 0, len(gods)/2)
	offlineItems := make([]map[string]interface{}, 0, len(gods)/2)
	var tmpItem map[string]interface{}
	appID := gg.getUserAppID(c)
	for _, godID := range gods {
		userInfo, err = user_pb.GetUser(c, &user_pb.GetUserReq{
			UserId: godID,
		})
		if err != nil || userInfo.GetErrcode() != 0 {
			continue
		}
		godGameV1, err = gg.dao.GetGodSpecialGameV1(godID, gameID)
		if err != nil {
			c.Error(err.Error())
			continue
		}
		tmpItem = make(map[string]interface{})
		tmpItem = map[string]interface{}{
			"user_id":        userInfo.GetData().GetUserId(),
			"username":       userInfo.GetData().GetUsername(),
			"avatar":         userInfo.GetData().GetAvatar(),
			"voice":          godGameV1.Voice,
			"aac":            godGameV1.Aac,
			"voice_duration": godGameV1.VoiceDuration,
			"desc":           godGameV1.Desc,
		}
		if appID == user_pb.APP_ID_ANDROID_LAOYUEGOU {
			// Android
			if godGameV1.PriceType == constants.PW_PRICE_TYPE_BY_OM {
				tmpItem["price"] = godGameV1.PeiWanPrice
				tmpItem["price_unit"] = "狗粮/分钟"
			} else {
				tmpItem["price"] = gameInfo.GetData().GetPrices()[godGameV1.PriceID]
				tmpItem["price_unit"] = "狗粮/分钟"
			}
		} else if appID == user_pb.APP_ID_IOS_TANSUO_LAOYUEGOU {
			// iOS探索版
			// if godGameV1.PriceType == constants.PW_PRICE_TYPE_BY_OM {
			// 	tmpItem["price"] = godGameV1.PeiWanPrice
			// 	tmpItem["price_unit"] = "狗粮/分钟"
			// } else {
			// 	tmpItem["price"] = gameInfo.GetData().GetPrices()[godGameV1.PriceID]
			// 	tmpItem["price_unit"] = "狗粮/分钟"
			// }
		}
		lts, err = pumpkinpb.UserStatus(c, &pumpkinpb.UserStatusReq{
			UserId: godID,
		})
		if err == nil && lts.GetErrcode() == 0 {
			if lts.GetData().GetStatus() == int32(imapipb.USER_ONLINE_STATUS_USER_ONLINE_STATUS_ONLINE) {
				tmpItem["status"] = constants.GOD_STATUS_ONLINE
				onlineItems = append(onlineItems, tmpItem)
			} else if lts.GetData().GetStatus() == int32(imapipb.USER_ONLINE_STATUS_USER_ONLINE_STATUS_OFFLINE) {
				tmpItem["status"] = constants.GOD_STATUS_OFFLINE
				offlineItems = append(offlineItems, tmpItem)
			} else if lts.GetData().GetStatus() == int32(imapipb.USER_ONLINE_STATUS_USER_ONLINE_STATUS_BUSY_LINE) {
				tmpItem["status"] = constants.GOD_STATUS_LINE_BUSY
				onlineItems = append(onlineItems, tmpItem)
			}
		} else {
			tmpItem["status"] = constants.GOD_STATUS_OFFLINE
			offlineItems = append(offlineItems, tmpItem)
		}
	}
	data["count"] = len(onlineItems) + len(offlineItems)
	data["items"] = append(onlineItems, offlineItems...)
	go gg.shence.Track(fmt.Sprintf("%d", gg.getCurrentUserID(c)),
		"flip",
		map[string]interface{}{
			"scene": "1v1大神列表",
			"page":  strconv.Itoa(p),
		}, true)
	return c.JSON2(StatusOK_V3, "", data)
}

func (gg *GodGame) Chat(c frame.Context) error {
	var req godgamepb.ChatReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	} else if req.GetGodId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	god := gg.dao.GetGod(req.GetGodId())
	if god.Status != constants.GOD_STATUS_PASSED {
		return c.JSON2(StatusOK_V3, "", nil)
	}
	if god.Desc != "" {
		if req.GetT() == 0 || req.GetT() < gg.dao.GetGodLastModifyDescTimestamp(req.GetGodId()) {
			// 发送大神自定义介绍给用户
			if currentUserID := gg.getCurrentUserID(c); currentUserID > 0 {
				go imapipb.SendMessage(c, &imapipb.SendMessageReq{
					Thread:      lyg_util.CreatePrivateMessageThread(req.GetGodId(), currentUserID).String(),
					FromId:      req.GetGodId(),
					ToId:        currentUserID,
					ContentType: imapipb.MESSAGE_CONTENT_TYPE_TEXT,
					Subtype:     imapipb.MESSAGE_SUBTYPE_CHAT,
					Message:     god.Desc,
					Apn:         "",
					Ttl:         1,
					Fanout:      []int64{},
				})
			}
		}
	}
	v1s, err := gg.dao.GetGodAllGameV1(req.GetGodId())
	if err != nil {
		c.Error(err.Error())
		return c.JSON2(StatusOK_V3, "", nil)
	}
	sort.Sort(v1s)
	items := make([]map[string]interface{}, 0, len(v1s))
	var uniprice int64
	for _, v1 := range v1s {
		if v1.GrabSwitch != constants.GRAB_SWITCH_OPEN {
			continue
		}
		if gg.isVoiceCallGame(v1.GameID) {
			// 语聊品类不展示
		} else {
			tmpData := make(map[string]interface{})
			if v1.PriceType == constants.PW_PRICE_TYPE_BY_OM {
				uniprice = v1.PeiWanPrice
			} else {
				cfgResp, err := gamepb.AcceptCfgV2(frame.TODO(), &gamepb.AcceptCfgV2Req{
					GameId: v1.GameID,
				})
				if err != nil || cfgResp.GetErrcode() != 0 {
					continue
				}
				uniprice = cfgResp.GetData().GetPrices()[v1.PriceID]
			}
			tmpData["pw_price"] = FormatPriceV1(uniprice)
			tmpData["gl"] = FormatRMB2Gouliang(uniprice)
			tmpData["game_id"] = v1.GameID
			tmpData["score"] = FormatScore(v1.Score)
			acceptResp, err := gamepb.Accept(c, &gamepb.AcceptReq{
				GameId:   v1.GameID,
				AcceptId: v1.HighestLevelID,
			})
			if err == nil && acceptResp.GetErrcode() == 0 {
				tmpData["highest_level_desc"] = acceptResp.GetData().GetName()
				tmpData["game_name"] = acceptResp.GetData().GetGameName()
			}
			tmpData["accept_num"] = FormatAcceptOrderNumber(v1.AcceptNum)
			tmpData["desc"] = FormatAcceptOrderNumber3(v1.AcceptNum)
			liveResp, err := livepb.GetGodLiveId(c, &livepb.GetGodLiveIdReq{
				GodId:  v1.GodID,
				GameId: v1.GameID,
			})
			if err == nil && liveResp.GetData() != nil && liveResp.GetData().GetRoomId() > 0 {
				// 优先返回直播
				tmpData["pw_status"] = order_const.PW_STATUS_LIVE
				tmpData["room_id"] = liveResp.GetData().GetRoomId()
			} else {
				tmpData["pw_status"] = order_const.PW_STATUS_FREE
			}
			tmpData["status"] = v1.Status
			items = append(items, tmpData)
		}
	}
	return c.JSON2(StatusOK_V3, "", map[string]interface{}{
		"items": items,
		// "call":  call,
	})
}

func (gg *GodGame) formatVideoInfo2(c frame.Context, hash string) string {
	fileInfo, err := lfspb.Info(c, &lfspb.InfoReq{
		Hash: hash,
	})
	if err == nil && fileInfo.GetErrcode() == 0 {
		return fileInfo.GetData().GetM3U8()
	}
	return ""
}

func (gg *GodGame) formatVideoInfo(c frame.Context, hash string) string {
	fileInfo, err := lfspb.Info(c, &lfspb.InfoReq{
		Hash: hash,
	})
	if err == nil && fileInfo.GetErrcode() == 0 {
		result := map[string]interface{}{
			"id":          0,
			"imageurl":    "",
			"share_url":   "",
			"src":         "",
			"stream_list": []interface{}{},
			"title":       "",
			"type":        "",
			"vid":         hash,
			"key":         fileInfo.GetData().GetKey(),
			"videoInfo": map[string]interface{}{
				"bgCoverUrl": "",
				"coverUrl":   fileInfo.GetData().GetFm(),
				"duration":   fileInfo.GetData().GetDuration(),
				"height":     fileInfo.GetData().GetHeight(),
				"width":      fileInfo.GetData().GetWidth(),
				"size":       fmt.Sprintf("%.2fM", float64(fileInfo.GetData().GetSize())/1048576),
				// "url":        fileInfo.GetData().GetM3U8(),
				"url": fileInfo.GetData().GetMp4(),
			},
		}
		if bs, err := json.Marshal(result); err == nil {
			return fmt.Sprintf("laoyuegou://playvideo?result=%s", bs)
		} else {
			c.Warnf("formatVideoInfo %s error %s", hash, err)
		}
	}
	return ""
}

func (gg *GodGame) GenPeiWanShareURL(godAvatar, godName, gameName, desc string, godID, gameID int64) string {
	var h5URL string
	title := fmt.Sprintf("#%s# %s", gameName, godName)
	subTitle := desc

	if gg.cfg.Env.Production() {
		h5URL = fmt.Sprintf("https://imgx.lygou.cc/tang/dist/pages/god/?user_id=%d&gameid=%d", godID, gameID)
	} else if gg.cfg.Env.QA() {
		h5URL = fmt.Sprintf("https://guest-test-imgx.lygou.cc/tang/dist/pages/god/?user_id=%d&gameid=%d", godID, gameID)
	} else if gg.cfg.Env.Stag() {
		h5URL = fmt.Sprintf("https://guest-staging-imgx.lygou.cc/tang/dist/pages/god/?user_id=%d&gameid=%d", godID, gameID)
	}
	if subTitle == "" {
		subTitle = h5URL
	}
	rawString := fmt.Sprintf("laoyuegou://share?title=%s&&share_url=%s&&share_content=%s&&imageurl=%s&&platform=0&&imageurl_sina=%s&&type=60000001&&god_id=%d&&game_id=%d",
		title, h5URL, subTitle, godAvatar, godAvatar, godID, gameID)
	return rawString
}

func (gg *GodGame) GodDetail(c frame.Context) error {
	var req godgamepb.GodDetailReq
	if err := c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	if gg.isVoiceCallGame(req.GetGameId()) {
		// 语聊品类不展示
		return c.JSON2(StatusOK_V3, "", nil)
	}
	gameStateResp, err := gamepb.Record(c, &gamepb.RecordReq{
		GameId: req.GetGameId(),
	})
	if err == nil && gameStateResp.GetErrcode() == 0 {
		if gameStateResp.GetData().GetState() == game_const.GAME_STATE_NO {
			return c.JSON2(ERR_CODE_DISPLAY_ERROR, "品类已下架", nil)
		}
	} else {
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "品类已下架", nil)
	}
	godInfo := gg.dao.GetGod(req.GetGodId())
	if godInfo.Status != constants.GOD_STATUS_PASSED {
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "大神状态异常", nil)
	}
	v1, err := gg.dao.GetGodSpecialGameV1(req.GetGodId(), req.GetGameId())
	if err != nil {
		return c.JSON2(ERR_CODE_NOT_FOUND, "", nil)
	}
	userinfo, err := gg.getSimpleUser(v1.GodID)
	if err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	resp, err := gamepb.AcceptCfgV2(c, &gamepb.AcceptCfgV2Req{
		GameId: req.GetGameId(),
	})
	if err != nil || resp.GetErrcode() != 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}

	var uniprice int64
	if v1.PriceType == constants.PW_PRICE_TYPE_BY_OM {
		uniprice = v1.PeiWanPrice
	} else {
		uniprice = resp.GetData().GetPrices()[v1.PriceID]
	}
	var regionDesc string
	if len(v1.Regions) > 0 {
		for _, region := range v1.Regions {
			regionDesc = regionDesc + resp.GetData().GetRegionDesc()[region] + ","
		}
		regionDesc = regionDesc[:len(regionDesc)-1]
	}

	var roomID int64
	freeStatus := order_const.PW_STATUS_FREE
	freeResp, err := plorderpb.Free(c, &plorderpb.FreeReq{
		GodId: v1.GodID,
	})
	if err == nil && freeResp.GetErrcode() == 0 {
		freeStatus = freeResp.GetData().GetStatus()
	}
	if req.GetS() != 1 {
		liveResp, err := livepb.GetGodLiveId(c, &livepb.GetGodLiveIdReq{
			GodId: v1.GodID,
		})
		if err == nil && liveResp.GetData() != nil && liveResp.GetData().GetRoomId() > 0 {
			// 优先返回直播
			freeStatus = order_const.PW_STATUS_LIVE
			roomID = liveResp.GetData().GetRoomId()
		} else {
			seatResp, err := pb_chatroom.IsOnSeat(c, &pb_chatroom.IsOnSeatReq{
				UserId: v1.GodID,
			})
			if err == nil && seatResp.GetData() != nil {
				freeStatus = order_const.PW_STATUS_ON_SEAT
				roomID = seatResp.GetData().GetRoomId()
			}
		}

	}
	freeStatusDesc := order_const.PW_STATS_DESC[freeStatus]

	var tmpImages, tmpTags, tmpPowers []string
	var tmpExt interface{}
	json.Unmarshal([]byte(v1.Images), &tmpImages)
	json.Unmarshal([]byte(v1.Tags), &tmpTags)
	json.Unmarshal([]byte(v1.Ext), &tmpExt)
	json.Unmarshal([]byte(v1.Powers), &tmpPowers)
	if len(tmpImages) == 0 {
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "大神形象照加载失败", nil)
	}
	if v1.GameScreenshot != "" {
		tmpImages = append(tmpImages, v1.GameScreenshot)
	}

	appVersion, _ := strconv.Atoi(strings.Replace(gg.getUserAppVersion(c), ".", "", -1))
	if appVersion >= 295 {
		// 2.9.5及以上支持webp
		for idx, _ := range tmpImages {
			tmpImages[idx] = tmpImages[idx] + "/w0"
		}
	}

	data := map[string]interface{}{
		"god_id":             v1.GodID,
		"god_name":           userinfo.GetUsername(),
		"god_avatar":         userinfo.GetAvatarSmall(),
		"sex":                godInfo.Gender,
		"age":                lyg_util.Age(userinfo.GetBirthday()),
		"game_id":            v1.GameID,
		"level":              v1.Level,
		"level_desc":         fmt.Sprintf("大神 Lv%d", v1.Level),
		"highest_level_id":   v1.HighestLevelID,
		"highest_level_desc": resp.GetData().GetLevelDesc()[v1.HighestLevelID],
		"region_accept_desc": regionDesc,
		"god_show_images":    tmpImages,
		"powers":             tmpPowers,
		"voice":              v1.Voice,
		"voice_duration":     v1.VoiceDuration,
		"aac":                v1.Aac,
		// "god_icon":           v1.GodIcon,
		"god_icon":       "", // 解决iOS bug
		"god_tags":       tmpTags,
		"ext":            tmpExt,
		"desc":           v1.Desc,
		"uniprice":       uniprice,
		"gl":             FormatRMB2Gouliang(uniprice),
		"order_cnt":      v1.AcceptNum,
		"order_cnt_desc": FormatAcceptOrderNumber3(v1.AcceptNum),
		"order_rate":     "100%",
		"regions":        v1.Regions,
		"levels":         v1.Levels,
		"score":          v1.Score,
		"score_desc":     FormatScore(v1.Score),
		"status":         freeStatus,
		"status_desc":    freeStatusDesc,
		"room_id":        roomID,
		"shareurl":       gg.GenPeiWanShareURL(userinfo.GetAvatarBig(), userinfo.GetUsername(), gameStateResp.GetData().GetGameName(), v1.Desc, v1.GodID, v1.GameID),
	}
	if v1.Video != "" {
		tmpStr := gg.formatVideoInfo(c, v1.Video)
		data["video"] = tmpStr
		data["videos"] = []string{tmpStr}
	}
	if v1.Videos != "" {
		var tmpVideos []string
		err = json.Unmarshal([]byte(v1.Videos), &tmpVideos)
		if err == nil && len(tmpVideos) > 0 {
			for idx, _ := range tmpVideos {
				tmpVideos[idx] = gg.formatVideoInfo(c, tmpVideos[idx])
			}
			data["videos"] = tmpVideos
		}
	}
	if gg.getUserAppID(c) == "1009" && gg.cfg.Env.Production() {
		// 探索版审核时，不展示视频
		data["video"] = ""
		data["videos"] = []string{}
	}
	if orderPercent, err := plorderpb.OrderFinishPercent(c, &plorderpb.OrderFinishPercentReq{
		GodId: req.GetGodId(),
		Days:  7,
	}); err == nil && orderPercent.GetErrcode() == 0 {
		data["order_rate"] = orderPercent.GetData()
	}

	commentData, _ := plcommentpb.GetGodGameComment(c, &plcommentpb.GetGodGameCommentReq{
		GodId:  req.GetGodId(),
		GameId: req.GetGameId(),
	})
	if commentData != nil && commentData.GetData() != nil {
		data["comments_cnt"] = commentData.GetData().GetCommentCnt()
		data["tags"] = commentData.GetData().GetTags()
	}
	hotComments, _ := plcommentpb.GetHotComments(c, &plcommentpb.GetHotCommentsReq{
		GodId:  req.GetGodId(),
		GameId: req.GetGameId(),
	})
	if hotComments != nil && len(hotComments.GetData()) > 0 {
		data["comments"] = hotComments.GetData()
	}
	// 2.9.7增加是否关注，sub=1：已关注；TODO：调用关注服务，检查当前用户是否关注过大神
	if currentUserID := gg.getCurrentUserID(c); currentUserID > 0 {
		followResp, err := followpb.Relation(c, &followpb.RelationReq{
			CurrentUid: currentUserID,
			TargetUid:  req.GetGodId(),
		})
		if err != nil {
			c.Warnf("%s", err.Error())
		} else if followResp.GetErrcode() != 0 {
			c.Warnf("%s", followResp.GetErrmsg())
		} else if followResp.GetData() == followpb.FOLLOW_STATUS_FOLLOW_STATUS_SINGLE || followResp.GetData() == followpb.FOLLOW_STATUS_FOLLOW_STATUS_BOTH {
			data["sub"] = 1
		}
	}
	return c.JSON2(StatusOK_V3, "", data)
}

// 苹果审核开启期间使用此方法
func (gg *GodGame) queryGodsForAppleAudit(args godgamepb.GodListReq, currentUser model.CurrentUser) ([]model.ESGodGameRedefine, int64) {
	var pwObjs []model.ESGodGameRedefine
	searchService := gg.esClient.Search().Index(gg.cfg.ES.PWIndexRedefine)
	query := elastic.NewBoolQuery().
		Must(elastic.NewRangeQuery("lts").
			Lte(gg.dao.GetHeadline(currentUser.UserID, args.Offset)).
			Gte(time.Now().AddDate(0, 0, -300)))
	query = query.Should(elastic.NewMatchQuery("gender", fmt.Sprintf("%d", constants.GENDER_MALE)).Boost(500))

	if args.Type == constants.SORT_TYPE_DEFAULT {
		if args.GameId > 0 {
			query = query.Must(elastic.NewTermQuery("game_id", args.GameId))
		} else {
			gids := []interface{}{}
			if currentUser.UserID == 0 {
				gids = []interface{}{gamepb.GAME_ID_PUBG, gamepb.GAME_ID_LOL, gamepb.GAME_ID_WZRY}
			} else if len(currentUser.GameIds) > 0 {
				gids = []interface{}{gamepb.GAME_ID_SY, gamepb.GAME_ID_XNLR}
				for _, v := range currentUser.GameIds {
					if gid, ok := gamepb.GameDicst[v]; ok {
						gids = append(gids, gid)
					}
				}
			}
			query = query.Should(elastic.NewTermsQuery("game_id", gids...).Boost(8))
		}
		// 智能排序
		query = query.Should(elastic.NewMatchQuery("peiwan_status", "1").Boost(9))
		query = query.Should(elastic.NewMatchQuery("reject_order", "2").Boost(6))
		query = query.Should(elastic.NewMatchQuery("peiwan_status", "2").Boost(5))
		query = query.Should(elastic.NewMatchQuery("reject_order", "1").Boost(3))
		searchService = searchService.Query(query).
			Sort("_score", false).
			Sort("weight", false).
			Sort("lts", false).
			Sort("seven_days_hours", false)
	} else if args.Type == constants.SORT_TYPE_HOT {
		// 按人气
		if args.GameId > 0 {
			query = query.Must(elastic.NewTermQuery("game_id", args.GameId))
		}
		searchService = searchService.Query(query).Sort("_score", false).Sort("seven_days_hours", false)
	} else if args.Type == constants.SORT_TYPE_NEW {
		// 按新鲜度
		if args.GameId > 0 {
			query = query.Must(elastic.NewTermQuery("game_id", args.GameId))
		}
		searchService = searchService.Query(query).Sort("_score", false).Sort("passedtime", false)
	}

	resp, err := searchService.From(int(args.Offset)).
		Size(int(args.Limit)).
		Pretty(true).
		Do(context.Background())
	if err != nil {
		return pwObjs, 0
	}
	if resp.Hits.TotalHits == 0 {
		return pwObjs, 0
	}
	var pwObj model.ESGodGameRedefine
	for _, item := range resp.Hits.Hits {
		if err = json.Unmarshal(*item.Source, &pwObj); err != nil {
			continue
		}
		pwObjs = append(pwObjs, pwObj)
	}
	return pwObjs, resp.Hits.TotalHits
}

func (gg *GodGame) queryGods2(args godgamepb.GodList2Req, currentUser model.CurrentUser) ([]model.ESGodGameRedefine, int64) {
	var pwObjs []model.ESGodGameRedefine
	searchService := gg.esClient.Search().Index(gg.cfg.ES.PWIndexRedefine)
	query := elastic.NewBoolQuery().
		Must(elastic.NewRangeQuery("lts").
			Lte(gg.dao.GetHeadline(currentUser.UserID, args.Offset)).
			Gte(time.Now().AddDate(0, 0, gg.cfg.GodLTSDuration))).
		Must(elastic.NewTermQuery("game_id", args.GameId))

	if currentUser.UserID == 0 {
		currentUser.Gender = constants.GENDER_MALE
	}
	// 智能排序
	query = query.Should(elastic.NewMatchQuery("peiwan_status", "1").Boost(9))
	if args.Gender == constants.GENDER_UNKNOW {
		if currentUser.Gender == constants.GENDER_FEMALE {
			query = query.Should(elastic.NewMatchQuery("gender", fmt.Sprintf("%d", constants.GENDER_FEMALE)).Boost(4))
			query = query.Should(elastic.NewMatchQuery("gender", fmt.Sprintf("%d", constants.GENDER_MALE)).Boost(7))

		} else {
			query = query.Should(elastic.NewMatchQuery("gender", fmt.Sprintf("%d", constants.GENDER_MALE)).Boost(4))
			query = query.Should(elastic.NewMatchQuery("gender", fmt.Sprintf("%d", constants.GENDER_FEMALE)).Boost(7))
		}
	} else {
		query = query.Must(elastic.NewTermQuery("gender", args.Gender))
	}
	query = query.Should(elastic.NewMatchQuery("reject_order", "2").Boost(6))
	query = query.Should(elastic.NewMatchQuery("peiwan_status", "2").Boost(5))
	query = query.Should(elastic.NewMatchQuery("reject_order", "1").Boost(3))
	// query = query.Should(elastic.NewMatchQuery("video", "1").Boost(10))
	src, _ := query.Source()
	bs, _ := json.Marshal(src)
	icelog.Debugf("## query:%s", string(bs))

	searchService = searchService.Query(query).
		Sort("weight", false).
		Sort("_score", false).
		Sort("lts", false).
		Sort("seven_days_hours", false)
	resp, err := searchService.From(int(args.Offset)).
		Size(int(args.Limit)).
		Pretty(true).
		Do(context.Background())
	if err != nil {
		return pwObjs, 0
	}
	if resp.Hits.TotalHits == 0 {
		return pwObjs, 0
	}
	var pwObj model.ESGodGameRedefine
	for _, item := range resp.Hits.Hits {
		if err = json.Unmarshal(*item.Source, &pwObj); err != nil {
			icelog.Errorf("### %s", err.Error())
			continue
		}
		pwObjs = append(pwObjs, pwObj)
	}
	return pwObjs, resp.Hits.TotalHits
}

func (gg *GodGame) queryGods(args godgamepb.GodListReq, currentUser model.CurrentUser) ([]model.ESGodGameRedefine, int64) {
	var pwObjs []model.ESGodGameRedefine
	var priceCondition, levelCondition []interface{}
	if len(args.Price) > 0 {
		priceCondition = make([]interface{}, 0, len(args.Price))
		for _, p := range args.Price {
			priceCondition = append(priceCondition, p)
		}
	}
	if len(args.Level) > 0 {
		levelCondition = make([]interface{}, 0, len(args.Level))
		for _, l := range args.Level {
			levelCondition = append(levelCondition, l)
		}
	}
	searchService := gg.esClient.Search().Index(gg.cfg.ES.PWIndexRedefine)
	query := elastic.NewBoolQuery().
		Must(elastic.NewRangeQuery("lts").
			Lte(gg.dao.GetHeadline(currentUser.UserID, args.Offset)).
			Gte(time.Now().AddDate(0, 0, gg.cfg.GodLTSDuration)))

	if args.Type == constants.SORT_TYPE_DEFAULT {
		if args.GameId > 0 {
			query = query.Must(elastic.NewTermQuery("game_id", args.GameId))
		} else {
			gids := []interface{}{gamepb.GAME_ID_PUBG, gamepb.GAME_ID_LOL, gamepb.GAME_ID_WZRY}
			query = query.Should(elastic.NewTermsQuery("game_id", gids...).Boost(8))
		}
		if currentUser.UserID == 0 {
			currentUser.Gender = constants.GENDER_MALE
		}
		// 智能排序
		query = query.Should(elastic.NewMatchQuery("peiwan_status", "1").Boost(9))
		if args.Gender == constants.GENDER_UNKNOW {
			if currentUser.Gender == constants.GENDER_FEMALE {
				query = query.Should(elastic.NewMatchQuery("gender", fmt.Sprintf("%d", constants.GENDER_FEMALE)).Boost(4))
				query = query.Should(elastic.NewMatchQuery("gender", fmt.Sprintf("%d", constants.GENDER_MALE)).Boost(7))

			} else {
				query = query.Should(elastic.NewMatchQuery("gender", fmt.Sprintf("%d", constants.GENDER_MALE)).Boost(4))
				query = query.Should(elastic.NewMatchQuery("gender", fmt.Sprintf("%d", constants.GENDER_FEMALE)).Boost(7))
			}
		} else {
			query = query.Must(elastic.NewTermQuery("gender", args.Gender))
		}
		if len(levelCondition) > 0 {
			query = query.Must(elastic.NewTermsQuery("highest_level_id", levelCondition...))
		}
		if len(priceCondition) > 0 {
			query = query.Must(elastic.NewTermsQuery("price_id", priceCondition...))
		}
		query = query.Should(elastic.NewMatchQuery("reject_order", "2").Boost(6))
		query = query.Should(elastic.NewMatchQuery("peiwan_status", "2").Boost(5))
		query = query.Should(elastic.NewMatchQuery("reject_order", "1").Boost(3))
		searchService = searchService.Query(query).
			Sort("weight", false).
			Sort("_score", false).
			Sort("lts", false).
			Sort("seven_days_hours", false)
	} else if args.Type == constants.SORT_TYPE_HOT {
		// 按人气
		if args.GameId > 0 {
			query = query.Must(elastic.NewTermQuery("game_id", args.GameId))
		}
		if args.Gender != constants.GENDER_UNKNOW {
			query = query.Must(elastic.NewTermQuery("gender", args.Gender))
		}
		if len(levelCondition) > 0 {
			query = query.Must(elastic.NewTermsQuery("highest_level_id", levelCondition...))
		}
		if len(priceCondition) > 0 {
			query = query.Must(elastic.NewTermsQuery("price_id", priceCondition...))
		}
		searchService = searchService.Query(query).Sort("_score", false).Sort("seven_days_hours", false)
	} else if args.Type == constants.SORT_TYPE_NEW {
		// 按新鲜度
		if args.GameId > 0 {
			query = query.Must(elastic.NewTermQuery("game_id", args.GameId))
		}
		if args.Gender != constants.GENDER_UNKNOW {
			query = query.Must(elastic.NewTermQuery("gender", args.Gender))
		}
		if len(levelCondition) > 0 {
			query = query.Must(elastic.NewTermsQuery("highest_level_id", levelCondition...))
		}
		if len(priceCondition) > 0 {
			query = query.Must(elastic.NewTermsQuery("price_id", priceCondition...))
		}
		searchService = searchService.Query(query).Sort("_score", false).Sort("passedtime", false)
	}
	// src, _ := query.Source()
	// bs, _ := json.Marshal(src)
	// icelog.Infof("### %d query:%s, gameid:%d, type:%d", currentUser.UserID, string(bs), args.GameId, args.Type)

	resp, err := searchService.From(int(args.Offset)).
		Size(int(args.Limit)).
		Pretty(true).
		Do(context.Background())
	if err != nil {
		return pwObjs, 0
	}
	if resp.Hits.TotalHits == 0 {
		return pwObjs, 0
	}
	var pwObj model.ESGodGameRedefine
	for _, item := range resp.Hits.Hits {
		if err = json.Unmarshal(*item.Source, &pwObj); err != nil {
			icelog.Errorf("### %s", err.Error())
			continue
		}
		pwObjs = append(pwObjs, pwObj)
	}
	return pwObjs, resp.Hits.TotalHits
}

func (gg *GodGame) queryRecommendGods(args godgamepb.GodListReq, currentUser model.CurrentUser) []model.ESGodGameRedefine {
	args.Limit = 40
	args.Offset = 0
	var pwObjs []model.ESGodGameRedefine
	var priceCondition []interface{}
	if len(args.Price) > 0 {
		priceCondition = make([]interface{}, 0, len(args.Price))
		for _, p := range args.Price {
			priceCondition = append(priceCondition, p)
		}
	}
	if args.Gender == constants.GENDER_UNKNOW && (len(args.Level) == 0 || len(args.Price) == 0) {
		priceCondition = make([]interface{}, 0)
	}

	searchService := gg.esClient.Search().Index(gg.cfg.ES.PWIndexRedefine)
	query := elastic.NewBoolQuery().
		Must(elastic.NewRangeQuery("lts").
			Lte(gg.dao.GetHeadline(currentUser.UserID, args.Offset)).
			Gte(time.Now().AddDate(0, 0, gg.cfg.GodLTSDuration)))

	if args.Type == constants.SORT_TYPE_DEFAULT {
		if args.GameId > 0 {
			query = query.Must(elastic.NewTermQuery("game_id", args.GameId))
		} else {
			gids := []interface{}{gamepb.GAME_ID_SY, gamepb.GAME_ID_SJ, gamepb.GAME_ID_XNLR, gamepb.GAME_ID_DWRG}
			query = query.Should(elastic.NewTermsQuery("game_id", gids...).Boost(8))
		}
		if currentUser.UserID == 0 {
			currentUser.Gender = constants.GENDER_MALE
		}
		// 智能排序
		query = query.Should(elastic.NewMatchQuery("peiwan_status", "1").Boost(9))
		if args.Gender == constants.GENDER_UNKNOW {
			if currentUser.Gender == constants.GENDER_FEMALE {
				query = query.Should(elastic.NewMatchQuery("gender", fmt.Sprintf("%d", constants.GENDER_FEMALE)).Boost(4))
				query = query.Should(elastic.NewMatchQuery("gender", fmt.Sprintf("%d", constants.GENDER_MALE)).Boost(7))

			} else {
				query = query.Should(elastic.NewMatchQuery("gender", fmt.Sprintf("%d", constants.GENDER_MALE)).Boost(4))
				query = query.Should(elastic.NewMatchQuery("gender", fmt.Sprintf("%d", constants.GENDER_FEMALE)).Boost(7))
			}
		} else {
			query = query.Must(elastic.NewTermQuery("gender", args.Gender))
		}
		if len(priceCondition) > 0 {
			query = query.Must(elastic.NewTermsQuery("price_id", priceCondition...))
		}
		query = query.Should(elastic.NewMatchQuery("reject_order", "2").Boost(6))
		query = query.Should(elastic.NewMatchQuery("peiwan_status", "2").Boost(5))
		query = query.Should(elastic.NewMatchQuery("reject_order", "1").Boost(3))
		searchService = searchService.Query(query).
			Sort("weight", false).
			Sort("_score", false).
			Sort("lts", false).
			Sort("seven_days_hours", false)
	} else if args.Type == constants.SORT_TYPE_HOT {
		// 按人气
		if args.GameId > 0 {
			query = query.Must(elastic.NewTermQuery("game_id", args.GameId))
		}
		if args.Gender != constants.GENDER_UNKNOW {
			query = query.Must(elastic.NewTermQuery("gender", args.Gender))
		}
		if len(priceCondition) > 0 {
			query = query.Must(elastic.NewTermsQuery("price_id", priceCondition...))
		}
		searchService = searchService.Query(query).Sort("_score", false).Sort("seven_days_hours", false)
	} else if args.Type == constants.SORT_TYPE_NEW {
		// 按新鲜度
		if args.GameId > 0 {
			query = query.Must(elastic.NewTermQuery("game_id", args.GameId))
		}
		if args.Gender != constants.GENDER_UNKNOW {
			query = query.Must(elastic.NewTermQuery("gender", args.Gender))
		}
		if len(priceCondition) > 0 {
			query = query.Must(elastic.NewTermsQuery("price_id", priceCondition...))
		}
		searchService = searchService.Query(query).Sort("_score", false).Sort("passedtime", false)
	}

	resp, err := searchService.From(int(args.Offset)).
		Size(int(args.Limit)).
		Pretty(true).
		Do(context.Background())
	if err != nil {
		return pwObjs
	}
	if resp.Hits.TotalHits == 0 {
		return pwObjs
	}
	var pwObj model.ESGodGameRedefine
	for _, item := range resp.Hits.Hits {
		if err = json.Unmarshal(*item.Source, &pwObj); err != nil {
			icelog.Warnf("query god error %s\n%s", err, *item.Source)
			continue
		}
		pwObjs = append(pwObjs, pwObj)
	}
	return pwObjs
}

func (gg *GodGame) getGodItems(pwObjs []model.ESGodGameRedefine) []map[string]interface{} {
	gods := make([]map[string]interface{}, 0, 10)
	var tmpImages []string
	var userinfo *user_pb.UserInfo
	var err error
	var god model.GodGameV1
	var resp *gamepb.AcceptCfgV2Resp
	var uniprice int64
	var roomID int64
	invalidItems := make([]string, 0, 4)
	ctx := frame.TODO()
	for _, pwObj := range pwObjs {
		userinfo, err = gg.getSimpleUser(pwObj.GodID)
		if err != nil || userinfo == nil || userinfo.GetInvalid() != user_pb.USER_INVALID_NO {
			invalidItems = append(invalidItems, fmt.Sprintf("%d-%d", pwObj.GodID, pwObj.GameID))
			continue
		}
		god, err = gg.dao.GetGodSpecialGameV1(pwObj.GodID, pwObj.GameID)
		if err != nil {
			invalidItems = append(invalidItems, fmt.Sprintf("%d-%d", pwObj.GodID, pwObj.GameID))
			continue
		}
		if god.GrabSwitch != constants.GRAB_SWITCH_OPEN {
			// 关闭接单开关不显示
			invalidItems = append(invalidItems, fmt.Sprintf("%d-%d", pwObj.GodID, pwObj.GameID))
			continue
		}
		tmpImages = make([]string, 0, 6)
		json.Unmarshal([]byte(god.Images), &tmpImages)
		if len(tmpImages) == 0 {
			continue
		}
		resp, err = gamepb.AcceptCfgV2(ctx, &gamepb.AcceptCfgV2Req{
			GameId: pwObj.GameID,
		})
		if err != nil || resp.GetErrcode() != 0 {
			continue
		}
		if god.PriceType == constants.PW_PRICE_TYPE_BY_OM {
			uniprice = god.PeiWanPrice
		} else {
			uniprice = resp.GetData().GetPrices()[god.PriceID]
		}
		liveResp, err := livepb.GetGodLiveId(ctx, &livepb.GetGodLiveIdReq{
			GodId:  pwObj.GodID,
			GameId: pwObj.GameID,
		})
		if err == nil && liveResp.GetData() != nil && liveResp.GetData().GetRoomId() > 0 {
			// 优先返回直播
			pwObj.PeiWanStatus = order_const.PW_STATUS_LIVE
			roomID = liveResp.GetData().GetRoomId()
		} else {
			roomID = 0
			pwObj.PeiWanStatus = order_const.PW_STATUS_FREE
		}

		gods = append(gods, map[string]interface{}{
			"god_id":         pwObj.GodID,
			"god_name":       userinfo.GetUsername(),
			"sex":            pwObj.Gender,
			"age":            lyg_util.Age(userinfo.GetBirthday()),
			"game_id":        god.GameID,
			"status":         pwObj.PeiWanStatus,
			"status_desc":    order_const.PW_STATS_DESC[pwObj.PeiWanStatus],
			"voice":          god.Voice,
			"voice_duration": god.VoiceDuration,
			"aac":            god.Aac,
			"imgs":           []string{fmt.Sprintf("%s/w400", tmpImages[0])},
			"god_icon":       god.GodIcon,
			"uniprice":       uniprice,
			"gl":             FormatRMB2Gouliang(uniprice),
			"order_cnt":      god.AcceptNum,
			"order_cnt_desc": FormatAcceptOrderNumber(god.AcceptNum),
			"room_id":        roomID,
		})
	}
	if len(invalidItems) > 0 {
		gg.ESBatchDeleteByID(invalidItems)
	}
	return gods
}

func (gg *GodGame) getGodItems2(c frame.Context, pwObjs []model.ESGodGameRedefine) []map[string]interface{} {
	gods := make([]map[string]interface{}, 0, 10)
	var tmpGod map[string]interface{}
	var err error
	for _, pwObj := range pwObjs {
		if tmpGod, err = gg.buildGodDetail(c, pwObj.GodID, pwObj.GameID); err == nil {
			gods = append(gods, tmpGod)
		}
	}
	return gods
}

// 指定游戏的陪玩大神列表
func (gg *GodGame) GodList(c frame.Context) error {
	var req godgamepb.GodListReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	} else if req.GetLimit() > 20 || req.GetLimit() == 0 {
		req.Limit = 20
	}
	if req.GetGameId() > 0 {
		if gg.isVoiceCallGame(req.GetGameId()) {
			// 语聊品类不展示
			return c.JSON2(StatusOK_V3, "", nil)
		}
		if gid, ok := gamepb.GameDicst[req.GetGameId()]; ok {
			req.GameId = gid
		}
	}
	currentUser := gg.getCurrentUser(c)
	var pwObjs []model.ESGodGameRedefine
	var hits int64
	var gods, recGods []map[string]interface{}
	pwObjs, hits = gg.queryGods(req, currentUser)
	gods = gg.getGodItems(pwObjs)
	if hits < req.Limit && (req.Gender > 0 || len(req.Price) > 0 || len(req.Level) > 0) {
		recGods = gg.getGodItems(gg.queryRecommendGods(req, currentUser))
	}
	return c.JSON2(StatusOK_V3, "", map[string]interface{}{
		"gods": gods,
		"rec":  recGods,
	})
}

func (gg *GodGame) GodListInternal(c frame.Context) error {
	var req godgamepb.GodListReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	} else if req.GetLimit() > 20 || req.GetLimit() == 0 {
		req.Limit = 20
	}
	if req.GetGameId() > 0 {
		if gg.isVoiceCallGame(req.GetGameId()) {
			// 语聊品类不展示
			return c.JSON2(StatusOK_V3, "", nil)
		}
		if gid, ok := gamepb.GameDicst[req.GetGameId()]; ok {
			req.GameId = gid
		}
	}
	currentUser := gg.getCurrentUser(c)
	var pwObjs []model.ESGodGameRedefine
	var hits int64
	var gods, recGods []map[string]interface{}
	pwObjs, hits = gg.queryGods(req, currentUser)
	gods = gg.getGodItems(pwObjs)
	if hits < req.Limit && (req.Gender > 0 || len(req.Price) > 0 || len(req.Level) > 0) {
		recGods = gg.getGodItems(gg.queryRecommendGods(req, currentUser))
	}
	return c.JSON2(StatusOK_V3, "", map[string]interface{}{
		"gods": gods,
		"rec":  recGods,
	})
}

func (gg *GodGame) GodList2(c frame.Context) error {
	var req godgamepb.GodList2Req
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	} else if req.GetGameId() == 0 {
		return c.JSON2(StatusOK_V3, "", nil)
	} else if req.GetLimit() > 20 || req.GetLimit() == 0 {
		req.Limit = 20
	}
	currentUser := gg.getCurrentUser(c)
	gender := req.GetGender()
	if gender != constants.GENDER_FEMALE && gender != constants.GENDER_MALE {
		if currentUser.Gender == constants.GENDER_FEMALE {
			gender = constants.GENDER_MALE
		} else {
			gender = constants.GENDER_FEMALE
		}
	}
	godInfos, totalCnt := gg.dao.GetGodListsByGender(req.GetGameId(), gender, req.GetOffset(), req.GetLimit(), c)
	if totalCnt > 0 {
		return c.RetSuccess("success", map[string]interface{}{
			"total": totalCnt,
			"gods":  godInfos,
		})
	}

	if req.GetOffset() == 0 {
		result, hits := gg.dao.GetGodListCache(req.GetGameId(), gender)
		if len(result) > 0 {
			return c.RetSuccess("", map[string]interface{}{
				"gods":  result,
				"total": hits,
			})
		}
	}
	var pwObjs []model.ESGodGameRedefine
	var hits int64
	var gods []map[string]interface{}
	pwObjs, hits = gg.queryGods2(req, currentUser)
	gods = gg.getGodItems2(c, pwObjs)
	if req.GetOffset() == 0 {
		gg.dao.SaveGodListCache(req.GetGameId(), gender, hits, gods)
	}
	return c.JSON2(StatusOK_V3, "", map[string]interface{}{
		"gods":  gods,
		"total": hits,
	})
}

// Ta的陪玩 V2
func (gg *GodGame) GodGamesV2(c frame.Context) error {
	godID := c.GetInt64("god_id", 0)
	if godID == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "invalid god_id", nil)
	}
	god := gg.dao.GetGod(godID)
	if god.Status != constants.GOD_STATUS_PASSED {
		return c.JSON2(StatusOK_V3, "", nil)
	}
	v1s, err := gg.dao.GetGodAllGameV1(godID)
	if err != nil {
		c.Warnf("GodGames error:%s", err)
		return c.JSON2(ERR_CODE_INTERNAL, "", nil)
	}
	if len(v1s) == 0 {
		return c.JSON2(StatusOK_V3, "", nil)
	}
	var v1 model.GodGameV1
	for _, tmpV1 := range v1s {
		if tmpV1.GrabSwitch == constants.GRAB_SWITCH_OPEN {
			v1 = tmpV1
			break
		}
	}
	if v1.GrabSwitch != constants.GRAB_SWITCH_OPEN {
		return c.JSON2(StatusOK_V3, "", nil)
	}
	var uniprice int64
	tmpData := make(map[string]interface{})
	if v1.PriceType == constants.PW_PRICE_TYPE_BY_OM {
		uniprice = v1.PeiWanPrice
	} else {
		cfgResp, err := gamepb.AcceptCfgV2(frame.TODO(), &gamepb.AcceptCfgV2Req{
			GameId: v1.GameID,
		})
		if err != nil || cfgResp.GetErrcode() != 0 {
			return c.JSON2(StatusOK_V3, "", nil)
		}
		uniprice = cfgResp.GetData().GetPrices()[v1.PriceID]
	}
	tmpData["pw_price"] = FormatPriceV1(uniprice)
	tmpData["gl"] = FormatRMB2Gouliang(uniprice)
	tmpData["game_id"] = v1.GameID
	tmpData["score"] = FormatScore(v1.Score)
	acceptResp, err := gamepb.Accept(c, &gamepb.AcceptReq{
		GameId:   v1.GameID,
		AcceptId: v1.HighestLevelID,
	})
	if err == nil && acceptResp.GetErrcode() == 0 {
		tmpData["highest_level_desc"] = acceptResp.GetData().GetName()
		tmpData["title"] = acceptResp.GetData().GetGameName()
	} else {
		return c.JSON2(StatusOK_V3, "", nil)
	}
	tmpData["accept_num"] = FormatAcceptOrderNumber(v1.AcceptNum)
	tmpData["status"] = constants.GOD_GAME_STATUS_PASSED
	return c.JSON2(StatusOK_V3, "", tmpData)
}

// Ta的陪玩 V3
func (gg *GodGame) GodGamesV3(c frame.Context) error {
	godID := c.GetInt64("god_id", 0)
	if godID == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "invalid god_id", nil)
	}
	god := gg.dao.GetGod(godID)
	if god.Status != constants.GOD_STATUS_PASSED {
		return c.JSON2(StatusOK_V3, "", nil)
	}
	v1s, err := gg.dao.GetGodAllGameV1(godID)
	if err != nil {
		c.Warnf("GodGames error:%s", err)
		return c.JSON2(ERR_CODE_INTERNAL, "", nil)
	}
	sort.Sort(v1s)
	data := make([]map[string]interface{}, 0, len(v1s))
	var uniprice int64
	for _, v1 := range v1s {
		if v1.GrabSwitch != constants.GRAB_SWITCH_OPEN {
			continue
		}
		if gg.isVoiceCallGame(v1.GameID) {
			// 不返回语聊品类
			continue
		}
		tmpData := make(map[string]interface{})
		if v1.PriceType == constants.PW_PRICE_TYPE_BY_OM {
			uniprice = v1.PeiWanPrice
		} else {
			cfgResp, err := gamepb.AcceptCfgV2(frame.TODO(), &gamepb.AcceptCfgV2Req{
				GameId: v1.GameID,
			})
			if err != nil || cfgResp.GetErrcode() != 0 {
				continue
			}
			uniprice = cfgResp.GetData().GetPrices()[v1.PriceID]
		}
		tmpData["pw_price"] = FormatPriceV1(uniprice)
		tmpData["gl"] = FormatRMB2Gouliang(uniprice)
		tmpData["game_id"] = v1.GameID
		tmpData["score"] = FormatScore(v1.Score)
		acceptResp, err := gamepb.Accept(c, &gamepb.AcceptReq{
			GameId:   v1.GameID,
			AcceptId: v1.HighestLevelID,
		})
		if err == nil && acceptResp.GetErrcode() == 0 {
			tmpData["highest_level_desc"] = acceptResp.GetData().GetName()
			tmpData["game_name"] = acceptResp.GetData().GetGameName()
		} else {
			icelog.Errorf("### Accept error:%s, %d", err, acceptResp.GetErrcode())
			continue
		}
		tmpData["accept_num"] = FormatAcceptOrderNumber(v1.AcceptNum)
		tmpData["status"] = constants.GOD_GAME_STATUS_PASSED
		data = append(data, tmpData)
	}
	return c.JSON2(StatusOK_V3, "", data)
}

// Ta的陪玩 V4
func (gg *GodGame) GodGamesV4(c frame.Context) error {
	godID := c.GetInt64("god_id", 0)
	if godID == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "invalid god_id", nil)
	}
	god := gg.dao.GetGod(godID)
	if god.Status != constants.GOD_STATUS_PASSED {
		return c.JSON2(StatusOK_V3, "", nil)
	}
	if god.Desc != "" {
		if t := c.GetInt64("t", 0); t == 0 || t < gg.dao.GetGodLastModifyDescTimestamp(godID) {
			// 发送大神自定义介绍给用户
			if currentUserID := gg.getCurrentUserID(c); currentUserID > 0 {
				go imapipb.SendMessage(c, &imapipb.SendMessageReq{
					Thread:      lyg_util.CreatePrivateMessageThread(godID, currentUserID).String(),
					FromId:      godID,
					ToId:        currentUserID,
					ContentType: imapipb.MESSAGE_CONTENT_TYPE_TEXT,
					Subtype:     imapipb.MESSAGE_SUBTYPE_CHAT,
					Message:     god.Desc,
					Apn:         "",
					Ttl:         1,
					Fanout:      []int64{},
				})
			}
		}
	}
	v1s, err := gg.dao.GetGodAllGameV1(godID)
	if err != nil {
		c.Warnf("GodGames error:%s", err)
		return c.JSON2(StatusOK_V3, "", nil)
	}
	sort.Sort(v1s)
	data := make([]map[string]interface{}, 0, len(v1s))
	var uniprice int64
	for _, v1 := range v1s {
		if v1.GrabSwitch != constants.GRAB_SWITCH_OPEN {
			continue
		}
		if gg.isVoiceCallGame(v1.GameID) {
			// 不返回语聊品类
			continue
		}
		tmpData := make(map[string]interface{})
		if v1.PriceType == constants.PW_PRICE_TYPE_BY_OM {
			uniprice = v1.PeiWanPrice
		} else {
			cfgResp, err := gamepb.AcceptCfgV2(frame.TODO(), &gamepb.AcceptCfgV2Req{
				GameId: v1.GameID,
			})
			if err != nil || cfgResp.GetErrcode() != 0 {
				continue
			}
			uniprice = cfgResp.GetData().GetPrices()[v1.PriceID]
		}
		tmpData["pw_price"] = FormatPriceV1(uniprice)
		tmpData["gl"] = FormatRMB2Gouliang(uniprice)
		tmpData["game_id"] = v1.GameID
		tmpData["score"] = FormatScore(v1.Score)
		acceptResp, err := gamepb.Accept(c, &gamepb.AcceptReq{
			GameId:   v1.GameID,
			AcceptId: v1.HighestLevelID,
		})
		if err == nil && acceptResp.GetErrcode() == 0 {
			tmpData["highest_level_desc"] = acceptResp.GetData().GetName()
			tmpData["game_name"] = acceptResp.GetData().GetGameName()
		} else {
			icelog.Errorf("### Accept error:%s, %d", err, acceptResp.GetErrcode())
			continue
		}
		tmpData["accept_num"] = FormatAcceptOrderNumber(v1.AcceptNum)
		tmpData["status"] = constants.GOD_GAME_STATUS_PASSED
		data = append(data, tmpData)
	}
	return c.JSON2(StatusOK_V3, "", data)
}

// 获取老的陪玩申请数据
func (gg *GodGame) OldData(c frame.Context) error {
	gameID := c.GetInt64("game_id", 0)
	if gameID == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	currentUserID := gg.getCurrentUserID(c)
	// TODO: 上线打开
	// if !gg.dao.CheckGodCanModifyGameInfo(currentUserID, gameID) {
	// 	return c.JSON2(ERR_CODE_DISPLAY_ERROR, "每周只可申请一次修改资料，请下周再申请", nil)
	// }
	data, err := gg.dao.GetOldData(currentUserID, gameID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.JSON2(StatusOK_V3, "", nil)
		}
		c.Warnf("%s", err.Error())
		return c.JSON2(ERR_CODE_INTERNAL, "", nil)
	}
	var tmpImages, tmpTags, tmpExt, tmpPowers interface{}
	json.Unmarshal([]byte(data.Images), &tmpImages)
	json.Unmarshal([]byte(data.Tags), &tmpTags)
	json.Unmarshal([]byte(data.Ext), &tmpExt)
	json.Unmarshal([]byte(data.Powers), &tmpPowers)
	ret := map[string]interface{}{
		"god_id":           data.UserID,
		"game_id":          data.GameID,
		"region_id":        data.RegionID,
		"highest_level_id": data.HighestLevelID,
		"game_screenshot":  data.GameScreenshot,
		"god_imgs":         tmpImages,
		"powers":           tmpPowers,
		"voice":            data.Voice,
		"voice_duration":   data.VoiceDuration,
		"aac":              data.Aac,
		"tags":             tmpTags,
		"ext":              tmpExt,
		"desc":             data.Desc,
	}
	if data.Video != "" {
		tmpStr := gg.formatVideoInfo(c, data.Video)
		ret["video"] = tmpStr
		ret["videos"] = []string{tmpStr}
	}
	if data.Videos != "" {
		var tmpVideos []string
		err = json.Unmarshal([]byte(data.Videos), &tmpVideos)
		if err == nil && len(tmpVideos) > 0 {
			for idx, _ := range tmpVideos {
				tmpVideos[idx] = gg.formatVideoInfo(c, tmpVideos[idx])
			}
			ret["videos"] = tmpVideos
		}
	}
	godApply := gg.dao.GetGodApply(currentUserID)
	if godApply.ID > 0 {
		ret["realname"] = godApply.RealName
		ret["idcard"] = godApply.IDcard
		ret["idcardurl"] = godApply.IDcardurl
		ret["idcardurl2"] = GenIDCardURL(godApply.IDcardurl, gg.cfg.OSS.OSSAccessID, gg.cfg.OSS.OSSAccessKey)
		ret["idcardtype"] = godApply.IDcardtype
		ret["phone"] = godApply.Phone
	}
	return c.JSON2(StatusOK_V3, "", ret)
}

// 格式化大神品类资料状态，只针对已通过或被冻结的品类进行格式化
// 用于适配：修改资料审核中(8)、修改资料被拒绝(9)
func transGodGameApplyStatus(godGameStatus, godGameApplyStatus int64) int64 {
	if godGameApplyStatus == constants.GOD_GAME_APPLY_STATUS_PENDING {
		return int64(8)
	} else if godGameApplyStatus == constants.GOD_GAME_APPLY_STATUS_REFUSED {
		return int64(9)
	}
	return godGameStatus
}

// 我是大神
func (gg *GodGame) MyGod(c frame.Context) error {
	currentUserID := gg.getCurrentUserID(c)
	if currentUserID == 0 {
		return c.JSON2(ERR_CODE_FORBIDDEN, "", nil)
	}
	godInfo := gg.dao.GetGod(currentUserID)
	if godInfo.ID == 0 {
		return c.JSON2(ERR_CODE_FORBIDDEN, "", nil)
	}
	data := make(map[string]interface{})
	live := make(map[string]interface{})
	live["status"] = 2
	live["url"] = "http://www.laoyuegou.com/home"
	liveResp, err := livepb.LiveOwnerInfo(c, &livepb.LiveOwnerInfoReq{
		Userid: currentUserID,
	})
	if err == nil && liveResp.GetErrcode() == 0 {
		if liveResp.GetData().GetIsOwner() {
			live["status"] = 1
			live["mags"] = fmt.Sprintf("%d/%d人", liveResp.GetData().GetCurManager(), liveResp.GetData().GetTotalManager())
			live["mutes"] = fmt.Sprintf("%d/%d人", liveResp.GetData().GetCurForbider(), liveResp.GetData().GetTotalForbider())
			live["room_id"] = liveResp.GetData().GetRoomId()
		}
	}
	data["live"] = live
	orderResp, err := plorderpb.Count(frame.TODO(), &plorderpb.CountReq{
		GodId: currentUserID,
	})
	if err == nil && orderResp.GetData() != nil {
		data["accept_num"] = orderResp.GetData().GetCompletedHoursAmount()
		data["accept_num_desc"] = FormatAcceptOrderNumber(orderResp.GetData().GetCompletedHoursAmount())
	}
	commentCountResp, err := plcommentpb.GetGodCommentCount(frame.TODO(), &plcommentpb.GetGodCommentCountReq{
		GodId: currentUserID,
	})
	if err == nil && commentCountResp.GetData() != nil {
		data["comment_num"] = commentCountResp.GetData().GetTotalCount()
	}

	incomeResp, err := plorderpb.Income(frame.TODO(), &plorderpb.IncomeReq{
		GodId: currentUserID,
	})
	if err == nil && incomeResp.GetData() != nil {
		data["income"] = map[string]interface{}{
			"user_id":      currentUserID,
			"month_income": incomeResp.GetData().GetMouth(),
			"total_income": incomeResp.GetData().GetTotal(),
		}
	}
	data["status"] = godInfo.Status
	data["desc"] = godInfo.Desc
	data["level_jump"] = gg.cfg.Urls["level_jump"]
	if appID := gg.getUserAppID(c); appID == "1006" {
		// TODO：给银兔版本临时增加的等级帮助地址
		data["level_jump"] = gg.cfg.Urls["level_jump_tu"]
	}
	data["god_help_jump"] = gg.cfg.Urls["god_help_jump"]
	data["tip"] = gg.cfg.Urls["tip"]
	godGames, err := gg.dao.GetGodAllGameV1(currentUserID)
	if err != nil {
		return c.JSON2(ERR_CODE_FORBIDDEN, "", nil)
	}
	blockedGodGames, err := gg.dao.GetGodBlockedGameV1(currentUserID)
	if err == nil && len(blockedGodGames) > 0 {
		godGames = append(godGames, blockedGodGames...)
	}
	settings := make([]map[string]interface{}, 0, len(godGames))
	var resp *gamepb.AcceptCfgV2Resp
	for _, godGame := range godGames {
		resp, err = gamepb.AcceptCfgV2(frame.TODO(), &gamepb.AcceptCfgV2Req{
			GameId: godGame.GameID,
		})
		if err != nil || resp == nil {
			continue
		}
		var tmpImgs, tmpTags, tmpExt, tmpVideos interface{}
		json.Unmarshal([]byte(godGame.Images), &tmpImgs)
		json.Unmarshal([]byte(godGame.Tags), &tmpTags)
		json.Unmarshal([]byte(godGame.Ext), &tmpExt)
		json.Unmarshal([]byte(godGame.Videos), &tmpVideos)
		settings = append(settings, map[string]interface{}{
			"game_id":    godGame.GameID,
			"god_id":     godGame.GodID,
			"level":      godGame.Level,
			"level_desc": fmt.Sprintf("大神 Lv%d", godGame.Level),
			"accept_settings": map[string]interface{}{
				"region_id": godGame.Regions,
				"level_id":  godGame.Levels,
			},
			"unit_price_id":       godGame.PriceID,
			"uniprice":            resp.GetData().GetPrices()[godGame.PriceID],
			"status":              transGodGameApplyStatus(godGame.Status, gg.dao.GetGodGameApplyStatus(godGame.GodID, godGame.GameID)),
			"highest_level_score": resp.GetData().GetLevels()[godGame.HighestLevelID],
			"highest_level_id":    godGame.HighestLevelID,
			"voice":               godGame.Voice,
			"voice_duration":      godGame.VoiceDuration,
			"aac":                 godGame.Aac,
			"video":               godGame.Video,
			"videos":              tmpVideos,
			"desc":                godGame.Desc,
			"god_imgs":            tmpImgs,
			"tags":                tmpTags,
			"ext":                 tmpExt,
			"grab_switch":         godGame.GrabSwitch,
			"grab_switch2":        godGame.GrabSwitch2,
			"grab_switch3":        godGame.GrabSwitch3,
			"grab_switch4":        godGame.GrabSwitch4,
			"order_cnt":           godGame.AcceptNum,
			"order_cnt_desc":      FormatAcceptOrderNumber(godGame.AcceptNum),
		})
	}
	data["order_settings"] = settings
	return c.JSON2(StatusOK_V3, "", data)
}

// 急速接单设置
func (gg *GodGame) AcceptQuickOrderSetting(c frame.Context) error {

	currentUser := gg.getCurrentUser(c)
	if currentUser.UserID == 0 {
		return c.JSON2(ERR_CODE_FORBIDDEN, "", nil)
	}
	godInfo := gg.dao.GetGod(currentUser.UserID)
	if godInfo.Status != constants.GOD_STATUS_PASSED {
		return c.JSON2(ERR_CODE_FORBIDDEN, "大神状态异常", nil)
	}
	var req godgamepb.AcceptOrderSettingReq
	var err error

	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	} else if req.GetGameId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "game_id is empty", nil)
	}
	godGame, err := gg.dao.GetGodSpecialGameV1(currentUser.UserID, req.GetGameId())
	if err != nil {
		blockedGodGame, _ := gg.dao.GetGodSpecialBlockedGameV1(currentUser.UserID, req.GetGameId())
		if blockedGodGame.Status == constants.GOD_GAME_STATUS_BLOCKED {
			return c.JSON2(ERR_CODE_FORBIDDEN, "陪玩服务被冻结，暂时无法接单", nil)
		}
		return c.JSON2(ERR_CODE_FORBIDDEN, "", nil)
	}
	icelog.Info(godGame)
	return c.JSON2(StatusOK_V3, "success", nil)
}

// 接单设置
func (gg *GodGame) AcceptOrderSetting(c frame.Context) error {
	currentUser := gg.getCurrentUser(c)
	if currentUser.UserID == 0 {
		return c.JSON2(ERR_CODE_FORBIDDEN, "", nil)
	}
	godInfo := gg.dao.GetGod(currentUser.UserID)
	if godInfo.Status != constants.GOD_STATUS_PASSED {
		return c.JSON2(ERR_CODE_FORBIDDEN, "大神状态异常", nil)
	}

	var req godgamepb.AcceptOrderSettingReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	} else if req.GetGameId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "game_id is empty", nil)
	} else if req.GetGrabSwitch() != constants.GRAB_SWITCH_OPEN {
		req.GrabSwitch = constants.GRAB_SWITCH_CLOSE
	}
	godGame, err := gg.dao.GetGodSpecialGameV1(currentUser.UserID, req.GetGameId())
	if err != nil {
		blockedGodGame, _ := gg.dao.GetGodSpecialBlockedGameV1(currentUser.UserID, req.GetGameId())
		if blockedGodGame.Status == constants.GOD_GAME_STATUS_BLOCKED {
			return c.JSON2(ERR_CODE_FORBIDDEN, "陪玩服务被冻结，暂时无法接单", nil)
		}
		return c.JSON2(ERR_CODE_FORBIDDEN, "", nil)
	}
	resp, err := gamepb.AcceptCfgV2(frame.TODO(), &gamepb.AcceptCfgV2Req{
		GameId: req.GetGameId(),
	})
	if err != nil || resp == nil {
		return c.JSON2(ERR_CODE_INTERNAL, "", nil)
	}
	highestLevelScore := resp.GetData().GetLevels()[godGame.HighestLevelID]
	if req.GetGrabSwitch() == constants.GRAB_SWITCH_OPEN {
		if godGame.PriceID == 0 && len(godGame.Regions) == 0 && len(godGame.Levels) == 0 {
			return c.JSON2(ERR_CODE_EMPTY_ACCEPT_SETTING, errEmptyAcceptSettingMsg, nil)
		} else if req.GetUnitPriceId() == 0 {
			return c.JSON2(ERR_CODE_DISPLAY_ERROR, "无效的陪玩价格", nil)
		} else if len(req.GetAcceptSettings().GetRegionId()) == 0 || len(req.GetAcceptSettings().GetLevelId()) == 0 {
			return c.JSON2(ERR_CODE_DISPLAY_ERROR, "平台大区和段位不能为空", nil)
		}
		godLevel, ok := resp.GetData().GetPriceLevel()[req.GetUnitPriceId()]
		if !ok || godGame.Level < godLevel {
			return c.JSON2(ERR_CODE_DISPLAY_ERROR, "无效的陪玩价格", nil)
		}

	} else {
		// 接单开关关闭时，抢单开关自动关闭
		req.GrabSwitch = constants.GRAB_SWITCH_CLOSE
		req.GrabSwitch2 = constants.GRAB_SWITCH2_CLOSE
		req.GrabSwitch3 = constants.GRAB_SWITCH3_CLOSE
		req.GrabSwitch4 = constants.GRAB_SWITCH4_CLOSE

		gg.dao.AcceptQuickOrderSetting(currentUser.UserID, req.GameId, constants.GRAB_SWITCH5_CLOSE)
	}
	if req.GetGrabSwitch2() != constants.GRAB_SWITCH2_OPEN {
		req.GrabSwitch2 = constants.GRAB_SWITCH2_CLOSE
	}
	if req.GetGrabSwitch3() != constants.GRAB_SWITCH3_OPEN {
		req.GrabSwitch3 = constants.GRAB_SWITCH3_CLOSE
	}
	if req.GetGrabSwitch4() != constants.GRAB_SWITCH4_OPEN {
		req.GrabSwitch4 = constants.GRAB_SWITCH4_CLOSE
	}
	bs, err := json.Marshal(req.GetAcceptSettings())
	if err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	settings := model.ORMOrderAcceptSetting{
		GameID:      req.GetGameId(),
		GodID:       currentUser.UserID,
		RegionLevel: string(bs),
		GrabSwitch:  req.GetGrabSwitch(),
		GrabSwitch2: req.GetGrabSwitch2(),
		GrabSwitch3: req.GetGrabSwitch3(),
		GrabSwitch4: req.GetGrabSwitch4(),
		PriceID:     req.GetUnitPriceId(),
	}
	err = gg.dao.ModifyAcceptOrderSetting(settings)
	if err != nil {
		c.Warnf("ModifyAcceptOrderSetting error:%s", err)
		return c.JSON2(ERR_CODE_INTERNAL, "", nil)
	}
	// 神策埋点
	go func() {
		gg.shence.Track(fmt.Sprintf("%d", currentUser.UserID),
			"singleSwitch",
			map[string]interface{}{
				"newAppName":     currentUser.AppID,
				"singleSwitchok": req.GetGrabSwitch() == constants.GRAB_SWITCH_OPEN,
			}, true)
		gg.shence.Track(fmt.Sprintf("%d", currentUser.UserID),
			"snatchingSwitch",
			map[string]interface{}{
				"newAppName":        currentUser.AppID,
				"snatchingSwitchok": req.GetGrabSwitch2() == constants.GRAB_SWITCH2_OPEN,
			}, true)
		gg.shence.Track(fmt.Sprintf("%d", currentUser.UserID),
			"PaidanSwitch",
			map[string]interface{}{
				"newAppName":     currentUser.AppID,
				"PaidanSwitchok": req.GetGrabSwitch3() == constants.GRAB_SWITCH3_OPEN,
			}, true)
	}()
	if gg.isVoiceCallGame(req.GetGameId()) {
		// 语聊品类
		redisConn := gg.dao.GetPlayRedisPool().Get()
		defer redisConn.Close()
		if req.GetGrabSwitch() == constants.GRAB_SWITCH_CLOSE {
			redisConn.Do("ZREM", core.RKVoiceCallGods(), currentUser.UserID)
		} else {
			if req.GetGrabSwitch4() == constants.GRAB_SWITCH4_OPEN {
				// 随机模式开关打开
				redisConn.Do("ZADD", core.RKVoiceCallGods(), 1, currentUser.UserID)
				pumpkinpb.RefreshAvatars(c, nil)
			} else {
				redisConn.Do("ZADD", core.RKVoiceCallGods(), 2, currentUser.UserID)
			}
		}
		go gg.shence.Track(fmt.Sprintf("%d", currentUser.UserID),
			"volteMatchingSwitch",
			map[string]interface{}{
				"newAppName": currentUser.AppID,
				"Switchok":   req.GetGrabSwitch4() == constants.GRAB_SWITCH4_OPEN,
			}, true)
	} else {
		// 非语聊品类
		redisConn := gg.dao.GetPlayRedisPool().Get()
		defer redisConn.Close()
		if resp.GetData().GetJsy() == game_const.GAME_SUPPORT_JSY_YES {
			jsyKey := core.RKJSYGods(req.GetGameId(), godInfo.Gender)
			jsyPaiDanKey := core.RKJSYPaiDanGods(req.GetGameId(), godInfo.Gender)
			if req.GetGrabSwitch2() == constants.GRAB_SWITCH2_OPEN {
				redisConn.Do("ZADD", jsyKey, time.Now().Unix(), currentUser.UserID)
			} else {
				redisConn.Do("ZREM", jsyKey, currentUser.UserID)
			}
			if req.GetGrabSwitch3() == constants.GRAB_SWITCH3_OPEN {
				redisConn.Do("ZADD", jsyPaiDanKey, time.Now().Unix(), currentUser.UserID)
			} else {
				redisConn.Do("ZREM", jsyPaiDanKey, currentUser.UserID)
			}
		}
		if godGame.GrabStatus != constants.GRAB_STATUS_YES {
			return c.JSON2(StatusOK_V3, "", nil)
		}
		for _, region := range godGame.Regions {
			for _, level := range godGame.Levels {
				redisConn.Do("ZREM", core.GodsRedisKey3(req.GetGameId(), region, level), currentUser.UserID)
			}
		}
		if req.GetGrabSwitch2() == constants.GRAB_SWITCH2_OPEN {
			for _, tmpRegion := range req.GetAcceptSettings().GetRegionId() {
				for _, tmpLevel := range req.GetAcceptSettings().GetLevelId() {
					score, _ := resp.GetData().GetLevels()[tmpLevel]
					if score > highestLevelScore {
						icelog.Info("目标段位高于当前最高段位", score, highestLevelScore)
						continue
					}
					redisConn.Do("ZADD", core.GodsRedisKey3(req.GetGameId(), tmpRegion, tmpLevel), godGame.HighestLevelID, currentUser.UserID)
				}
			}
		}
	}

	if godGame.Recommend == constants.RECOMMEND_YES {
		esID := fmt.Sprintf("%d-%d", godGame.GodID, godGame.GameID)
		if req.GetGrabSwitch() == constants.GRAB_SWITCH_OPEN {
			oldData, err := gg.ESGetGodGame(esID)
			if err == nil {
				gg.ESUpdateGodGame(esID, map[string]interface{}{
					"lts":      time.Now(),
					"price_id": req.GetUnitPriceId(),
				})
			} else {
				oldData, err = gg.BuildESGodGameData(godGame.GodID, godGame.GameID)
				if err != nil {
					icelog.Warnf("BuildESGodGameData %s error %s", esID, err.Error())
				} else {
					oldData.LTS = time.Now()
					gg.ESAddGodGame(oldData)
				}
			}
		} else {
			gg.ESDeleteGodGame(esID)
		}
	}
	return c.JSON2(StatusOK_V3, "", nil)
}

// 大神定向单接单设置数据查询
func (gg *GodGame) Dxd(c frame.Context) error {
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
		// if v1.GameID == 15 && isIOS {
		// 	// iOS下定向单，王者荣耀不展示Android大区
		// 	tmpRegions := make([]*gamepb.Region2, 0, 2)
		// 	tmpRegionIDs := make([]int64, 0, 2)
		// 	if len(dxdResp.GetData().GetRegion1()) > 0 {
		// 		for _, region2 := range dxdResp.GetData().GetRegion1()[0].GetRegion2() {
		// 			if strings.Index(region2.GetName(), "安卓") != -1 {
		// 				continue
		// 			}
		// 			tmpRegionIDs = append(tmpRegionIDs, region2.GetId())
		// 			tmpRegions = append(tmpRegions, region2)
		// 		}
		// 		if len(tmpRegions) == 0 {
		// 			continue
		// 		}
		// 		dxdResp.GetData().GetRegion1()[0].Region2 = tmpRegions
		// 		game["regions"] = tmpRegionIDs
		// 		game["region1"] = dxdResp.GetData().GetRegion1()
		// 	}
		// } else {
		game["regions"] = v1.Regions
		game["region1"] = dxdResp.GetData().GetRegion1()
		// }

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

// 修改大神自定义介绍
func (gg *GodGame) ModifyDesc(c frame.Context) error {
	var req godgamepb.ModifyDescReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "参数错误[1]", nil)
	}
	if req.GetDesc() != "" {
		if len([]rune(req.GetDesc())) > 1000 {
			return c.JSON2(ERR_CODE_BAD_REQUEST, "内容不能超过1000字", nil)
		}
		resp, err := keywordpb.Check(c, &keywordpb.CheckReq{
			Channel: "level1",
			Content: req.GetDesc(),
		})
		if err == nil && resp.GetErrcode() == 0 && len(resp.GetData()) > 0 {
			return c.JSON2(ERR_CODE_FORBIDDEN, "内容包含敏感词："+resp.GetData()[0]+"等", nil)
		}
	}
	currentUserID := gg.getCurrentUserID(c)
	if err = gg.dao.ModifyGodDesc(currentUserID, req.GetDesc()); err != nil {
		c.Errorf("ModifyDesc error %s", err)
		return c.JSON2(ERR_CODE_INTERNAL, "", nil)
	}

	return c.JSON2(StatusOK_V3, "", nil)
}

func (gg *GodGame) buildGodDetail(c frame.Context, godID, gameID int64) (map[string]interface{}, error) {
	godInfo := gg.dao.GetGod(godID)
	if godInfo.Status != constants.GOD_STATUS_PASSED {
		return nil, fmt.Errorf("invalid god status %d", godInfo.Status)
	}
	v1, err := gg.dao.GetGodSpecialGameV1(godID, gameID)
	if err != nil {
		return nil, err
	}
	userinfo, err := gg.getSimpleUser(v1.GodID)
	if err != nil {
		return nil, err
	}
	gameRecord, err := gamepb.Record(c, &gamepb.RecordReq{
		GameId: gameID,
	})
	if err != nil || gameRecord.GetErrcode() != 0 {
		return nil, fmt.Errorf("game %d record not found", gameID)
	}
	resp, err := gamepb.AcceptCfgV2(c, &gamepb.AcceptCfgV2Req{
		GameId: gameID,
	})
	if err != nil {
		return nil, err
	}

	var uniprice int64
	if v1.PriceType == constants.PW_PRICE_TYPE_BY_OM {
		uniprice = v1.PeiWanPrice
	} else {
		uniprice = resp.GetData().GetPrices()[v1.PriceID]
	}
	var regionDesc string
	if len(v1.Regions) > 0 {
		for _, region := range v1.Regions {
			regionDesc = regionDesc + resp.GetData().GetRegionDesc()[region] + ","
		}
		regionDesc = regionDesc[:len(regionDesc)-1]
	}

	var freeStatus int64
	var freeStatusDesc string
	var roomID int64
	var template int32

	liveResp, err := livepb.GetGodLiveId(c, &livepb.GetGodLiveIdReq{
		GodId:  v1.GodID,
		GameId: v1.GameID,
	})
	if err == nil && liveResp.GetData() != nil && liveResp.GetData().GetRoomId() > 0 {
		// 优先返回直播
		freeStatus = order_const.PW_STATUS_LIVE
		freeStatusDesc = order_const.PW_STATS_DESC[order_const.PW_STATUS_LIVE]
		roomID = liveResp.GetData().GetRoomId()
	} else {
		freeResp, err := plorderpb.Free(c, &plorderpb.FreeReq{
			GodId: v1.GodID,
		})
		if err != nil || freeResp.GetErrcode() != 0 {
			freeStatus = order_const.PW_STATUS_FREE
			freeStatusDesc = order_const.PW_STATS_DESC[order_const.PW_STATUS_FREE]
		} else {
			freeStatus = freeResp.GetData().GetStatus()
			freeStatusDesc = freeResp.GetData().GetStatusDesc()
			if freeStatus == order_const.PW_STATUS_FREE {
				seatResp, err := pb_chatroom.IsOnSeat(c, &pb_chatroom.IsOnSeatReq{
					UserId: v1.GodID,
				})
				if err == nil && seatResp.GetData() != nil {
					freeStatus = order_const.PW_STATUS_ON_SEAT
					freeStatusDesc = order_const.PW_STATS_DESC[order_const.PW_STATUS_ON_SEAT]
					roomID = seatResp.GetData().GetRoomId()
					template = seatResp.GetData().GetTemplate()
				}
			}
		}
	}
	var tmpImages, tmpTags, tmpPowers []string
	var tmpExt interface{}
	json.Unmarshal([]byte(v1.Images), &tmpImages)
	json.Unmarshal([]byte(v1.Tags), &tmpTags)
	json.Unmarshal([]byte(v1.Powers), &tmpPowers)
	json.Unmarshal([]byte(v1.Ext), &tmpExt)
	if v1.GameScreenshot != "" {
		tmpImages = append(tmpImages, v1.GameScreenshot)
	}
	for idx, _ := range tmpImages {
		tmpImages[idx] = tmpImages[idx] + "/w0"
	}
	data := map[string]interface{}{
		"god_id":             v1.GodID,
		"god_name":           userinfo.GetUsername(),
		"god_avatar":         userinfo.GetAvatarSmall(),
		"sex":                godInfo.Gender,
		"age":                lyg_util.Age(userinfo.GetBirthday()),
		"game_id":            v1.GameID,
		"level":              v1.Level,
		"level_desc":         fmt.Sprintf("大神 Lv%d", v1.Level),
		"highest_level_id":   v1.HighestLevelID,
		"highest_level_desc": resp.GetData().GetLevelDesc()[v1.HighestLevelID],
		"region_accept_desc": regionDesc,
		"god_show_images":    tmpImages,
		"powers":             tmpPowers,
		"voice":              v1.Voice,
		"voice_duration":     v1.VoiceDuration,
		"aac":                v1.Aac,
		"god_icon":           v1.GodIcon,
		"god_tags":           tmpTags,
		"ext":                tmpExt,
		"desc":               v1.Desc,
		"uniprice":           uniprice,
		"gl":                 FormatRMB2Gouliang(uniprice),
		"order_cnt":          v1.AcceptNum,
		"order_cnt_desc":     FormatAcceptOrderNumber3(v1.AcceptNum),
		"order_rate":         "100%",
		"regions":            v1.Regions,
		"levels":             v1.Levels,
		"score":              v1.Score,
		"score_desc":         FormatScore(v1.Score),
		"status":             freeStatus,
		"status_desc":        freeStatusDesc,
		"room_id":            roomID,
		"template":           template,
		"shareurl":           gg.GenPeiWanShareURL(userinfo.GetAvatarBig(), userinfo.GetUsername(), gameRecord.GetData().GetGameName(), v1.Desc, v1.GodID, v1.GameID),
	}
	if v1.Video != "" {
		tmpStr := gg.formatVideoInfo(c, v1.Video)
		data["video"] = tmpStr
		data["videos"] = []string{tmpStr}
	}
	if v1.Videos != "" {
		var tmpVideos []string
		err = json.Unmarshal([]byte(v1.Videos), &tmpVideos)
		if err == nil && len(tmpVideos) > 0 {
			for idx, _ := range tmpVideos {
				tmpVideos[idx] = gg.formatVideoInfo(c, tmpVideos[idx])
			}
			data["videos"] = tmpVideos
		}
	}
	if orderPercent, err := plorderpb.OrderFinishPercent(c, &plorderpb.OrderFinishPercentReq{
		GodId: v1.GodID,
		Days:  7,
	}); err == nil && orderPercent.GetErrcode() == 0 {
		data["order_rate"] = orderPercent.GetData()
	}
	commentData, _ := plcommentpb.GetGodGameComment(c, &plcommentpb.GetGodGameCommentReq{
		GodId:  godID,
		GameId: gameID,
	})
	if commentData != nil && commentData.GetData() != nil {
		data["comments_cnt"] = commentData.GetData().GetCommentCnt()
		data["tags"] = commentData.GetData().GetTags()
	}
	hotComments, _ := plcommentpb.GetHotComments(c, &plcommentpb.GetHotCommentsReq{
		GodId:  godID,
		GameId: gameID,
	})
	if hotComments != nil && len(hotComments.GetData()) > 0 {
		data["comments"] = hotComments.GetData()
	}
	return data, nil
}

func (gg *GodGame) OldInfo(c frame.Context) error {
	currentUserID := gg.getCurrentUserID(c)
	god := gg.dao.GetGod(currentUserID)
	if god.UserID != currentUserID {
		return c.JSON2(ERR_CODE_NOT_FOUND, "", nil)
	}
	data := map[string]interface{}{
		"god_id":      god.UserID,
		"realname":    god.RealName,
		"sex":         god.Gender,
		"phone":       god.Phone,
		"idcard_type": god.IDcardtype,
		"idcard":      god.IDcard,
		"idcard_url":  GenIDCardURL(god.IDcardurl, gg.cfg.OSS.OSSAccessID, gg.cfg.OSS.OSSAccessKey),
		"createdtime": FormatDatetime(god.Createdtime),
		"updatedtime": FormatDatetime(god.Updatedtime),
	}
	return c.RetSuccess("success", data)
}
