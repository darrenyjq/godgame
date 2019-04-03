package api

import (
	"context"
	"fmt"
	"github.com/olivere/elastic"
	"godgame/core"
	"iceberg/frame"
	"iceberg/frame/icelog"
	"laoyuegou.com/geo"
	lyg_util "laoyuegou.com/util"
	game_const "laoyuegou.pb/game/constants"
	"laoyuegou.pb/game/pb"
	"laoyuegou.pb/godgame/constants"
	"laoyuegou.pb/godgame/model"
	"laoyuegou.pb/godgame/pb"
	"laoyuegou.pb/imapi/pb"
	"laoyuegou.pb/plcomment/pb"
	"laoyuegou.pb/plorder/pb"
	purse_pb "laoyuegou.pb/purse/pb"
	sapb "laoyuegou.pb/sa/pb"
	"laoyuegou.pb/union/pb"
	"laoyuegou.pb/user/pb"
	"strconv"
	"strings"
	"time"
)

// 大神审核
func (gg *GodGame) GodAudit(c frame.Context) error {
	var req godgamepb.GodAuditReq
	if err := c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	} else if req.GetGodId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "大神ID为空", nil)
	} else if req.GetGameId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "游戏ID为空", nil)
	} else if req.GetStatus() == constants.GOD_GAME_APPLY_STATUS_REFUSED && req.GetReason() == "" {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "请提供拒绝原因", nil)
	}
	if req.GetRecommend() != constants.RECOMMEND_YES {
		req.Recommend = constants.RECOMMEND_NO
	}
	if req.GetGrabStatus() != constants.GRAB_STATUS_YES {
		req.GrabStatus = constants.GRAB_STATUS_NO
	}
	recordResp, err := gamepb.Record(frame.TODO(), &gamepb.RecordReq{GameId: req.GetGameId()})
	if err != nil || recordResp.GetErrcode() != 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "游戏数据加载失败", nil)
	}
	newGame := gg.dao.GetGodGame(req.GetGodId(), req.GetGameId()).UserID == 0
	isGod, err := gg.dao.GodGameAudit(req.GetStatus(), req.GetGameId(), req.GetGodId(), req.GetRecommend(), req.GetGrabStatus())
	if err != nil {
		if strings.Index(err.Error(), "uni_phone") != -1 {
			return c.JSON2(ERR_CODE_DISPLAY_ERROR, "手机号已使用", nil)
		}
		return c.JSON2(ERR_CODE_INTERNAL, err.Error(), nil)
	}
	gameName := recordResp.GetData().GetGameName()
	var msg imapipb.CommonSystemMessage
	var push imapipb.PushNotification

	if req.GetStatus() == constants.GOD_GAME_STATUS_PASSED {
		if newGame {
			msg = imapipb.CommonSystemMessage{
				Title:   fmt.Sprintf("%s大神审核通过", gameName),
				Content: fmt.Sprintf("恭喜您已成为%s大神，赶快去设置接单吧", gameName),
				JText:   "查看详情",
				JUrl:    imapipb.LYG_URL_WSDS,
			}
			push = imapipb.PushNotification{
				UserDefine: imapipb.PushUserDefine{UrlScheme: imapipb.LYG_URL_SYSTEM_MSG}.Marshall(),
				Title:      fmt.Sprintf("%s大神审核通过", gameName),
				Desc:       fmt.Sprintf("恭喜您已成为%s大神，赶快去设置接单吧", gameName),
				Sound:      "default",
			}
			// 神策埋点
			go func() {
				gg.shence.ProfileSet(fmt.Sprintf("%d", req.GetGodId()),
					map[string]interface{}{
						"godplayerok": true,
						"frozen":      false,
					}, true)
				gg.shence.Track(fmt.Sprintf("%d", req.GetGodId()),
					"ApplyforGod",
					map[string]interface{}{
						"Godtype": gameName,
					}, true)
			}()
		} else {
			// 更新上一次修改资料时间，限制一周只可以改一次
			gg.dao.ModifyLastModifyInfoTime(req.GetGodId(), req.GetGameId())

			msg = imapipb.CommonSystemMessage{
				Title:   fmt.Sprintf("%s资料修改已通过", gameName),
				Content: fmt.Sprintf("您的%s资料修改通过，赶快去看看吧", gameName),
				JText:   "查看详情",
				JUrl:    imapipb.LYG_URL_WSDS,
			}
			push = imapipb.PushNotification{
				UserDefine: imapipb.PushUserDefine{UrlScheme: imapipb.LYG_URL_SYSTEM_MSG}.Marshall(),
				Title:      fmt.Sprintf("%s资料修改已通过", gameName),
				Desc:       fmt.Sprintf("您的%s资料修改通过，赶快去看看吧", gameName),
				Sound:      "default",
			}
		}
		if !isGod {
			// gg.sendGodStatusChangeCMD(req.GetGodId(), constants.GOD_STATUS_PASSED)
			msg2, _ := json.Marshal(map[string]interface{}{
				"status": constants.GOD_STATUS_PASSED,
			})
			imapipb.SendSystemNotify(c, &imapipb.SendSystemNotifyReq{
				Subtype: 6014,
				Message: string(msg2),
				Apn:     "",
				Ext:     "",
				ToId:    req.GetGodId(),
			})

			ext := imapipb.SingleImgTextExt{
				Title: "大神接单攻略",
				Img:   "https://s7.lygou.cc/hot_res/jiedangonglue.jpg",
				Href:  "laoyuegou://enterfeed?result=%7b%22id%22%3a%22575045%22%2c%22item_type%22%3a%221%22%7d",
			}
			extContent, _ := json.Marshal(ext)
			imapipb.SendPublicMsg(c, &imapipb.SendPublicMsgReq{
				ContentType: imapipb.MESSAGE_CONTENT_TYPE_SINGLE_IMAGE_TEXT,
				Subtype:     imapipb.MESSAGE_SUBTYPE_CHAT,
				Message:     "大神接单攻略",
				Apn:         "",
				Ext:         string(extContent),
				PubId:       imapipb.PUBLIC_XIAO_MI_SHU,
				ToId:        req.GetGodId(),
			})
		}
	} else if req.GetStatus() == constants.GOD_GAME_APPLY_STATUS_REFUSED {
		if newGame {
			msg = imapipb.CommonSystemMessage{
				Title:   fmt.Sprintf("%s大神审核未通过", gameName),
				Content: fmt.Sprintf("很抱歉%s大神申请未通过，失败原因：%s", gameName, req.GetReason()),
			}
			push = imapipb.PushNotification{
				Title:      fmt.Sprintf("%s大神审核未通过", gameName),
				Desc:       "很抱歉大神申请未通过，请查看失败原因或重新申请",
				UserDefine: imapipb.PushUserDefine{UrlScheme: imapipb.LYG_URL_SYSTEM_MSG}.Marshall(),
				Sound:      "default",
			}
		} else {
			msg = imapipb.CommonSystemMessage{
				Title:   fmt.Sprintf("%s资料修改未通过", gameName),
				Content: fmt.Sprintf("%s大神修改资料未通过，失败原因：%s", gameName, req.GetReason()),
			}
			push = imapipb.PushNotification{
				Title:      fmt.Sprintf("%s资料修改未通过", gameName),
				Desc:       fmt.Sprintf("%s大神修改资料未通过", gameName),
				UserDefine: imapipb.PushUserDefine{UrlScheme: imapipb.LYG_URL_SYSTEM_MSG}.Marshall(),
				Sound:      "default",
			}
		}
	}
	msgContent, _ := json.Marshal(msg)
	pushContent, _ := json.Marshal(push)
	imapipb.SendSystemNotify(c, &imapipb.SendSystemNotifyReq{
		Subtype: 5001,
		Message: string(msgContent),
		Apn:     string(pushContent),
		Ext:     "",
		ToId:    req.GetGodId(),
	})
	if req.GetRecommend() == constants.RECOMMEND_YES {
		data, err := gg.BuildESGodGameData(req.GetGodId(), req.GetGameId())
		if err != nil {
			return c.JSON2(StatusOK_V3, "", nil)
		}
		data.LTS = time.Now()
		err = gg.ESAddGodGame(data)
		if err != nil {
			return c.JSON2(StatusOK_V3, "", nil)
		}
	} else {
		esID := fmt.Sprintf("%d-%d", req.GetGodId(), req.GetGameId())
		data, err := gg.ESGetGodGame(esID)
		if err == nil && data.GodID == req.GetGodId() {
			err = gg.ESDeleteGodGame(esID)
			if err != nil {
				return c.JSON2(StatusOK_V3, "", nil)
			}
		}
	}
	return c.JSON2(StatusOK_V3, "", nil)
}

