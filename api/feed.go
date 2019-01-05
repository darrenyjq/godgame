package api

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	"godgame/core"
	"iceberg/frame"
	"laoyuegou.com/util"
	pb_chatroom "laoyuegou.pb/chatroom/pb"
	"laoyuegou.pb/game/pb"
	"laoyuegou.pb/godgame/constants"
	"laoyuegou.pb/godgame/model"
	"laoyuegou.pb/godgame/pb"
	"laoyuegou.pb/plorder/pb"
	"sort"
	"time"
)

// 陪玩首页Feed流
func (gg *GodGame) Feeds2(c frame.Context) error {
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
	currentUser := gg.getCurrentUser(c)
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

func (gg *GodGame) Feeds(c frame.Context) error {
	begin := time.Now()
	currentUser := gg.getCurrentUser(c)
	var resp godgamepb.FeedsResp_IndexFeedRespData
	resp.P = -1
	resp.Feeds = make([]*godgamepb.FeedsResp_IndexFeedRespData_FeedObj, 0, 25)
	feeds, err := gg.dao.GetTimeLine()
	if err == nil && len(feeds) > 0 {
		for _, feed := range feeds {
			if feed.GetTy() == constants.FEED_TYPE_ROOM {
				if rooms, err := gg.getFeedRooms(c); err == nil {
					feed.Body = rooms
				} else {
					c.Errorf("%s", err.Error())
				}
			} else if feed.GetTy() == constants.FEED_TYPE_GOD {
				if gods, err := gg.getFeedRecommendGods(feed.GetTi(), feed.GetBody(), c); err == nil {
					feed.Body = gods
				} else {
					c.Errorf("%s", err.Error())
				}
			}
			resp.Feeds = append(resp.Feeds, feed)
		}
	} else {
		c.Error(err.Error())
	}
	games, err := gamepb.List(c, &gamepb.GamesReq{})
	if err == nil && games.GetErrcode() == 0 {
		for _, game := range games.GetData() {
			gods, err := gg.getFeedGods(game.GetGameId(), currentUser)
			if err != nil {
				c.Error(err.Error())
				continue
			} else {
				gods.Ti = game.GetGameName()
			}
			resp.Feeds = append(resp.Feeds, gods)
		}
	} else {
		c.Error(err.Error())
	}
	return c.JSON2(StatusOK_V3, "", resp)
}

func (gg *GodGame) getFeedRecommendGods(title, body string, ctx frame.Context) (string, error) {
	c := gg.dao.GetRedisPool().Get()
	defer c.Close()
	redisKey := core.RKFeedGods(util.MD5Sum([]byte(title)))
	ret, err := redis.String(c.Do("GET", redisKey))
	if err == nil {
		return ret, nil
	}
	var gods []model.FeedGod
	err = json.Unmarshal([]byte(body), &gods)
	if err != nil {
		return "", err
	}
	if len(gods) == 0 {
		return "", nil
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
		fResp, err := plorderpb.Free(ctx, &plorderpb.FreeReq{
			GodId: god.GodID,
		})
		if err == nil && fResp.GetErrcode() == 0 {
			god.Free = fResp.GetData().GetStatus()
		}
		gods2 = append(gods2, god)
	}
	if len(gods2) == 0 {
		return "", nil
	}
	sort.Sort(model.SortFeedGod(gods2))
	bs, err := json.Marshal(gods2)
	if err != nil {
		return "", err
	}
	c.Do("SET", redisKey, string(bs), "EX", 60)
	return string(bs), nil
}

func (gg *GodGame) getFeedRooms(ctx frame.Context) (string, error) {
	c := gg.dao.GetRedisPool().Get()
	defer c.Close()
	rooms, err := redis.String(c.Do("GET", core.RKFeedRooms()))
	if err == nil {
		return rooms, nil
	}
	resp, err := pb_chatroom.Hot(ctx, &pb_chatroom.Omit{})
	if err != nil {
		return "", err
	} else if resp.GetErrcode() != 0 {
		return "", fmt.Errorf("%s", resp.GetErrmsg())
	}
	bs, err := json.Marshal(resp.GetData())
	if err != nil {
		return "", err
	}
	c.Do("SET", core.RKFeedRooms(), string(bs), "EX", 60)
	return string(bs), nil
}

func (gg *GodGame) getFeedGods(gameID int64, currentUser model.CurrentUser) (*godgamepb.FeedsResp_IndexFeedRespData_FeedObj, error) {
	c := gg.dao.GetRedisPool().Get()
	defer c.Close()
	if currentUser.Gender != constants.GENDER_MALE && currentUser.Gender != constants.GENDER_FEMALE {
		currentUser.Gender = constants.GENDER_UNKNOW
	}
	var gods *godgamepb.FeedsResp_IndexFeedRespData_FeedObj
	bs, err := redis.Bytes(c.Do("GET", core.RKFeedGodsByGender(gameID, currentUser.Gender)))
	if err == nil {
		err = json.Unmarshal(bs, &gods)
		if err == nil {
			return gods, nil
		}
	}
	pwObjs, _ := gg.queryGods(godgamepb.GodListReq{
		GameId: gameID,
		Offset: 0,
		Limit:  8,
		Type:   constants.SORT_TYPE_DEFAULT,
	}, currentUser)
	tmpGods := gg.getGodItems(pwObjs)
	if len(tmpGods) > 4 {
		bs, err = json.Marshal(map[string]interface{}{
			"game_id": gameID,
			"gods":    tmpGods[0:4],
		})
		if err != nil {
			return nil, err
		}
		gods = &godgamepb.FeedsResp_IndexFeedRespData_FeedObj{
			Ty:   constants.FEED_TYPE_GAME,
			Body: string(bs),
		}
		if bs, err = json.Marshal(gods); err == nil {
			c.Do("SET", core.RKFeedGodsByGender(gameID, currentUser.Gender), string(bs), "EX", 60)
		}
		return gods, nil
	}
	return nil, fmt.Errorf("大神数据获取失败")
}
