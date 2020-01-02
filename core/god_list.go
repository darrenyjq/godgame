package core

import (
	"fmt"
	"iceberg/frame"

	"github.com/gomodule/redigo/redis"
	lyg_fmt "laoyuegou.com/util/format"
	pb_chatroom "laoyuegou.pb/chatroom/pb"
	gamepb "laoyuegou.pb/game/pb"
	"laoyuegou.pb/godgame/constants"
	"laoyuegou.pb/godgame/model"
	lfspb "laoyuegou.pb/lfs/pb"
	livepb "laoyuegou.pb/live/pb"
	plcommentpb "laoyuegou.pb/plcomment/pb"
	order_const "laoyuegou.pb/plorder/constants"
	plorderpb "laoyuegou.pb/plorder/pb"
	userpb "laoyuegou.pb/user/pb"
)

func (dao *Dao) formatVideoInfo(c frame.Context, hash string) string {
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

func (dao *Dao) GenPeiWanShareURL(godAvatar, godName, gameName, desc string, godID, gameID int64) string {
	var h5URL string
	title := fmt.Sprintf("#%s# %s", gameName, godName)
	subTitle := desc

	if dao.Cfg.Env.Production() {
		h5URL = fmt.Sprintf("https://imgx.lygou.cc/tang/dist/pages/god/?user_id=%d&gameid=%d", godID, gameID)
	} else if dao.Cfg.Env.QA() {
		h5URL = fmt.Sprintf("https://guest-test-imgx.lygou.cc/tang/dist/pages/god/?user_id=%d&gameid=%d", godID, gameID)
	} else if dao.Cfg.Env.Stag() {
		h5URL = fmt.Sprintf("https://guest-staging-imgx.lygou.cc/tang/dist/pages/god/?user_id=%d&gameid=%d", godID, gameID)
	}
	if subTitle == "" {
		subTitle = h5URL
	}
	rawString := fmt.Sprintf("laoyuegou://share?title=%s&&share_url=%s&&share_content=%s&&imageurl=%s&&platform=0&&imageurl_sina=%s&&type=60000001&&god_id=%d&&game_id=%d",
		title, h5URL, subTitle, godAvatar, godAvatar, godID, gameID)
	return rawString
}

// GetGodListsByGender 按性别分页获取品类大神列表
func (dao *Dao) GetGodListsByGender(gameID, gender, offset, limit int64, ctx frame.Context) ([]map[string]interface{}, int64) {
	redisKey := RKGodListByGender(gameID, gender)
	c := dao.Cpool.Get()
	defer c.Close()
	totalCnt, _ := redis.Int64(c.Do("ZCARD", redisKey))
	if totalCnt == 0 {
		return []map[string]interface{}{}, 0
	}
	gids, _ := redis.Int64s(c.Do("ZREVRANGE", redisKey, offset, offset+limit-1))
	if len(gids) == 0 {
		return []map[string]interface{}{}, totalCnt
	}
	var gameName string
	if record, err := gamepb.Record(ctx, &gamepb.RecordReq{GameId: gameID}); err == nil && record.GetErrcode() == 0 {
		gameName = record.GetData().GetGameName()
	}

	gods := make([]map[string]interface{}, 0, len(gids))
	resp, err := gamepb.AcceptCfgV2(ctx, &gamepb.AcceptCfgV2Req{
		GameId: gameID,
	})
	if err != nil || resp.GetErrcode() != 0 {
		return gods, totalCnt
	}
	batch, err := userpb.Batch(ctx, &userpb.BatchReq{
		Uids: gids,
	})
	if err != nil || batch.GetErrcode() != 0 || len(batch.GetData()) == 0 {
		return gods, totalCnt
	}
	var v1 model.GodGameV1
	var godInfo model.God
	var data map[string]interface{}
	var uniprice int64
	var regionDesc string
	var freeStatus int64
	var freeStatusDesc string
	var roomID int64
	var template int32

	for _, user := range batch.GetData() {
		godInfo = dao.GetGod(user.GetUserId())
		if godInfo.Status != constants.GOD_STATUS_PASSED {
			continue
		}
		v1, err = dao.GetGodSpecialGameV1(user.GetUserId(), gameID)
		if err != nil {
			continue
		}
		data = make(map[string]interface{})
		if v1.PriceType == constants.PW_PRICE_TYPE_BY_OM {
			uniprice = v1.PeiWanPrice
		} else {
			uniprice = resp.GetData().GetPrices()[v1.PriceID]
		}
		if len(v1.Regions) > 0 {
			for _, region := range v1.Regions {
				regionDesc = regionDesc + resp.GetData().GetRegionDesc()[region] + ","
			}
			regionDesc = regionDesc[:len(regionDesc)-1]
		}

		liveResp, err := livepb.GetGodLiveId(ctx, &livepb.GetGodLiveIdReq{
			GodId:  v1.GodID,
			GameId: v1.GameID,
		})
		if err == nil && liveResp.GetData() != nil && liveResp.GetData().GetRoomId() > 0 {
			// 优先返回直播
			freeStatus = order_const.PW_STATUS_LIVE
			freeStatusDesc = order_const.PW_STATS_DESC[order_const.PW_STATUS_LIVE]
			roomID = liveResp.GetData().GetRoomId()
		} else {
			freeResp, err := plorderpb.Free(ctx, &plorderpb.FreeReq{
				GodId: v1.GodID,
			})
			if err != nil || freeResp.GetErrcode() != 0 {
				freeStatus = order_const.PW_STATUS_FREE
				freeStatusDesc = order_const.PW_STATS_DESC[order_const.PW_STATUS_FREE]
			} else {
				freeStatus = freeResp.GetData().GetStatus()
				freeStatusDesc = freeResp.GetData().GetStatusDesc()
				if freeStatus == order_const.PW_STATUS_FREE {
					seatResp, err := pb_chatroom.IsOnSeat(ctx, &pb_chatroom.IsOnSeatReq{
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

		data = map[string]interface{}{
			"god_id":             v1.GodID,
			"god_name":           user.GetUsername(),
			"god_avatar":         user.GetAvatar(),
			"sex":                godInfo.Gender,
			"age":                user.GetAge(),
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
			"gl":                 lyg_fmt.FormatRMB2Gouliang(uniprice),
			"order_cnt":          v1.AcceptNum,
			"order_cnt_desc":     lyg_fmt.FormatAcceptOrderNumber3(v1.AcceptNum),
			"order_rate":         "100%",
			"regions":            v1.Regions,
			"levels":             v1.Levels,
			"score":              v1.Score,
			"score_desc":         lyg_fmt.FormatScore(v1.Score),
			"status":             freeStatus,
			"status_desc":        freeStatusDesc,
			"room_id":            roomID,
			"template":           template,
			"shareurl":           dao.GenPeiWanShareURL(user.GetAvatar(), user.GetUsername(), gameName, v1.Desc, v1.GodID, v1.GameID),
		}
		if v1.Video != "" {
			tmpStr := dao.formatVideoInfo(ctx, v1.Video)
			data["video"] = tmpStr
			data["videos"] = []string{tmpStr}
		}
		if v1.Videos != "" {
			var tmpVideos []string
			err = json.Unmarshal([]byte(v1.Videos), &tmpVideos)
			if err == nil && len(tmpVideos) > 0 {
				for idx, _ := range tmpVideos {
					tmpVideos[idx] = dao.formatVideoInfo(ctx, tmpVideos[idx])
				}
				data["videos"] = tmpVideos
			}
		}
		if orderPercent, err := plorderpb.OrderFinishPercent(ctx, &plorderpb.OrderFinishPercentReq{
			GodId: v1.GodID,
			Days:  7,
		}); err == nil && orderPercent.GetErrcode() == 0 {
			data["order_rate"] = orderPercent.GetData()
		}
		if commentData, err := plcommentpb.GodPageComment(ctx, &plcommentpb.GodPageCommentReq{
			GodId:  v1.GodID,
			GameId: gameID,
		}); err == nil && commentData.GetErrcode() == 0 {
			data["comments_cnt"] = commentData.GetData().GetCommentCount()
			data["tags"] = commentData.GetData().GetTags()
			data["comments"] = commentData.GetData().GetHotComments()
		}
		gods = append(gods, data)
	}

	return gods, totalCnt
}

//GetInvialdGod 获取所有审核通过的大神
func (dao *Dao) GetInvialdGod() (results []*model.God, err error) {
	if err := dao.dbr.Model(&model.God{}).Where("status = ?", 1).Scan(&results).Error; err != nil {
		return nil, err
	} else if len(results) == 0 {
		return nil, fmt.Errorf("暂无审核通过的大神")
	}
	return results, err
}