// 陪玩品类列表，已通过、审核中、已冻结状态
func (gg *GodGame) OMGodGames(c frame.Context) error {
	var req godgamepb.OMGodGamesReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	var userV1 core.UserInfoV1
	var godID int64
	if req.GetGouhao() > 0 {
		userV1, err = gg.dao.UserV1ByGouHao(req.GetGouhao())
		if err != nil {
			return c.JSON2(ERR_CODE_BAD_REQUEST, "用户信息查询失败", nil)
		}
		godID = userV1.UserID
	}
	godGames, err := gg.dao.GetGodGameApplys(req.GetStatus(), req.GetGameId(), godID, req.GetOffset(), req.GetGender(), req.GetLeaderId())
	if err != nil {
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, err.Error(), nil)
	} else if len(godGames) == 0 {
		return c.JSON2(StatusOK_V3, "", nil)
	}
	items := make([]map[string]interface{}, 0, len(godGames))
	var item, godInfo map[string]interface{}
	var god *model.God
	var godIcon *model.TmpGodIcon
	for _, godGame := range godGames {
		userV1, err = gg.dao.UserV1ByID(godGame.UserID)
		if err != nil {
			continue
		}
		god = gg.dao.GetGodByUserID(godGame.UserID)
		if god == nil {
			continue
		}
		var tmpImages, tmpTags, tmpExt, tmpPowers interface{}
		// item = make(map[string]interface{})
		item = map[string]interface{}{
			"god_id":           godGame.UserID,
			"god_level":        godGame.GodLevel,
			"game_id":          godGame.GameID,
			"region_id":        godGame.RegionID,
			"highest_level_id": godGame.HighestLevelID,
			"game_screenshot":  godGame.GameScreenshot,
			"voice":            godGame.Voice,
			"voice_duration":   godGame.VoiceDuration,
			"aac":              godGame.Aac,
			"desc":             godGame.Desc,
			"createdtime":      FormatDatetime(godGame.Createdtime),
			"updatedtime":      FormatDatetime(godGame.Updatedtime),
			"status":           godGame.Status,
			"grab_status":      godGame.GrabStatus,
			"recommend":        godGame.Recommend,
		}
		if godGame.Video != "" {
			item["video"] = gg.formatVideoInfo2(c, godGame.Video)
		}
		if godGame.Videos != "" {
			var tmpVideos []string
			err = json.Unmarshal([]byte(godGame.Videos), &tmpVideos)
			if err == nil && len(tmpVideos) > 0 {
				for idx, _ := range tmpVideos {
					tmpVideos[idx] = gg.formatVideoInfo2(c, tmpVideos[idx])
				}
				item["videos"] = tmpVideos
			}
		} else if godGame.Video != "" {
			item["videos"] = []string{item["video"].(string)}
		}
		json.Unmarshal([]byte(godGame.Images), &tmpImages)
		json.Unmarshal([]byte(godGame.Tags), &tmpTags)
		json.Unmarshal([]byte(godGame.Ext), &tmpExt)
		json.Unmarshal([]byte(godGame.Powers), &tmpPowers)
		item["god_imgs"] = tmpImages
		item["tags"] = tmpTags
		item["ext"] = tmpExt
		item["powers"] = tmpPowers
		godInfo = make(map[string]interface{})
		godInfo = map[string]interface{}{
			"userid":      userV1.UserID,
			"gouhao":      fmt.Sprintf("%d", userV1.GouHao),
			"username":    userV1.NickName,
			"realname":    god.RealName,
			"idcard":      god.IDcard,
			"idcard_type": god.IDcardtype,
			"idcardurl":   GenIDCardURL(god.IDcardurl, gg.cfg.OSS.OSSAccessID, gg.cfg.OSS.OSSAccessKey),
			"phone":       god.Phone,
			"sex":         god.Gender,
			"birthday":    userV1.Birthday,
			"status":      god.Status,
		}
		if godIcon, err = gg.dao.GetGodIcon(godGame.UserID); godIcon != nil && err == nil {
			godInfo["god_icon"] = godIcon
		}

		item["god_info"] = godInfo
		if godGame.Status == constants.GOD_GAME_STATUS_PASSED ||
			godGame.Status == constants.GOD_GAME_STATUS_BLOCKED {
			item["passedtime"] = godGame.Passedtime
			item["grab_status"] = godGame.GrabStatus
			item["recommend"] = godGame.Recommend
			item["peiwan_price_type"] = godGame.PeiwanPriceType
			item["peiwan_price"] = godGame.PeiwanPrice
			incomeResp, err := plorderpb.Income(frame.TODO(), &plorderpb.IncomeReq{
				GodId: godGame.UserID,
			})
			if err == nil && incomeResp.GetData() != nil {
				// 已通过状态的大神，还需要返回收入和接单数据
				item["month_income"] = incomeResp.GetData().GetMouth()
				item["total_income"] = incomeResp.GetData().GetTotal()
				item["balance"] = incomeResp.GetData().GetBalance()
			}
			orderResp, err := plorderpb.Count(frame.TODO(), &plorderpb.CountReq{
				GodId:  godGame.UserID,
				GameId: godGame.GameID,
			})
			if err == nil && orderResp.GetData() != nil {
				item["accept_num"] = orderResp.GetData().GetCompletedAmount()
				item["hour"] = orderResp.GetData().GetCompletedHoursAmount()
			}
			comment, err := plcommentpb.GetGodGameComment(frame.TODO(), &plcommentpb.GetGodGameCommentReq{
				GodId:  godGame.UserID,
				GameId: godGame.GameID,
			})
			if err == nil && comment.GetData() != nil {
				item["order_count"] = comment.GetData().GetScore()
			}
		}
		if userV1.LTS > 0 {
			item["last_login_time"] = FormatDatetimeV2(time.Unix(userV1.LTS, 0))
		}
		unionResp, err := unionpb.Member(c, &unionpb.MemberReq{
			MemberId: godGame.UserID,
		})
		if err == nil && unionResp.GetErrcode() == 0 {
			item["union_id"] = unionResp.GetData().GetUnionId()
			item["union_name"] = unionResp.GetData().GetUnionName()
		}
		items = append(items, item)
	}

	return c.JSON2(StatusOK_V3, "", map[string]interface{}{
		"count": len(items),
		"items": items,
	})
}

