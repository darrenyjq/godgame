package api

import (
	"encoding/json"
	"iceberg/frame"
	pb_chatroom "laoyuegou.pb/chatroom/pb"
	game_const "laoyuegou.pb/game/constants"
	"laoyuegou.pb/game/pb"
	"laoyuegou.pb/godgame/constants"
	"laoyuegou.pb/godgame/model"
	"laoyuegou.pb/godgame/pb"
	"laoyuegou.pb/plorder/pb"
	"sort"
)

// 陪玩首页Feed流
func (gg *GodGame) Feeds(c frame.Context) error {
	var req godgamepb.FeedsReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	var resp godgamepb.FeedsResp_IndexFeedRespData
	var feeds []*godgamepb.FeedsResp_IndexFeedRespData_FeedObj
	resp.P = -1
	if req.GetP() < 0 {
		return c.JSON2(StatusOK_V3, "", nil)
	} else if req.GetP() == 0 {
		feeds, err = gg.dao.GetTimeLine()
		if err != nil {
			return c.JSON2(ERR_CODE_INTERNAL, "", nil)
		}
	}
	tmpResp, err := gamepb.List(frame.TODO(), &gamepb.GamesReq{})
	if err != nil || tmpResp == nil {
		return c.JSON2(ERR_CODE_INTERNAL, "", nil)
	}
	games := tmpResp.GetData()
	var game *gamepb.GamesData
	var ok bool
	var myGameID int64
	currentUser := gg.getCurrentUser(c)
	if len(currentUser.GameIds) > 0 {
		tmpMap := make(map[int64]byte)
		for _, userGame := range currentUser.GameIds {
			if myGameID, ok = game_const.GameDicst[userGame]; ok {
				tmpMap[myGameID] = 1
			}
		}
		var arr1, arr2 = make([]*gamepb.GamesData, 0, len(games)), make([]*gamepb.GamesData, 0, len(games))
		for _, game = range games {
			if _, ok = tmpMap[game.GetGameId()]; ok {
				arr1 = append(arr1, game)
			} else {
				arr2 = append(arr2, game)
			}
		}
		games = append(arr1, arr2...)
	}
	gameLen := len(games)
	if len(feeds) == 0 {
		// 获取游戏品类
		args := godgamepb.GodListReq{
			Offset: 0,
			Limit:  8,
			Type:   constants.SORT_TYPE_DEFAULT,
		}
		var pwObjs []model.ESGodGame
		var gods []map[string]interface{}
		var bs []byte
		for _, game = range games[gameLen/2 : gameLen] {
			args.GameId = game.GetGameId()
			pwObjs, _ = gg.queryGods(args, currentUser)
			gods = gg.getGodItems(pwObjs)
			if len(gods) > 4 {
				bs, err = json.Marshal(map[string]interface{}{
					"game_id": game.GetGameId(),
					"gods":    gods[0:4],
				})
				if err != nil {
					continue
				}
				resp.Feeds = append(resp.Feeds, &godgamepb.FeedsResp_IndexFeedRespData_FeedObj{
					Ty:   constants.FEED_TYPE_GAME,
					Ti:   game.GetGameName(),
					Body: string(bs),
				})
			}
		}

		resp.P = -1
	} else {
		resp.P = 100
		for _, feed := range feeds {
			if feed == nil {
				continue
			}
			if feed.GetTy() == constants.FEED_TYPE_ROOM {
				resp2, err := pb_chatroom.Hot(frame.TODO(), &pb_chatroom.Omit{})
				if err != nil || resp2 == nil || len(resp2.GetData()) == 0 {
					continue
				}
				bs, err := json.Marshal(resp2.GetData())
				if err != nil {
					continue
				}
				feed.Body = string(bs)
			} else if feed.GetTy() == constants.FEED_TYPE_GOD {
				var gods []model.FeedGod
				err = json.Unmarshal([]byte(feed.GetBody()), &gods)
				if err != nil || len(gods) == 0 {
					continue
				}
				gods2 := make([]model.FeedGod, 0, len(gods))
				for _, god := range gods {
					if user, err := gg.getSimpleUser(god.GodID); err == nil {
						god.GodName = user.GetUsername()
					} else {
						continue
					}
					godInfo := gg.dao.GetGod(god.GodID)
					if godInfo.Status == constants.GOD_STATUS_PASSED {
						god.Gender = godInfo.Gender
					}
					if v1, err := gg.dao.GetGodSpecialGameV1(god.GodID, god.GameID); err == nil {
						if v1.GrabSwitch != constants.GRAB_SWITCH_OPEN {
							// 关闭接单开关不显示
							continue
						}
						var imgs []string
						err = json.Unmarshal([]byte(v1.Images), &imgs)
						if len(imgs) > 0 {
							god.Img = imgs[0] + "/400"
						} else {
							continue
						}
						god.GodIcon = v1.GodIcon
					}
					fResp, err := plorderpb.Free(c, &plorderpb.FreeReq{
						GodId: god.GodID,
					})
					if err != nil {
						c.Error(err.Error())
					} else if fResp.GetErrcode() != 0 {
						c.Error(fResp.GetErrmsg())
					} else {
						god.Free = fResp.GetData().GetStatus()
					}
					gods2 = append(gods2, god)
				}
				if len(gods2) > 0 {
					sort.Sort(model.SortFeedGod(gods2))
					if bs, err := json.Marshal(gods2); err == nil {
						feed.Body = string(bs)
					}
				}
			}
			resp.Feeds = append(resp.Feeds, feed)
		}
		args := godgamepb.GodListReq{
			Offset: 0,
			Limit:  8,
			Type:   constants.SORT_TYPE_DEFAULT,
		}
		var pwObjs []model.ESGodGame
		var gods []map[string]interface{}
		var bs []byte
		for _, game = range games[0 : gameLen/2] {
			args.GameId = game.GetGameId()
			pwObjs, _ = gg.queryGods(args, currentUser)
			gods = gg.getGodItems(pwObjs)
			if len(gods) > 4 {
				bs, err = json.Marshal(map[string]interface{}{
					"game_id": game.GetGameId(),
					"gods":    gods[0:4],
				})
				if err != nil {
					continue
				}
				resp.Feeds = append(resp.Feeds, &godgamepb.FeedsResp_IndexFeedRespData_FeedObj{
					Ty:   constants.FEED_TYPE_GAME,
					Ti:   game.GetGameName(),
					Body: string(bs),
				})
			}
		}
	}

	return c.JSON2(StatusOK_V3, "", resp)
}