// 冻结大神
func (gg *GodGame) BlockGod(c frame.Context) error {
	var req godgamepb.BlockGodReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	} else if req.GetGodId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "大神ID不能为空", nil)
	} else if req.GetReason() == "" {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "请提供冻结原因", nil)
	}
	err = gg.dao.BlockGod(req.GetGodId())
	if err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "冻结大神失败", nil)
	}

	v1s, err := gg.dao.GetGodAllGameV1(req.GetGodId())
	if err == nil && len(v1s) > 0 {
		for _, v1 := range v1s {
			// 如果大神被推荐到首页，冻结则需要从首页下掉
			if v1.Recommend == constants.RECOMMEND_YES {
				gg.ESDeleteGodGame(fmt.Sprintf("%d-%d", req.GetGodId(), v1.GameID))
			}
			// 如果大神具有抢开黑单权限，需要从大神池删除
			if v1.GrabStatus == constants.GRAB_STATUS_YES {
				gg.dao.DisableGodGrabOrder(v1.GameID, req.GetGodId())
			}
			// 从即时约大神池删除
			gg.dao.RemoveFromJSYGodPool(v1.GameID, req.GetGodId())
			gg.dao.RemoveFromJSYPaiDanGodPool(v1.GameID, req.GetGodId())
		}
	}
	msg := imapipb.CommonSystemMessage{
		Title:   "大神身份冻结",
		Content: fmt.Sprintf("您的大神身份被冻结\n冻结原因：%s", req.GetReason()),
		JText:   "点击申诉",
		JUrl:    imapipb.LYG_URL_COMPLAIN,
	}
	push := imapipb.PushNotification{
		UserDefine: imapipb.PushUserDefine{UrlScheme: imapipb.LYG_URL_SYSTEM_MSG}.Marshall(),
		Title:      "大神身份冻结",
		Desc:       fmt.Sprintf("您的大神身份被冻结\n冻结原因：%s", req.GetReason()),
		Sound:      "default",
	}
	msgContent, _ := json.Marshal(msg)
	pushContent, _ := json.Marshal(push)
	imapipb.SendSystemNotify(c, &imapipb.SendSystemNotifyReq{
		Subtype: 5001,
		Message: string(msgContent),
		Apn:     string(pushContent),
		Ext:     "",
		ToId:    req.GetGodId(),
	})

	// send backgroup notify
	msg2, _ := json.Marshal(map[string]interface{}{
		"status": constants.GOD_STATUS_BLOCKED,
	})
	imapipb.SendSystemNotify(c, &imapipb.SendSystemNotifyReq{
		Subtype: 6014,
		Message: string(msg2),
		Apn:     "",
		Ext:     "",
		ToId:    req.GetGodId(),
	})
	go gg.shence.ProfileSet(fmt.Sprintf("%d", req.GetGodId()),
		map[string]interface{}{
			"godplayerok": true,
			"frozen":      true,
		}, true)
	return c.JSON2(StatusOK_V3, "", nil)
}

// 解除冻结大神
func (gg *GodGame) UnBlockGod(c frame.Context) error {
	var req godgamepb.UnBlockGodReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	} else if req.GetGodId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "大神ID不能为空", nil)
	}
	err = gg.dao.UnBlockGod(req.GetGodId())
	if err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "解除冻结大神失败", nil)
	}
	v1s, err := gg.dao.GetGodAllGameV1(req.GetGodId())
	if err == nil && len(v1s) > 0 {
		for _, v1 := range v1s {
			// 如果大神被推荐到首页，解冻则需要重新放回首页
			if v1.Recommend == constants.RECOMMEND_YES {
				oldData, err := gg.BuildESGodGameData(v1.GodID, v1.GameID)
				if err == nil {
					oldData.LTS = time.Now()
					err = gg.ESAddGodGame(oldData)
					if err != nil {
						icelog.Errorf("ESAddGodGame %d-%d error %s", v1.GodID, v1.GameID, err.Error())
					}
				} else {
					icelog.Errorf("BuildESGodGameData %d-%d error %s", v1.GodID, v1.GameID, err.Error())
				}
			}
		}
	}
	msg := imapipb.CommonSystemMessage{
		Title:   "大神身份解除冻结",
		Content: "您的大神身份被解除冻结，请重新设置接单",
		JText:   "设置接单",
		JUrl:    imapipb.LYG_URL_WSDS,
	}
	push := imapipb.PushNotification{
		UserDefine: imapipb.PushUserDefine{UrlScheme: imapipb.LYG_URL_SYSTEM_MSG}.Marshall(),
		Title:      "大神身份解除冻结",
		Desc:       "您的大神身份被解除冻结，请重新设置接单",
		Sound:      "default",
	}
	msgContent, _ := json.Marshal(msg)
	pushContent, _ := json.Marshal(push)
	imapipb.SendSystemNotify(c, &imapipb.SendSystemNotifyReq{
		Subtype: 5001,
		Message: string(msgContent),
		Apn:     string(pushContent),
		Ext:     "",
		ToId:    req.GetGodId(),
	})

	// send backgroup notify
	msg2, _ := json.Marshal(map[string]interface{}{
		"status": constants.GOD_STATUS_PASSED,
	})
	imapipb.SendSystemNotify(c, &imapipb.SendSystemNotifyReq{
		Subtype: 6014,
		Message: string(msg2),
		Apn:     "",
		Ext:     "",
		ToId:    req.GetGodId(),
	})
	go gg.shence.ProfileSet(fmt.Sprintf("%d", req.GetGodId()),
		map[string]interface{}{
			"godplayerok": true,
			"frozen":      false,
		}, true)
	return c.JSON2(StatusOK_V3, "", nil)
}

// 冻结大神品类
func (gg *GodGame) BlockGodGame(c frame.Context) error {
	var req godgamepb.BlockGodGameReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	} else if req.GetGodId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "大神ID不能为空", nil)
	} else if req.GetGameId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "品类ID不能为空", nil)
	} else if req.GetReason() == "" {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "请提供冻结原因", nil)
	}
	v1, err := gg.dao.GetGodSpecialGameV1(req.GetGodId(), req.GetGameId())
	if err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "陪玩信息加载失败", nil)
	}
	recordResp, err := gamepb.Record(frame.TODO(), &gamepb.RecordReq{GameId: req.GetGameId()})
	if err != nil || recordResp.GetErrcode() != 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "游戏数据加载失败", nil)
	}
	err = gg.dao.BlockGodGame(req.GetGodId(), req.GetGameId())
	if err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "冻结大神品类失败", nil)
	}
	msg := imapipb.CommonSystemMessage{
		Title:   fmt.Sprintf("%s大神身份冻结", recordResp.GetData().GetGameName()),
		Content: fmt.Sprintf("您的%s大神身份被冻结\n冻结原因：%s", recordResp.GetData().GetGameName(), req.GetReason()),
		JText:   "点击申诉",
		JUrl:    imapipb.LYG_URL_COMPLAIN,
	}
	push := imapipb.PushNotification{
		UserDefine: imapipb.PushUserDefine{UrlScheme: imapipb.LYG_URL_SYSTEM_MSG}.Marshall(),
		Title:      fmt.Sprintf("%s大神身份冻结", recordResp.GetData().GetGameName()),
		Desc:       fmt.Sprintf("您的%s大神身份被冻结\n冻结原因：%s", recordResp.GetData().GetGameName(), req.GetReason()),
		Sound:      "default",
	}
	msgContent, _ := json.Marshal(msg)
	pushContent, _ := json.Marshal(push)
	imapipb.SendSystemNotify(c, &imapipb.SendSystemNotifyReq{
		Subtype: 5001,
		Message: string(msgContent),
		Apn:     string(pushContent),
		Ext:     "",
		ToId:    req.GetGodId(),
	})
	// 如果大神被推荐到首页，冻结则需要从首页下掉
	if v1.Recommend == constants.RECOMMEND_YES {
		gg.ESDeleteGodGame(fmt.Sprintf("%d-%d", req.GetGodId(), req.GetGameId()))
	}
	// 如果大神具有抢开黑单权限，需要从大神池删除
	if v1.GrabStatus == constants.GRAB_STATUS_YES {
		gg.dao.DisableGodGrabOrder(req.GetGameId(), req.GetGodId())
	}
	// 从即时约大神池删除
	gg.dao.RemoveFromJSYGodPool(req.GetGameId(), req.GetGodId())
	gg.dao.RemoveFromJSYPaiDanGodPool(req.GetGameId(), req.GetGodId())
	return c.JSON2(StatusOK_V3, "", nil)
}

// 解除冻结大神游戏
func (gg *GodGame) UnBlockGodGame(c frame.Context) error {
	var req godgamepb.UnBlockGodGameReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	} else if req.GetGodId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "大神ID不能为空", nil)
	} else if req.GetGameId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "品类ID不能为空", nil)
	}
	godGame := gg.dao.GetGodGame(req.GetGodId(), req.GetGameId())
	if godGame.ID == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "陪玩数据加载失败", nil)
	}
	recordResp, err := gamepb.Record(frame.TODO(), &gamepb.RecordReq{GameId: req.GetGameId()})
	if err != nil || recordResp.GetErrcode() != 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "游戏数据加载失败", nil)
	}
	err = gg.dao.UnBlockGodGame(req.GetGodId(), req.GetGameId())
	if err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "解除冻结大神品类失败", nil)
	}
	msg := imapipb.CommonSystemMessage{
		Title:   fmt.Sprintf("%s大神身份解除冻结", recordResp.GetData().GetGameName()),
		Content: fmt.Sprintf("您的%s大神身份被解除冻结，请重新设置接单", recordResp.GetData().GetGameName()),
		JText:   "设置接单",
		JUrl:    imapipb.LYG_URL_WSDS,
	}
	push := imapipb.PushNotification{
		UserDefine: imapipb.PushUserDefine{UrlScheme: imapipb.LYG_URL_SYSTEM_MSG}.Marshall(),
		Title:      fmt.Sprintf("%s大神身份解除冻结", recordResp.GetData().GetGameName()),
		Desc:       fmt.Sprintf("您的%s大神身份被解除冻结，请重新设置接单", recordResp.GetData().GetGameName()),
		Sound:      "default",
	}
	msgContent, _ := json.Marshal(msg)
	pushContent, _ := json.Marshal(push)
	imapipb.SendSystemNotify(c, &imapipb.SendSystemNotifyReq{
		Subtype: 5001,
		Message: string(msgContent),
		Apn:     string(pushContent),
		Ext:     "",
		ToId:    req.GetGodId(),
	})
	if godGame.Recommend == constants.RECOMMEND_YES {
		oldData, err := gg.BuildESGodGameData(godGame.UserID, godGame.GameID)
		if err != nil {
			return c.JSON2(ERR_CODE_INTERNAL, "创建陪玩首页数据失败", nil)
		}
		oldData.LTS = time.Now()
		err = gg.ESAddGodGame(oldData)
		if err != nil {
			return c.JSON2(ERR_CODE_INTERNAL, "创建陪玩首页数据失败", nil)
		}
	}
	return c.JSON2(StatusOK_V3, "", nil)
}

// 设置大神是否可以接上分开黑单
func (gg *GodGame) ModifyGrabPermission(c frame.Context) error {
	var req godgamepb.ModifyGrabPermissionReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	} else if req.GetGodId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "大神ID不能为空", nil)
	} else if req.GetGameId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "品类ID不能为空", nil)
	} else if req.GetGrabStatus() != constants.GRAB_STATUS_YES {
		req.GrabStatus = constants.GRAB_STATUS_NO
	}
	err = gg.dao.ModifyGrabPermission(req.GetGodId(), req.GetGameId(), req.GetGrabStatus())
	if err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "修改开黑上分权限失败", nil)
	}
	return c.JSON2(StatusOK_V3, "", nil)
}

// 获取老的陪玩信息
func (gg *GodGame) OMOldData(c frame.Context) error {
	var req godgamepb.OMOldInfoReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	userInfo, err := gg.getSimpleUser(req.GetGodId())
	if err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	godInfo := gg.dao.GetGod(req.GetGodId())
	gameInfo := gg.dao.GetGodGame(req.GetGodId(), req.GetGameId())
	var tmpImgs, tmpTags, tmpExt, tmpPowers interface{}
	json.Unmarshal([]byte(gameInfo.Images), &tmpImgs)
	json.Unmarshal([]byte(gameInfo.Tags), &tmpTags)
	json.Unmarshal([]byte(gameInfo.Ext), &tmpExt)
	json.Unmarshal([]byte(gameInfo.Powers), &tmpPowers)
	var video string
	if gameInfo.Video != "" {
		video = gg.formatVideoInfo2(c, gameInfo.Video)
	}
	var tmpVideos []string
	if gameInfo.Videos != "" {
		err = json.Unmarshal([]byte(gameInfo.Videos), &tmpVideos)
		if err == nil && len(tmpVideos) > 0 {
			for idx, _ := range tmpVideos {
				tmpVideos[idx] = gg.formatVideoInfo2(c, tmpVideos[idx])
			}
		}
	} else if gameInfo.Video != "" {
		tmpVideos = []string{video}
	}
	return c.JSON2(StatusOK_V3, "", map[string]interface{}{
		"god_info": map[string]interface{}{
			"userid":      userInfo.GetUserId(),
			"realname":    godInfo.RealName,
			"idcard":      godInfo.IDcard,
			"idcardtype":  godInfo.IDcardtype,
			"idcardurl":   godInfo.IDcardurl,
			"phone":       godInfo.Phone,
			"createdtime": FormatDatetime(godInfo.Createdtime),
			"updatedtime": FormatDatetime(godInfo.Updatedtime),
			"status":      godInfo.Status,
			"sex":         godInfo.Gender,
			"birthday":    userInfo.GetBirthday(),
			"gouhao":      fmt.Sprintf("%d", userInfo.GetGouhao()),
			"username":    userInfo.GetUsername(),
		},
		"game_info": map[string]interface{}{
			"god_id":            userInfo.GetUserId(),
			"god_level":         gameInfo.GodLevel,
			"game_id":           gameInfo.GameID,
			"region_id":         gameInfo.RegionID,
			"highest_level_id":  gameInfo.HighestLevelID,
			"game_screenshot":   gameInfo.GameScreenshot,
			"god_imgs":          tmpImgs,
			"powers":            tmpPowers,
			"voice":             gameInfo.Voice,
			"aac":               gameInfo.Aac,
			"voice_duration":    gameInfo.VoiceDuration,
			"video":             video,
			"videos":            tmpVideos,
			"tags":              tmpTags,
			"ext":               tmpExt,
			"desc":              gameInfo.Desc,
			"createdtime":       FormatDatetime(gameInfo.Createdtime),
			"updatedtime":       FormatDatetime(gameInfo.Updatedtime),
			"passedtime":        FormatDatetime(gameInfo.Passedtime),
			"status":            gameInfo.Status,
			"grab_status":       gameInfo.GrabStatus,
			"recommend":         gameInfo.Recommend,
			"peiwan_price_type": gameInfo.PeiwanPriceType,
			"peiwan_price":      gameInfo.PeiwanPrice,
		},
	})
}

// 修改陪玩首页权重
func (gg *GodGame) ModifyUpperGodGame(c frame.Context) error {
	var req godgamepb.ModifyUpperGodGameReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	if req.GetWeight() != 0 {
		v1, err := gg.dao.GetGodSpecialGameV1(req.GetGodId(), req.GetGameId())
		if err != nil {
			return c.JSON2(ERR_CODE_BAD_REQUEST, "陪玩数据加载失败："+err.Error(), nil)
		} else if v1.Recommend != constants.RECOMMEND_YES {
			return c.JSON2(ERR_CODE_BAD_REQUEST, "请先将大神设置展示在首页", nil)
		}
	}
	err = gg.dao.ModifyUpperGodGame(req.GetGodId(), req.GetGameId(), req.GetWeight())
	if err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	if esGodGame, err := gg.BuildESGodGameData(req.GetGodId(), req.GetGameId()); err == nil {
		esGodGame.LTS = time.Now()
		gg.ESAddGodGame(esGodGame)
	}
	return c.JSON2(StatusOK_V3, "", nil)
}

// 获取陪玩首页权重列表
func (gg *GodGame) GetUpperGodGames(c frame.Context) error {
	items, items3, err := gg.dao.GetUpperGodGames()
	if err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	} else if len(items) == 0 {
		return c.JSON2(StatusOK_V3, "", nil)
	}
	items2 := make([]*godgamepb.GetUpperGodGamesResp_GodGame, 0, len(items))
	var section []string
	var godID, gameID int64
	var userInfo *userpb.UserInfo
	for _, k := range items3 {
		section = strings.Split(k, "-")
		if len(section) != 2 {
			continue
		}
		godID, _ = strconv.ParseInt(section[0], 10, 64)
		gameID, _ = strconv.ParseInt(section[1], 10, 64)
		userInfo, err = gg.getSimpleUser(godID)
		if err != nil || userInfo == nil {
			continue
		}
		items2 = append(items2, &godgamepb.GetUpperGodGamesResp_GodGame{
			GameId:   gameID,
			GodId:    godID,
			Gouhao:   userInfo.GetGouhao(),
			Username: userInfo.GetUsername(),
			Weight:   items[k],
		})
	}
	return c.JSON2(StatusOK_V3, "", &godgamepb.GetUpperGodGamesResp_Data{
		Count: int64(len(items2)),
		Items: items2,
	})
}

// 修改大神陪玩信息
func (gg *GodGame) ModifyGodGame(c frame.Context) error {
	var req godgamepb.ModifyGodGameReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	godGame := gg.dao.GetGodGame(req.GetGodId(), req.GetGameId())
	if godGame.ID == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "陪玩数据加载失败", nil)
	}
	oldGrabStatus := godGame.GrabStatus
	if req.GetPeiwanPriceType() == constants.PW_PRICE_TYPE_BY_OM {
		if req.GetPeiwanPrice() == 0 {
			return c.JSON2(ERR_CODE_BAD_REQUEST, "无效的陪玩价格", nil)
		}
	} else if req.GetPeiwanPriceType() == constants.PW_PRICE_TYPE_BY_ID {
		// TODO: chekc price id
	} else {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "无效的陪玩价格类型", nil)
	}
	if req.GetGameScreenshot() == "" {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "战绩截图不能为空", nil)
	}
	if len(req.GetGodImgs()) == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "陪玩形象照不能为空", nil)
	} else if len(req.GetGodImgs()) > 6 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "陪玩形象照最多6张", nil)
	}
	if req.GetRecommend() != constants.RECOMMEND_YES {
		req.Recommend = constants.RECOMMEND_NO
	}
	acceptCfgResp, err := gamepb.AcceptCfgV2(frame.TODO(), &gamepb.AcceptCfgV2Req{
		GameId: req.GetGameId(),
	})
	if err != nil || acceptCfgResp == nil {
		return c.JSON2(ERR_CODE_INTERNAL, "", nil)
	}
	if _, ok := acceptCfgResp.GetData().GetRegions()[req.GetRegionId()]; !ok {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "无效的平台大区", nil)
	}
	if _, ok := acceptCfgResp.GetData().GetLevels()[req.GetHighestLevelId()]; !ok {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "无效的最高段位", nil)
	}
	godGame.RegionID = req.GetRegionId()
	godGame.HighestLevelID = req.GetHighestLevelId()
	godGame.GodLevel = req.GetGodLevel()
	godGame.GrabStatus = req.GetGrabStatus()
	godGame.GameScreenshot = req.GetGameScreenshot()
	godGame.Recommend = req.GetRecommend()
	if bs, err := json.Marshal(req.GetGodImgs()); err == nil {
		godGame.Images = string(bs)
	}
	godGame.PeiwanPriceType = req.GetPeiwanPriceType()
	godGame.PeiwanPrice = req.GetPeiwanPrice()
	godGame.Updatedtime = time.Now()
	err = gg.dao.ModifyGodGameInfo(godGame)
	if err != nil {
		return c.JSON2(ERR_CODE_INTERNAL, err.Error(), nil)
	}
	if oldGrabStatus == constants.GRAB_STATUS_YES && req.GetGrabStatus() == constants.GRAB_STATUS_NO {
		// 取消大神抢单权限需要将大神从大神池移除
		gg.dao.DisableGodGrabOrder(req.GetGameId(), req.GetGodId())
	}
	oldData, err := gg.ESGetGodGame(fmt.Sprintf("%d-%d", godGame.UserID, godGame.GameID))
	if godGame.Recommend == constants.RECOMMEND_NO {
		if oldData.GodID == godGame.UserID {
			gg.ESDeleteGodGame(fmt.Sprintf("%d-%d", godGame.UserID, godGame.GameID))
		}
	} else {
		if oldData.GodID == 0 {
			oldData, err = gg.BuildESGodGameData(godGame.UserID, godGame.GameID)
			if err != nil {
				return c.JSON2(StatusOK_V3, "", nil)
			}
			oldData.LTS = time.Now()
			err = gg.ESAddGodGame(oldData)
			if err != nil {
				return c.JSON2(StatusOK_V3, "", nil)
			}
		} else {
			err = gg.ESUpdateGodGame(fmt.Sprintf("%d-%d", godGame.UserID, godGame.GameID),
				map[string]interface{}{
					"highest_level_id": req.GetHighestLevelId(),
					"price":            req.GetPeiwanPrice()})
			if err != nil {
				return c.JSON2(StatusOK_V3, "", nil)
			}
		}
	}
	return c.JSON2(StatusOK_V3, "", nil)
}

// OM后台修改大神资料
func (gg *GodGame) ModifyGodInfo(c frame.Context) error {
	var req godgamepb.ModifyGodInfoReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	if req.GetGodId() == 0 || req.GetRealname() == "" || req.GetIdcard() == "" ||
		req.GetIdcardUrl() == "" || req.GetPhone() == "" ||
		(req.GetSex() != constants.GENDER_FEMALE && req.GetSex() != constants.GENDER_MALE) ||
		(req.GetLeaderSwitch() == constants.GOD_LEADER_SWITCH_OPEN && req.GetLeaderId() == 0) || len(req.GetCerts()) == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "参数错误", nil)
	}
	if !IDCARD_RE.MatchString(req.GetIdcard()) {
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "身份证号码格式有误", nil)
	}
	if !gg.dao.IsGod(req.GetGodId()) {
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "非大神身份，不可执行此操作", nil)
	}
	if req.GetLeaderSwitch() == constants.GOD_LEADER_SWITCH_OPEN && gg.dao.GetGodLeaderByID(req.GetLeaderId()) == nil {
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "会长不存在", nil)
	}
	certsBs, _ := json.Marshal(req.GetCerts())
	godHistory := model.GodsHistory{
		GodID:        req.GetGodId(),
		RealName:     req.GetRealname(),
		IDcard:       req.GetIdcard(),
		IDcardurl:    req.GetIdcardUrl(),
		Phone:        req.GetPhone(),
		Gender:       req.GetSex(),
		Alipay:       req.GetAlipay(),
		LeaderSwitch: req.GetLeaderSwitch(),
		LeaderID:     req.GetLeaderId(),
		CertsDB:      string(certsBs),
	}
	godHistory, err = gg.dao.ModifyGodInfo(godHistory)
	if err != nil {
		return c.JSON2(ERR_CODE_INTERNAL, err.Error(), nil)
	}
	return c.JSON2(StatusOK_V3, "", godHistory)
}

// OM后台新增会长信息
func (gg *GodGame) AddGodLeader(c frame.Context) error {
	var req godgamepb.AddGodLeaderReq
	if err := c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	if req.GetRealname() == "" || req.GetIdcard() == "" ||
		req.GetPhone() == "" || req.GetAlipay() == "" {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	} else if !IDCARD_RE.MatchString(req.GetIdcard()) {
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "身份证号码格式有误", nil)
	}
	godLeader := model.GodLeader{
		RealName:    req.GetRealname(),
		IDcard:      req.GetIdcard(),
		Phone:       req.GetPhone(),
		Alipay:      req.GetAlipay(),
		Createdtime: time.Now(),
	}
	ret, err := gg.dao.CreateGodLeader(godLeader)
	if err != nil {
		if strings.Index(err.Error(), "uni_idcard") != -1 {
			return c.JSON2(ERR_CODE_DISPLAY_ERROR, "已经存在的身份证号", nil)
		}
		return c.JSON2(ERR_CODE_INTERNAL, err.Error(), nil)
	}
	return c.JSON2(StatusOK_V3, "", ret)
}

// OM后台修改会长信息
func (gg *GodGame) ModifyGodLeader(c frame.Context) error {
	var req godgamepb.ModifyGodLeaderReq
	if err := c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	if req.GetLeaderId() == 0 || req.GetRealname() == "" || req.GetIdcard() == "" ||
		req.GetPhone() == "" || req.GetAlipay() == "" {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	} else if !IDCARD_RE.MatchString(req.GetIdcard()) {
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "身份证号码格式有误", nil)
	}
	godLeader := model.GodLeader{
		ID:          req.GetLeaderId(),
		RealName:    req.GetRealname(),
		IDcard:      req.GetIdcard(),
		Phone:       req.GetPhone(),
		Alipay:      req.GetAlipay(),
		Updatedtime: time.Now(),
	}
	ret, err := gg.dao.ModifyGodLeader(godLeader)
	if err != nil {
		if strings.Index(err.Error(), "uni_idcard") != -1 {
			return c.JSON2(ERR_CODE_DISPLAY_ERROR, "已经存在的身份证号", nil)
		}
		return c.JSON2(ERR_CODE_INTERNAL, err.Error(), nil)
	}
	return c.JSON2(StatusOK_V3, "", ret)
}

// OM后台根据身份证号码精确搜索会长信息
func (gg *GodGame) GodLeader(c frame.Context) error {
	var req godgamepb.GodLeaderReq
	if err := c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	if req.GetIdcard() == "" {
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "身份证号码不能为空", nil)
	}
	godLeader := gg.dao.GetGodLeaderByIDCard(req.GetIdcard())
	return c.JSON2(StatusOK_V3, "", godLeader)
}

// OM后台查看会长列表
func (gg *GodGame) GodLeaders(c frame.Context) error {
	var req godgamepb.GodLeadersReq
	if err := c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	godLeaders, err := gg.dao.QueryGodLeaders(req)
	if err != nil {
		return c.JSON2(ERR_CODE_INTERNAL, err.Error(), nil)
	}
	return c.JSON2(StatusOK_V3, "", map[string]interface{}{
		"items": godLeaders,
	})
}

// OM后台大神资料修改审核
func (gg *GodGame) GodModifyAudit(c frame.Context) error {
	var req godgamepb.GodModifyAuditReq
	if err := c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	if req.GetGodId() == 0 || (req.GetStatus() != 1 && req.GetStatus() != 3) {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "参数错误", nil)
	}
	err := gg.dao.AuditModifyGodInfo(req.GetGodId(), req.GetStatus())
	if err != nil {
		errmsg := "资料审核失败"
		if strings.Index(err.Error(), "uni_phone") != -1 {
			errmsg = "资料审核失败：手机号已被使用"
		} else if strings.Index(err.Error(), "uni_alipayaccount") != -1 {
			errmsg = "资料审核失败：支付宝账号已被使用"
		}
		return c.JSON2(ERR_CODE_INTERNAL, errmsg, nil)
	}
	return c.JSON2(StatusOK_V3, "", nil)
}

// OM获取大神资料审核列表
func (gg *GodGame) GodModifyList(c frame.Context) error {
	var req godgamepb.GodModifyListReq
	if err := c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	status, godID := req.GetStatus(), int64(0)
	if req.GetStatus() < 1 || req.GetStatus() > 3 {
		status = 0
	}
	if req.GetGouhao() > 0 {
		resp, err := userpb.Info(frame.TODO(), &userpb.InfoReq{
			GouHao: req.GetGouhao(),
		})
		if err == nil && resp.GetErrcode() == 0 && resp.GetData() != nil {
			godID = resp.GetData().GetUserId()
		}
	}
	ret, err := gg.dao.QueryGodHistory(status, godID, req.GetOffset())
	if err != nil {
		return c.JSON2(ERR_CODE_INTERNAL, err.Error(), nil)
	}
	if len(ret) == 0 {
		return c.JSON2(StatusOK_V3, "", nil)
	}
	data := make([]map[string]interface{}, 0, len(ret))
	var godInfo *userpb.UserInfo
	var godLeader *model.GodLeader
	var tmpData map[string]interface{}
	for _, godHistory := range ret {
		var certsJSON []string
		godInfo, err = gg.getSimpleUser(godHistory.GodID)
		if err != nil || godInfo == nil {
			continue
		}
		if err = json.Unmarshal([]byte(godHistory.CertsDB), &certsJSON); err != nil || len(certsJSON) == 0 {
			continue
		}
		if godHistory.LeaderSwitch == constants.GOD_LEADER_SWITCH_OPEN {
			if godLeader = gg.dao.GetGodLeaderByID(godHistory.LeaderID); godLeader == nil {
				continue
			}
		}
		tmpData = map[string]interface{}{
			"god_id":        godHistory.GodID,
			"gouhao":        godInfo.GetGouhao(),
			"realname":      godHistory.RealName,
			"sex":           godHistory.Gender,
			"phone":         godHistory.Phone,
			"idcard":        godHistory.IDcard,
			"idcard_url":    GenIDCardURL(godHistory.IDcardurl, gg.cfg.OSS.OSSAccessID, gg.cfg.OSS.OSSAccessKey),
			"alipay":        godHistory.Alipay,
			"certs":         certsJSON,
			"createdtime":   FormatDatetime(godHistory.Createdtime),
			"status":        godHistory.Status,
			"leader_switch": godHistory.LeaderSwitch,
		}
		if godLeader != nil {
			tmpData["leader_info"] = godLeader
		}
		data = append(data, tmpData)
	}

	return c.JSON2(StatusOK_V3, "", map[string]interface{}{
		"items": data,
	})
}

// OM获取大神老资料
func (gg *GodGame) OldGodInfo(c frame.Context) error {
	var req godgamepb.OldGodInfoReq
	if err := c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	if req.GetGodId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "参数错误", nil)
	}
	god := gg.dao.GetGod(req.GetGodId())
	if god.UserID != req.GetGodId() {
		return c.JSON2(ERR_CODE_NOT_FOUND, "", nil)
	}
	godInfo, err := gg.getSimpleUser(god.UserID)
	if err != nil || godInfo == nil {
		return c.JSON2(ERR_CODE_INTERNAL, "大神信息获取失败", nil)
	}
	var godAlipay string
	accountResp, err := purse_pb.Account(frame.TODO(), &purse_pb.AccountReq{
		Mid: god.UserID,
	})
	if err == nil && accountResp.GetErrcode() == 0 {
		godAlipay = accountResp.GetData().GetWdAccount()
	}
	data := map[string]interface{}{
		"god_id":        god.UserID,
		"gouhao":        godInfo.GetGouhao(),
		"realname":      god.RealName,
		"sex":           god.Gender,
		"phone":         god.Phone,
		"idcard":        god.IDcard,
		"idcard_url":    GenIDCardURL(god.IDcardurl, gg.cfg.OSS.OSSAccessID, gg.cfg.OSS.OSSAccessKey),
		"alipay":        godAlipay,
		"leader_switch": god.LeaderSwitch,
		"createdtime":   FormatDatetime(god.Createdtime),
		"updatedtime":   FormatDatetime(god.Updatedtime),
	}
	var godLeader *model.GodLeader
	if god.LeaderSwitch == constants.GOD_LEADER_SWITCH_OPEN {
		godLeader = gg.dao.GetGodLeaderByID(god.LeaderID)
		if godLeader != nil {
			data["leader_info"] = godLeader
		}
	}
	return c.JSON2(StatusOK_V3, "", data)
}

// OM判断是否是大神
func (gg *GodGame) IsGod(c frame.Context) error {
	var req godgamepb.IsGodReq
	if err := c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	if len(req.GetUserids()) == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	var ret = make(map[int64]int64)
	var god model.God
	for _, uid := range req.GetUserids() {
		god = gg.dao.GetGod(uid)
		if god.UserID == uid {
			ret[uid] = god.Status
		} else {
			ret[uid] = constants.GOD_STATUS_UNAUTHED
		}
	}
	return c.JSON2(StatusOK_V3, "", ret)
}

// OM后台重置陪玩首页Feed流
func (gg *GodGame) ResetTimeLine(c frame.Context) error {
	var req godgamepb.ResetTimeLineReq
	if err := c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	if len(req.GetP()) == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "参数错误", nil)
	}
	err := gg.dao.ResetTimeLine(req.GetP())
	if err != nil {
		return c.JSON2(ERR_CODE_INTERNAL, err.Error(), nil)
	}
	return c.JSON2(StatusOK_V3, "", nil)
}

// OM后台批量获取指定游戏的一批大神陪玩信息
func (gg *GodGame) Batch(c frame.Context) error {
	var req godgamepb.BatchReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	if req.GetGameId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	if len(req.GetGodIds()) == 0 {
		return c.JSON2(StatusOK_V3, "", nil)
	} else if len(req.GetGodIds()) > 20 {
		req.GodIds = req.GodIds[:20]
	}

	var userInfo *userpb.UserInfo
	var freeResp *plorderpb.FreeResp
	var godInfo model.God
	var v1 model.GodGameV1
	var tmpImgs []string
	ret := make([]map[string]interface{}, 0, len(req.GodIds))
	for _, godID := range req.GodIds {
		userInfo, err = gg.getSimpleUser(godID)
		if err != nil || userInfo == nil || userInfo.GetInvalid() != userpb.USER_INVALID_NO {
			continue
		}
		godInfo = gg.dao.GetGod(godID)
		if godInfo.Status != constants.GOD_STATUS_PASSED {
			continue
		}
		v1, err = gg.dao.GetGodSpecialGameV1(godID, req.GetGameId())
		if err != nil {
			continue
		}
		err = json.Unmarshal([]byte(v1.Images), &tmpImgs)
		if err != nil || len(tmpImgs) == 0 {
			continue
		}
		freeResp, err = plorderpb.Free(frame.TODO(), &plorderpb.FreeReq{
			GodId: godID,
		})
		if err != nil || freeResp.GetErrcode() != 0 {
			continue
		}
		ret = append(ret, map[string]interface{}{
			"game_id":  req.GetGameId(),
			"god_id":   godID,
			"username": userInfo.GetUsername(),
			"gouhao":   userInfo.GetGouhao(),
			"img":      tmpImgs[0] + "/400",
			"status":   freeResp.GetData().GetStatus(),
		})
	}
	return c.JSON2(StatusOK_V3, "", ret)
}

func (gg *GodGame) GodDetail2(c frame.Context) error {
	var req godgamepb.GodDetailReq
	if err := c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	gameStateResp, err := gamepb.State(frame.TODO(), &gamepb.StateReq{
		GameId: req.GetGameId(),
	})
	if err == nil && gameStateResp.GetErrcode() == 0 {
		if gameStateResp.GetData().GetState() == game_const.GAME_STATE_NO {
			return c.JSON2(ERR_CODE_DISPLAY_ERROR, "品类已下架", nil)
		}
	}
	godInfo := gg.dao.GetGod(req.GetGodId())
	if godInfo.Status != constants.GOD_STATUS_PASSED {
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "大神状态异常", nil)
	}
	v1, err := gg.dao.GetGodSpecialGameV1(req.GetGodId(), req.GetGameId())
	if err != nil {
		return c.JSON2(ERR_CODE_NOT_FOUND, "", nil)
	}
	// // 大神关闭接单开关，则不返回陪玩信息
	// if v1.GrabSwitch == constants.GRAB_SWITCH_CLOSE {
	// 	return c.JSON2(ERR_CODE_DISPLAY_ERROR, "大神停止接单了", nil)
	// }
	userinfo, err := gg.getSimpleUser(v1.GodID)
	if err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	resp, err := gamepb.AcceptCfgV2(frame.TODO(), &gamepb.AcceptCfgV2Req{
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

	freeResp, err := plorderpb.Free(frame.TODO(), &plorderpb.FreeReq{
		GodId: v1.GodID,
	})
	if err != nil || freeResp.GetErrcode() != 0 {
		return c.JSON2(ERR_CODE_INTERNAL, "", nil)
	}

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

	data := map[string]interface{}{
		"god_id":             v1.GodID,
		"grab_switch":        v1.GrabSwitch,
		"god_icon":           v1.GodIcon,
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
		"god_tags":           tmpTags,
		"ext":                tmpExt,
		"desc":               v1.Desc,
		"uniprice":           uniprice,
		"gl":                 FormatRMB2Gouliang(uniprice),
		"order_cnt":          v1.AcceptNum,
		"order_cnt_desc":     FormatAcceptOrderNumber(v1.AcceptNum),
		"order_rate":         "100%",
		"regions":            v1.Regions,
		"levels":             v1.Levels,
		"score":              v1.Score,
		"score_desc":         FormatScore(v1.Score),
		"status":             freeResp.GetData().GetStatus(),
		"status_desc":        freeResp.GetData().GetStatusDesc(),
		"shareurl":           gg.GenPeiWanShareURL(tmpImages[0], userinfo.GetUsername(), "", v1.GodID, v1.GameID),
	}
	if v1.Video != "" {
		data["video"] = gg.formatVideoInfo2(c, v1.Video)
	}
	if v1.Videos != "" {
		var tmpVideos []string
		err = json.Unmarshal([]byte(v1.Videos), &tmpVideos)
		if err == nil && len(tmpVideos) > 0 {
			for idx, _ := range tmpVideos {
				tmpVideos[idx] = gg.formatVideoInfo2(c, tmpVideos[idx])
			}
			data["videos"] = tmpVideos
		}
	}
	orderRateResp, _ := sapb.GodAcceptOrderPer(frame.TODO(), &sapb.GodAcceptOrderPerReq{
		GodId:     v1.GodID,
		BeforeDay: 7,
	})
	if orderRateResp != nil && orderRateResp.GetData() > 0 {
		if orderRateResp.GetData() < 60 {
			data["order_rate"] = "60%"
		} else if orderRateResp.GetData() >= 60 {
			data["order_rate"] = fmt.Sprintf("%d%%", orderRateResp.GetData())
		}
	}

	commentData, _ := plcommentpb.GetGodGameComment(frame.TODO(), &plcommentpb.GetGodGameCommentReq{
		GodId:  req.GetGodId(),
		GameId: req.GetGameId(),
	})
	if commentData != nil && commentData.GetData() != nil {
		data["comments_cnt"] = commentData.GetData().GetCommentCnt()
		data["tags"] = commentData.GetData().GetTags()
	}
	hotComments, _ := plcommentpb.GetHotComments(frame.TODO(), &plcommentpb.GetHotCommentsReq{
		GodId:  req.GetGodId(),
		GameId: req.GetGameId(),
	})
	if hotComments != nil && len(hotComments.GetData()) > 0 {
		data["comments"] = hotComments.GetData()
	}
	return c.JSON2(StatusOK_V3, "", data)
}

func (gg *GodGame) WxInfo(c frame.Context) error {
	var req godgamepb.WxInfoReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "参数错误[1]", nil)
	} else if req.GetUserid() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "参数错误[2]", nil)
	} else if req.GetOpenid() == "" {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "参数错误[3]", nil)
	}
	godInfo := gg.dao.GetGod(req.GetUserid())
	godStatus := constants.GOD_STATUS_UNAUTHED
	if godInfo.UserID > 0 {
		godStatus = godInfo.Status
	}
	wxStatus := 3
	wxUsed := 2
	wxResp, err := userpb.WxBindInfo(c, &userpb.WxBindInfoReq{
		UserId: req.GetUserid(),
		OpenId: req.GetOpenid(),
	})
	if err == nil && wxResp.GetData() != nil {
		openID := wxResp.GetData().GetOpenid()
		if openID == req.GetOpenid() {
			wxStatus = 1
		} else if openID != "" && openID != req.GetOpenid() {
			wxStatus = 2
		}
		userID := wxResp.GetData().GetUserid()
		if userID != 0 && userID != req.GetUserid() {
			wxUsed = 1
		}
	}
	data := &godgamepb.WxInfoResp_Data{
		GodStatus: godStatus,
		WxStatus:  int64(wxStatus),
		WxUsed:    int64(wxUsed),
	}
	return c.JSON2(StatusOK_V3, "", data)
}

// OM新增大神认证标签
func (gg *GodGame) AddIcon(c frame.Context) error {
	var req godgamepb.AddIconReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	if req.GetName() == "" {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "标签名不能为空", nil)
	} else if req.GetUrl() == "" {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "标签图片不能为空", nil)
	}
	godIcon, err := gg.dao.AddGodIcon(model.GodIcon{
		Name: req.GetName(),
		Url:  req.GetUrl(),
	})
	if err != nil || godIcon == nil {
		c.Errorf("AddIcon error %s", err)
		return c.JSON2(ERR_CODE_INTERNAL, "内部错误[1]", nil)
	}
	return c.JSON2(StatusOK_V3, "", &godgamepb.GodIcon{
		Id:          godIcon.ID,
		Name:        godIcon.Name,
		Url:         godIcon.Url,
		Createdtime: lyg_util.PrettyDateV3(godIcon.Createdtime),
		Status:      godIcon.Status,
	})
}

// OM修改大神认证标签
func (gg *GodGame) ModifyIcon(c frame.Context) error {
	var req godgamepb.ModifyIconReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	if req.GetId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "参数错误[1]", nil)
	} else if req.GetName() == "" {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "标签名称不能为空", nil)
	} else if req.GetUrl() == "" {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "标签图片不能为空", nil)
	}
	if req.GetStatus() == 1 {
		// 停用
		err = gg.dao.DisableGodIcon(req.GetId())
		if err != nil {
			c.Errorf("ModifyIcon error %s", err)
			return c.JSON2(ERR_CODE_INTERNAL, "标签停用失败", nil)
		}
	}
	godIcon, err := gg.dao.ModifyGodIcon(model.GodIcon{
		ID:   req.GetId(),
		Name: req.GetName(),
		Url:  req.GetUrl(),
	})
	if err != nil || godIcon == nil {
		c.Errorf("ModifyIcon error %s", err)
		return c.JSON2(ERR_CODE_INTERNAL, "内部错误[1]", nil)
	}

	return c.JSON2(StatusOK_V3, "", &godgamepb.GodIcon{
		Id:          godIcon.ID,
		Name:        godIcon.Name,
		Url:         godIcon.Url,
		Createdtime: lyg_util.PrettyDateV3(godIcon.Createdtime),
		Status:      godIcon.Status,
	})
}

// OM获取大神认证标签列表
func (gg *GodGame) Icons(c frame.Context) error {
	var req godgamepb.IconsReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	if req.GetPage() <= 0 {
		req.Page = 1
	}
	items, err := gg.dao.GetGodIconList(req.GetPage())
	if err != nil {
		c.Errorf("Icons error %s", err)
		return c.JSON2(ERR_CODE_INTERNAL, "内部错误[1]", nil)
	}
	godIcons := make([]*godgamepb.GodIcon, 0, len(items))
	for _, item := range items {
		godIcons = append(godIcons, &godgamepb.GodIcon{
			Id:          item.ID,
			Name:        item.Name,
			Url:         item.Url,
			Createdtime: lyg_util.PrettyDateV3(item.Createdtime),
			Status:      item.Status,
		})
	}
	return c.JSON2(StatusOK_V3, "", &godgamepb.IconsResp_Data{
		Count: int64(len(godIcons)),
		Items: godIcons,
	})
}

// OM给大神配置定时标签
func (gg *GodGame) SetGodIcon(c frame.Context) error {
	var req godgamepb.SetGodIconReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	} else if req.GetIconId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "标签ID不能为空", nil)
	} else if req.GetBegin() == 0 || req.GetEnd() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "请配置展示时间段", nil)
	} else if req.GetBegin() >= req.GetEnd() {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "展示结束时间必须大于开始时间", nil)
	} else if time.Now().Unix() >= req.GetEnd() {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "展示结束时间必须大于当前时间", nil)
	}
	if god := gg.dao.GetGod(req.GetGodId()); god.Status != constants.GOD_STATUS_PASSED {
		return c.JSON2(ERR_CODE_FORBIDDEN, "大神身份未通过审核", nil)
	}
	if err = gg.dao.SetGodIcon(req.GetGodId(), req.GetIconId(), req.GetBegin(), req.GetEnd()); err != nil {
		c.Errorf("SetGodIcon error %s", err)
		return c.JSON2(ERR_CODE_INTERNAL, "内部错误[1]", nil)
	}
	return c.JSON2(StatusOK_V3, "", nil)
}

// OM后台停用大神标签
func (gg *GodGame) DisableGodIcon(c frame.Context) error {
	var req godgamepb.DisableGodIconReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	gg.dao.RemoveGodIcon(req.GetGodId())
	return c.JSON2(StatusOK_V3, "", nil)
}

// 获取附近的大神
func (gg *GodGame) NearbyGods(c frame.Context) error {
	var req godgamepb.NearbyGodsReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	} else if req.GetLat() == 0 || req.GetLng() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "参数错误[1]", nil)
	} else if req.GetDistance() == "" {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "参数错误[3]", nil)
	}
	currentPoint, err := geo.NewLatLng(strconv.FormatFloat(req.GetLat(), 'E', -1, 64), strconv.FormatFloat(req.GetLng(), 'E', -1, 64))
	if err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "参数错误[3]", nil)
	}
	limit := 20
	offset := (int(req.GetPage()-1) * limit)
	gender := constants.GENDER_UNKNOW
	if req.GetGender() == constants.GENDER_FEMALE {
		gender = constants.GENDER_MALE
	} else if req.GetGender() == constants.GENDER_MALE {
		gender = constants.GENDER_FEMALE
	}
	q := elastic.NewBoolQuery().Must(elastic.NewRangeQuery("lts").Gte(time.Now().AddDate(0, 0, -3)))
	if gender != constants.GENDER_UNKNOW {
		q = q.Must(elastic.NewTermQuery("gender", gender))
	}
	if req.GetCity() != "" {
		// 查询同省
		q = q.Must(elastic.NewTermQuery("city.keyword", req.GetCity()))
	} else {
		// 查询附近xxx公里内
		query := elastic.NewGeoDistanceQuery("location").
			GeoPoint(elastic.GeoPointFromLatLon(req.GetLat(), req.GetLng())).
			Distance(req.GetDistance())
		q = q.Filter(query)
	}
	fsc := elastic.NewFetchSourceContext(true).Include("god_id", "location")
	searchSource := elastic.NewSearchSource().Query(q).FetchSourceContext(fsc)
	if src, err := searchSource.Source(); err == nil {
		if bs, err := json.Marshal(src); err == nil {
			c.Infof("### src %s", bs)
		}
	}
	// agg := elastic.NewTermsAggregation().Field("god_id")
	resp, err := elastic.NewSearchService(gg.esClient).Index(gg.cfg.ES.PWIndex).
		Query(q).SearchSource(searchSource).Sort("lts", false).From(offset).Size(limit).
		// Aggregation("agg_god", agg).
		Do(context.Background())
	if err != nil {
		c.Warnf("%s", err)
		return c.JSON2(ERR_CODE_INTERNAL, "内部错误[1]", nil)
	}
	// var ar elastic.AggregationBucketKeyItems
	// err = json.Unmarshal(*resp.Aggregations["agg_god"], &ar)
	// if err != nil {
	// 	c.Warnf("%s", err)
	// 	return c.JSON2(ERR_CODE_INTERNAL, "内部错误[2]", nil)
	// }

	items := make(map[string]string)
	var pwObj model.ESGodGame
	var godPoint *geo.LatLng
	var tmpGodID string
	for _, hit := range resp.Hits.Hits {
		if err = json.Unmarshal(*hit.Source, &pwObj); err != nil {
			continue
		} else if pwObj.Location == nil {
			continue
		}
		tmpGodID = strconv.FormatInt(pwObj.GodID, 10)
		if _, ok := items[tmpGodID]; ok {
			continue
		}
		if godPoint, err = geo.NewLatLng(strconv.FormatFloat(pwObj.Location.Lat, 'E', -1, 64),
			strconv.FormatFloat(pwObj.Location.Lon, 'E', -1, 64)); err == nil && godPoint != nil {
			items[tmpGodID] = fmt.Sprintf("%.1fkm", Round(currentPoint.CalcDistance(godPoint), 1, false))
		}
		// for _, bucket := range ar.Buckets {
		// 	if gid, _ := bucket.KeyNumber.Int64(); gid == pwObj.GodID {
		// 		if godPoint, err = geo.NewLatLng(strconv.FormatFloat(pwObj.Location.Lat, 'E', -1, 64),
		// 			strconv.FormatFloat(pwObj.Location.Lon, 'E', -1, 64)); err == nil && godPoint != nil {
		// 			items[tmpGodID] = fmt.Sprintf("%.1fkm", Round(currentPoint.CalcDistance(godPoint), 1, false))
		// 			break
		// 		}
		// 	}
		// }
	}
	return c.JSON2(StatusOK_V3, "", items)
}

func (gg *GodGame) DropGodCache(c frame.Context) error {
	var req godgamepb.DropGodCacheReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.RetBadRequestError(err.Error())
	}
	gg.dao.DropGodCache(req.GetGodId())
	return c.RetSuccess("", nil)
}
