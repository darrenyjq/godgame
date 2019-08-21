package core

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	"github.com/jinzhu/gorm"
	"iceberg/frame"
	"iceberg/frame/icelog"
	log "iceberg/frame/icelog"
	game_const "laoyuegou.pb/game/constants"
	"laoyuegou.pb/game/pb"
	"laoyuegou.pb/godgame/constants"
	"laoyuegou.pb/godgame/model"
	"laoyuegou.pb/godgame/pb"
	plcommentpb "laoyuegou.pb/plcomment/pb"
	"laoyuegou.pb/plorder/pb"
	purse_pb "laoyuegou.pb/purse/pb"
	"math/rand"
	"regexp"
	"sort"
	"time"
)

func (dao *Dao) GetGodListCache(gameID, gender int64) ([]map[string]interface{}, int64) {

	key := fmt.Sprintf("G:{%d}:{%d}:GodList", gameID, gender)
	totalHitKey := fmt.Sprintf("G:{%d}:{%d}:GodListHit", gameID, gender)
	var result []map[string]interface{}
	var hits int64
	c := dao.cpool.Get()
	defer c.Close()
	if exists, _ := redis.Bool(c.Do("EXISTS", key)); exists {
		if bs, err := redis.Bytes(c.Do("GET", key)); err == nil {
			if err = json.Unmarshal(bs, &result); err == nil {
				hits, _ = redis.Int64(c.Do("GET", totalHitKey))
				return result, hits
			}
		}
	}
	return result, hits
}

func (dao *Dao) SaveGodListCache(gameID, gender, hits int64, result []map[string]interface{}) {
	key := fmt.Sprintf("G:{%d}:{%d}:GodList", gameID, gender)
	totalHitKey := fmt.Sprintf("G:{%d}:{%d}:GodListHit", gameID, gender)
	bs, err := json.Marshal(result)
	if err != nil {
		return
	}
	c := dao.cpool.Get()
	defer c.Close()
	c.Do("SET", key, string(bs), "EX", 10)
	c.Do("SET", totalHitKey, hits, "EX", 10)
}

// 获取一键呼叫语聊大神，随机开关打开状态的
func (dao *Dao) GetRandCallGods() ([]int64, error) {
	c := dao.cpool.Get()
	defer c.Close()
	gods, err := redis.Int64s(c.Do("ZRANGEBYSCORE", RKVoiceCallGods(), 1, 1))
	if err != nil {
		return []int64{}, err
	} else if len(gods) == 0 {
		return []int64{}, nil
	}
	return gods, nil
}

// 获取语聊大神，接单开关打开状态的
func (dao *Dao) GetRandCallGods2(start, stop int) ([]int64, error) {
	c := dao.cpool.Get()
	defer c.Close()
	gods, err := redis.Int64s(c.Do("ZRANGE", RKVoiceCallGods(), start, stop))
	if err != nil {
		return []int64{}, err
	} else if len(gods) == 0 {
		return []int64{}, nil
	}
	return gods, nil
}

// 获取申请大神短信验证码
func (dao *Dao) SendApplyCode(phone string) error {
	redisKey := RKAuthCodeForPhone(phone)
	c := dao.cpool.Get()
	defer c.Close()
	authCode := rand.Intn(900000) + 100000
	if ret, _ := redis.String(c.Do("SET", redisKey, authCode, "NX", "EX", 120)); ret != "OK" {
		return fmt.Errorf("请稍后再试")
	}
	txt := fmt.Sprintf("【捞月狗】%d您申请陪玩大神的验证码。", authCode)
	if !regexp.MustCompile("^(\\+86|\\+852|\\+853|\\+886)?\\d{1,11}").MatchString(phone) {
		txt = fmt.Sprintf("【laoyuegou】%dYou apply for the verification code of the game master.", authCode)
	}
	return dao.ypClient.Send(phone, txt)
}

func (dao *Dao) CheckApplyCode(code, phone string) bool {
	c := dao.cpool.Get()
	defer c.Close()
	authCode, _ := redis.String(c.Do("GET", RKAuthCodeForPhone(phone)))
	return code == authCode
}

// 获取大神修改自定义介绍的时间
func (dao *Dao) GetGodLastModifyDescTimestamp(godID int64) int64 {
	c := dao.cpool.Get()
	defer c.Close()
	ts, _ := redis.Int64(c.Do("GET", RKGodLastModifyDesc(godID)))
	return ts
}

// 判断用户是否为大神（已通过、被冻结都算大神）
func (dao *Dao) IsGod(userID int64) bool {
	god := dao.GetGod(userID)
	return god.UserID > 0
}

// IsGod2 判断是否为已通过的大神，数据可能有延迟，用于Feed流判断是否为大神准确度要求不高的场景使用
func (dao *Dao) IsGod2(userID int64) bool {
	key := RKGodInfo(userID)
	var god model.God
	var err error
	c := dao.cpool.Get()
	defer c.Close()
	bs, _ := redis.Bytes(c.Do("GET", key))
	if len(bs) == 1 {
		return false
	} else if len(bs) > 1 {
		err = json.Unmarshal(bs, &god)
		if err == nil && god.Status == constants.GOD_STATUS_PASSED {
			return true
		}
	}
	err = dao.dbr.Where("userid=?", userID).First(&god).Error
	if err != nil {
		c.Do("SET", key, "0", "EX", 1800)
		return false
	}
	bs, _ = json.Marshal(god)
	c.Do("SET", RKGodInfo(userID), string(bs))
	return god.Status == constants.GOD_STATUS_PASSED
}

func (dao *Dao) GetGod(userID int64) model.God {
	var god model.God
	c := dao.cpool.Get()
	defer c.Close()
	bs, _ := redis.Bytes(c.Do("GET", RKGodInfo(userID)))
	err := json.Unmarshal(bs, &god)
	if err == nil {
		return god
	}
	err = dao.dbr.Where("userid=?", userID).First(&god).Error
	if err != nil {
		return god
	}
	bs, _ = json.Marshal(god)
	c.Do("SET", RKGodInfo(userID), string(bs))
	return god
}

func (dao *Dao) DropGodCache(userID int64) {
	c := dao.cpool.Get()
	defer c.Close()
	c.Do("DEL", RKGodInfo(userID))
}

func (dao *Dao) GetGodApply(userID int64) model.GodApply {
	var godApply model.GodApply
	dao.dbr.Where("userid=?", userID).First(&godApply)
	return godApply
}

func (dao *Dao) GetGodGame(godID, gameID int64) model.GodGame {
	var godGame model.GodGame
	c := dao.cpool.Get()
	defer c.Close()
	bs, _ := redis.Bytes(c.Do("GET", RKGodGameInfo(godID, gameID)))
	err := json.Unmarshal(bs, &godGame)
	if err == nil {
		return godGame
	}
	err = dao.dbr.Where("userid=? AND gameid=?", godID, gameID).First(&godGame).Error
	if err != nil {
		return godGame
	}
	bs, _ = json.Marshal(godGame)
	c.Do("SET", RKGodGameInfo(godID, gameID), string(bs), "EX", 604800)
	return godGame
}

// 根据身份证查询大神
func (dao *Dao) GetGodByIDCard(idcard string) *model.God {
	var god model.God
	err := dao.dbr.Where("idcard=?", idcard).First(&god).Error
	if err != nil || god.UserID == 0 {
		return nil
	}
	return &god
}

// 根据手机号查询大神
func (dao *Dao) GetGodByPhone(phone string) *model.God {
	var god model.God
	err := dao.dbr.Where("phone=?", phone).First(&god).Error
	if err != nil || god.UserID == 0 {
		return nil
	}
	return &god
}

// 根据用户ID查询大神
func (dao *Dao) GetGodByUserID(userID int64) *model.God {
	var god model.God
	dao.dbr.Table("play_gods").Where("userid=?", userID).First(&god)
	if god.UserID > 0 {
		return &god
	}
	dao.dbr.Table("play_gods_apply").Where("userid=?", userID).First(&god)
	if god.UserID > 0 {
		return &god
	}
	return nil
}

// 大神申请
func (dao *Dao) GodApply(godApply model.GodApply) error {
	godApply.Createdtime = time.Now()
	return dao.dbw.Table("play_gods_apply").Where("userid=?", godApply.UserID).Assign(godApply).FirstOrCreate(&godApply).Error
}

// 冻结大神
// 只有已通过状态的大神可以被冻结
func (dao *Dao) BlockGod(godID int64) error {
	var god model.God
	err := dao.dbw.Where(&model.God{UserID: godID, Status: constants.GOD_STATUS_PASSED}).First(&god).Error
	if err != nil {
		return err
	}
	god.Status = constants.GOD_STATUS_BLOCKED
	err = dao.dbw.Model(&god).Update("status", constants.GOD_STATUS_BLOCKED).Error
	if err == nil {
		// 冻结后，自动将所有游戏的接单开关设为关闭，解冻不恢复开关状态，需要让大神自己手动开启
		dao.dbw.Table("play_god_accept_setting").Where("god_id=?", godID).Updates(map[string]interface{}{"grab_switch": constants.GRAB_SWITCH_CLOSE, "grab_switch2": constants.GRAB_SWITCH2_CLOSE, "grab_switch3": constants.GRAB_SWITCH3_CLOSE})
		// Update("grab_switch", constants.GRAB_SWITCH_CLOSE)
		bs, _ := json.Marshal(god)
		c := dao.cpool.Get()
		defer c.Close()
		c.Do("SET", RKGodInfo(godID), string(bs))
		c.Do("DEL", GodAcceptOrderSettingKey(godID))
	}
	return err
}

// 解除冻结大神
// 解除冻结之后，大神状态恢复为已通过
func (dao *Dao) UnBlockGod(godID int64) error {
	var god model.God
	err := dao.dbw.Where(&model.God{UserID: godID, Status: constants.GOD_STATUS_BLOCKED}).First(&god).Error
	if err != nil {
		return err
	}
	god.Status = constants.GOD_STATUS_PASSED
	err = dao.dbw.Model(&god).Update("status", constants.GOD_STATUS_PASSED).Error
	if err == nil {
		c := dao.cpool.Get()
		defer c.Close()
		bs, _ := json.Marshal(god)
		c.Do("SET", RKGodInfo(godID), string(bs))
	}
	return err
}

// 陪玩品类申请
func (dao *Dao) GodGameApply(apply model.GodGameApply) error {
	apply.Status = constants.GOD_GAME_APPLY_STATUS_PENDING
	apply.Createdtime = time.Now()
	var oldData model.GodGameApply
	var err error
	dao.dbw.Table("play_god_games_apply").Where("userid=? AND gameid=?", apply.UserID, apply.GameID).First(&oldData)
	if oldData.ID > 0 {
		apply.ID = oldData.ID
		err = dao.dbw.Save(&apply).Error
	} else {
		err = dao.dbw.Create(&apply).Error
	}
	if err != nil {
		return err
	}
	bs, err := json.Marshal(apply)
	c := dao.cpool.Get()
	defer c.Close()
	if err == nil {
		c.Do("SET", RKGodGameApply(apply.UserID, apply.GameID), string(bs), "EX", 604800)
	}
	return nil
}

// 修改大神视频
func (dao *Dao) ModifyVideos(apply model.GodGameApply) error {
	var old model.GodGameApply
	var err error
	var bs []byte
	dao.dbr.Table("play_god_games_apply").
		Where("userid=? AND gameid=?", apply.UserID, apply.GameID).
		First(&old)
	if old.ID > 0 {
		old.Status = constants.GOD_GAME_APPLY_STATUS_PENDING
		old.Createdtime = time.Now()
		old.Video = apply.Video
		old.Videos = apply.Videos
		err = dao.dbw.Save(&old).Error
		if err == nil {
			bs, _ = json.Marshal(old)
		}
	} else {
		apply.Status = constants.GOD_GAME_APPLY_STATUS_PENDING
		apply.Createdtime = time.Now()
		apply.ID = 0
		err = dao.dbw.Create(&apply).Error
		if err == nil {
			bs, _ = json.Marshal(apply)
		}
	}
	if err != nil {
		return err
	}
	if len(bs) > 0 {
		c := dao.cpool.Get()
		defer c.Close()
		c.Do("SET", RKGodGameApply(apply.UserID, apply.GameID), string(bs), "EX", 604800)
	}
	return nil
}

// 判断用户是否可以修改陪玩资料，一周一次
func (dao *Dao) CheckGodCanModifyGameInfo(godID, gameID int64) bool {
	redisKey := RKLastModifyInfoDate(godID)
	c := dao.cpool.Get()
	fin, _ := redis.Int64(c.Do("HGET", redisKey, fmt.Sprintf("fin%d", gameID)))
	c.Close()
	now := time.Now().Unix()
	return (now - fin) >= constants.VALID_MODIFY_INFO_DURATION
}

// 获取申请列表
func (dao *Dao) GetGodGameApplys(status, gameID, godID, offset, gender, leaderID int64) ([]model.GodGame, error) {
	limit := 10
	items := make([]model.GodGame, 0, limit)
	var err error
	var db *gorm.DB
	if status == constants.GOD_GAME_STATUS_PASSED || status == constants.GOD_GAME_STATUS_BLOCKED {
		db = dao.dbr.Table("play_god_games").Where("play_god_games.status=?", status)
		if gameID > 0 {
			db = db.Where("play_god_games.gameid=?", gameID)
		}
		if godID > 0 {
			db = db.Where("play_god_games.userid=?", godID)
		}
		if gender > 0 || leaderID > 0 {
			db = db.Joins("inner join play_gods on play_god_games.userid=play_gods.userid")
			if gender > 0 {
				db = db.Where("play_gods.gender=?", gender)
			}
			if leaderID > 0 {
				db = db.Where("play_gods.leader_id=?", leaderID)
			}
		}
		err = db.Order("play_god_games.createdtime desc").Offset(offset).Limit(limit).Find(&items).Error
	} else if status == constants.GOD_GAME_APPLY_STATUS_PENDING || status == constants.GOD_GAME_APPLY_STATUS_REFUSED {
		sql := "select * from play_god_games_apply "
		sql2 := "select * from play_god_games_apply "
		if gender > 0 || leaderID > 0 {
			sql += " inner join play_gods on play_god_games_apply.userid=play_gods.userid "
			sql += fmt.Sprintf(" where not exists (select * from play_god_games where play_god_games.userid=play_god_games_apply.userid and play_god_games.gameid=play_god_games_apply.gameid) and play_god_games_apply.status=%d", status)

			sql2 += " inner join play_gods_apply on play_god_games_apply.userid=play_gods_apply.userid "
			sql2 += fmt.Sprintf(" where not exists (select * from play_god_games where play_god_games.userid=play_god_games_apply.userid and play_god_games.gameid=play_god_games_apply.gameid) and play_god_games_apply.status=%d", status)
			if gender > 0 {
				sql += fmt.Sprintf(" and play_gods.gender=%d", gender)
				sql2 += fmt.Sprintf(" and play_gods_apply.gender=%d", gender)
			}
			if leaderID > 0 {
				sql += fmt.Sprintf(" and play_gods.leader_id=%d", leaderID)
			}
		} else {
			sql += fmt.Sprintf(" where not exists (select * from play_god_games where play_god_games.userid=play_god_games_apply.userid and play_god_games.gameid=play_god_games_apply.gameid) and play_god_games_apply.status=%d", status)
			sql2 += fmt.Sprintf(" where not exists (select * from play_god_games where play_god_games.userid=play_god_games_apply.userid and play_god_games.gameid=play_god_games_apply.gameid) and play_god_games_apply.status=%d", status)
		}
		if gameID > 0 {
			sql = fmt.Sprintf("%s and play_god_games_apply.gameid=%d", sql, gameID)
			sql2 = fmt.Sprintf("%s and play_god_games_apply.gameid=%d", sql2, gameID)
		}
		if godID > 0 {
			sql = fmt.Sprintf("%s and play_god_games_apply.userid=%d", sql, godID)
			sql2 = fmt.Sprintf("%s and play_god_games_apply.userid=%d", sql2, godID)
		}
		sql = fmt.Sprintf("%s order by play_god_games_apply.createdtime desc limit %d offset %d", sql, limit, offset)
		err = dao.dbr.Raw(sql).Scan(&items).Error
		if gender > 0 && godID == 0 {
			sql2 = fmt.Sprintf("%s order by play_god_games_apply.createdtime desc limit %d offset %d", sql2, limit, offset)
			items2 := make([]model.GodGame, 0, limit)
			dao.dbr.Raw(sql2).Scan(&items2)
			if len(items2) > 0 {
				items = append(items, items2...)
			}
		}
	} else if status == 100 {
		// 已通过、被冻结
		db = dao.dbr.Table("play_god_games")
		if gameID > 0 {
			db = db.Where("play_god_games.gameid=?", gameID)
		}
		if godID > 0 {
			db = db.Where("play_god_games.userid=?", godID)
		}
		if gender > 0 || leaderID > 0 {
			db = db.Joins("inner join play_gods on play_god_games.userid=play_gods.userid")
			if gender > 0 {
				db = db.Where("play_gods.gender=?", gender)
			}
			if leaderID > 0 {
				db = db.Where("play_gods.leader_id=?", leaderID)
			}
		}
		err = db.Order("play_god_games.createdtime desc").Offset(offset).Limit(limit).Find(&items).Error
	} else if status == 8 {
		// 已通过或被冻结状态下再次改资料
		db = dao.dbr.Table("play_god_games_apply").Select("play_god_games_apply.*,play_god_games.recommend").Joins("inner join play_god_games on play_god_games_apply.userid=play_god_games.userid and play_god_games_apply.gameid=play_god_games.gameid").Where("play_god_games_apply.status=?", constants.GOD_GAME_APPLY_STATUS_PENDING)
		if gameID > 0 {
			db = db.Where("play_god_games_apply.gameid=?", gameID)
		}
		if godID > 0 {
			db = db.Where("play_god_games_apply.userid=?", godID)
		}
		if gender > 0 || leaderID > 0 {
			db = db.Joins("inner join play_gods on play_god_games_apply.userid=play_gods.userid")
			if gender > 0 {
				db = db.Where("play_gods.gender=?", gender)
			}
			if leaderID > 0 {
				db = db.Where("play_gods.leader_id=?", leaderID)
			}
		}
		err = db.Order("play_god_games_apply.createdtime desc").Offset(offset).Limit(limit).Find(&items).Error
	} else {
		err = fmt.Errorf("无效的状态条件%d", status)
		return items, err
	}
	return items, err
}

// 审核品类申请
func (dao *Dao) GodGameAudit(status, gameID, godID, recommend, grabStatus int64) (bool, error) {
	var err error
	var godGame, oldGodGame model.GodGame
	var isGod bool
	err = dao.dbw.Table("play_god_games_apply").Where("userid=? AND gameid=?", godID, gameID).First(&godGame).Error
	if err != nil {
		return isGod, err
	}
	if godGame.Status != constants.GOD_GAME_APPLY_STATUS_PENDING {
		return isGod, fmt.Errorf("非审核中的状态%d，不能执行此操作", godGame.Status)
	}
	dao.dbw.Table("play_god_games").Where("userid=? AND gameid=?", godID, gameID).First(&oldGodGame)
	isGod = dao.IsGod(godID)
	if status == constants.GOD_GAME_STATUS_PASSED {
		var god model.God
		if !isGod {
			err = dao.dbw.Table("play_gods_apply").Where("userid=?", godID).First(&god).Error
			if err != nil {
				return isGod, err
			}
			// 创建钱包账户
			purse_pb.Create(frame.TODO(), &purse_pb.CreateReq{
				Mid:          godID,
				Phone:        god.Phone,
				Name:         god.RealName,
				IdentityCard: god.IDcard,
			})
		}
		godGame.Recommend = recommend
		godGame.GrabStatus = grabStatus
		godGame.ID = 0 // 防止FirstOrCreate时主键重复
		firstGodGame := godGame
		firstGodGame.Status = constants.GOD_GAME_STATUS_PASSED
		firstGodGame.GodLevel = 1
		firstGodGame.Createdtime = time.Now()
		firstGodGame.Passedtime = time.Now()
		godGame.Updatedtime = time.Now()
		if oldGodGame.ID > 0 {
			godGame.Status = oldGodGame.Status
		}
		tx := dao.dbw.Begin()
		err = tx.Error
		if err != nil {
			return isGod, err
		}
		// err = tx.Table("play_god_games").Where("userid=? AND gameid=?", godID, gameID).Assign(godGame).FirstOrCreate(&firstGodGame).Error
		db := tx.Table("play_god_games").Where("userid=? AND gameid=?", godID, gameID).Assign(godGame)
		if godGame.Aac == "" {
			db = db.Update("aac", "")
		}
		if godGame.Video == "" {
			db = db.Update("video", "")
		}
		if godGame.Videos == "" {
			db = db.Update("videos", "")
		}
		if godGame.Powers == "" {
			db = db.Update("powers", "")
		}
		err = db.FirstOrCreate(&firstGodGame).Error
		if err != nil {
			tx.Rollback()
			return isGod, err
		}
		err = tx.Table("play_god_games_apply").Where("userid=? AND gameid=?", godID, gameID).Delete(model.GodGameApply{}).Error
		if err != nil {
			tx.Rollback()
			return isGod, err
		}

		if !isGod {
			god.ID = 0 // 防止FirstOrCreate时主键重复
			god.Status = constants.GOD_STATUS_PASSED
			god.Createdtime = time.Now()
			err = tx.Table("play_gods").Where("userid=?", godID).Assign(god).FirstOrCreate(&god).Error
			if err != nil {
				tx.Rollback()
				return isGod, err
			}
			err = tx.Table("play_gods_apply").Where("userid=?", godID).Delete(model.GodApply{}).Error
			if err != nil {
				tx.Rollback()
				return isGod, err
			}
		}
		err = tx.Commit().Error
		if err != nil {
			return isGod, err
		}
		c := dao.cpool.Get()
		defer c.Close()
		bs, _ := json.Marshal(firstGodGame)
		if !isGod {
			bs, _ = json.Marshal(god)
			c.Do("SET", RKGodInfo(godID), string(bs))
		} else {
			c.Do("DEL", RKOneGodGameV1(godID, gameID), RKSimpleGodGamesKey(godID))
		}
		c.Do("DEL", RKGodGameApply(godID, gameID), RKGodGameInfo(godID, gameID))
	} else if status == constants.GOD_GAME_APPLY_STATUS_REFUSED {
		err = dao.dbw.Table("play_god_games_apply").Where("userid=? AND gameid=?", godID, gameID).Update("status", constants.GOD_GAME_APPLY_STATUS_REFUSED).Error
		c := dao.cpool.Get()
		defer c.Close()
		c.Do("DEL", RKGodGameApply(godID, gameID), RKOneGodGameV1(godID, gameID), RKGodGameInfo(godID, gameID), RKSimpleGodGamesKey(godID))
	} else {
		return isGod, fmt.Errorf("无效的审核状态%d", status)
	}
	return isGod, err
}

// 更新上一次品类资料修改时间
func (dao *Dao) ModifyLastModifyInfoTime(godID, gameID int64) {
	c := dao.cpool.Get()
	c.Do("HSET", RKLastModifyInfoDate(godID), fmt.Sprintf("fin%d", gameID), time.Now().Unix())
	c.Close()
}

// 冻结品类
// 只有已通过状态的品类可以被冻结
func (dao *Dao) BlockGodGame(godID, gameID int64) error {
	var godGame model.GodGame
	err := dao.dbw.Where("userid=? AND gameid=? AND status=?", godID, gameID, constants.GOD_GAME_STATUS_PASSED).First(&godGame).Error
	if err != nil {
		return err
	}
	err = dao.dbw.Model(&godGame).Update("status", constants.GOD_GAME_STATUS_BLOCKED).Error
	// 冻结后，自动游戏的接单开关设为关闭，解冻不恢复开关状态，需要让大神自己手动开启
	dao.dbw.Table("play_god_accept_setting").Where("god_id=? AND game_id=?", godID, gameID).Updates(map[string]interface{}{"grab_switch": constants.GRAB_SWITCH_CLOSE, "grab_switch2": constants.GRAB_SWITCH2_CLOSE, "grab_switch3": constants.GRAB_SWITCH3_CLOSE})
	c := dao.cpool.Get()
	c.Do("DEL", RKGodGameInfo(godID, gameID), GodAcceptOrderSettingKey(godID))
	c.Close()
	return err
}

// 解除冻结品类
func (dao *Dao) UnBlockGodGame(godID, gameID int64) error {
	var godGame model.GodGame
	err := dao.dbw.Where("userid=? AND gameid=? AND status=?", godID, gameID, constants.GOD_GAME_STATUS_BLOCKED).First(&godGame).Error
	if err != nil {
		return err
	}
	err = dao.dbw.Model(&godGame).Update("status", constants.GOD_GAME_STATUS_PASSED).Error
	c := dao.cpool.Get()
	c.Do("DEL", RKGodGameInfo(godID, gameID), RKOneGodGameV1(godID, gameID), RKSimpleGodGamesKey(godID))
	c.Close()
	return err
}

// 设置是否允许接开黑上分单
func (dao *Dao) ModifyGrabPermission(godID, gameID, grabStatus int64) error {
	var godGame model.GodGame
	err := dao.dbw.Where("userid=? AND gameid=? AND status=?", godID, gameID, constants.GOD_GAME_STATUS_PASSED).First(&godGame).Error
	if err != nil {
		return err
	}
	if grabStatus == constants.GRAB_STATUS_YES {
		err = dao.dbw.Where("id=?", godGame.ID).Update("grab_status", constants.GRAB_STATUS_YES).Error
	} else if grabStatus == constants.GRAB_STATUS_NO {
		err = dao.dbw.Where("id=?", godGame.ID).Update("grab_status", constants.GRAB_STATUS_NO).Error
		// 从老的开黑大神池删除
		resp, err := gamepb.AcceptCfgV2(frame.TODO(), &gamepb.AcceptCfgV2Req{
			GameId: gameID,
		})
		if err != nil || resp.GetErrcode() != 0 {
			icelog.Errorf("ModifyGrabPermission: clean old data error[%d] %v", resp.GetErrcode(), err)
		} else {
			redisConn := dao.cpool.Get()
			defer redisConn.Close()
			for region, _ := range resp.GetData().GetRegions() {
				for level, _ := range resp.GetData().GetLevels() {
					redisConn.Do("ZREM", GodsRedisKey3(gameID, region, level), godID)
				}
			}
		}
	}
	return err
}

// 获取老的陪玩申请数据
func (dao *Dao) GetOldData(godID, gameID int64) (model.GodGameApply, error) {
	var data model.GodGameApply
	var err error
	err = dao.dbr.Table("play_god_games_apply").Where("userid=? AND gameid=?", godID, gameID).First(&data).Error
	if err == nil && data.ID > 0 {
		if data.Status == constants.GOD_GAME_APPLY_STATUS_REFUSED {
			// 被拒绝的老视频,不返回；返回老的已通过的视频
			data.Video = ""
			data.Videos = ""
			var data2 model.GodGameApply
			if err = dao.dbr.Table("play_god_games").Where("userid=? AND gameid=?", godID, gameID).First(&data2).Error; err == nil {
				data.Video = data2.Video
				data.Videos = data2.Videos
			}
		}
		return data, nil
	}
	err = dao.dbr.Table("play_god_games").Where("userid=? AND gameid=?", godID, gameID).First(&data).Error
	return data, err
}

func (dao *Dao) GetGodAllGameV1(godID int64) (model.GodGameV1sSortedByAcceptNum, error) {
	var err error
	var v1s model.GodGameV1sSortedByAcceptNum
	v1s = make(model.GodGameV1sSortedByAcceptNum, 0, 10)
	var games []model.GodGame
	err = dao.dbr.Table("play_god_games").Select("gameid").Where("userid=? AND status=?", godID, constants.GOD_GAME_STATUS_PASSED).Find(&games).Error
	if err != nil {
		return v1s, err
	}
	for _, game := range games {
		if v1, err := dao.GetGodSpecialGameV1(godID, game.GameID); err == nil {
			v1s = append(v1s, v1)
		}
	}
	return v1s, nil
}

func (dao *Dao) GetGodSpecialGameV1(godID, gameID int64) (model.GodGameV1, error) {
	var v1 model.GodGameV1
	gameStateResp, err := gamepb.State(frame.TODO(), &gamepb.StateReq{
		GameId: gameID,
	})
	if err != nil || gameStateResp.GetErrcode() != 0 {
		return v1, fmt.Errorf("数据加载失败")
	} else if gameStateResp.GetData().GetState() != game_const.GAME_STATE_OK {
		return v1, fmt.Errorf("游戏已下架")
	}
	var acceptNum int64
	orderResp, err := plorderpb.Count(frame.TODO(), &plorderpb.CountReq{
		GodId:  godID,
		GameId: gameID,
	})
	if err != nil {
		icelog.Errorf("Get orderCount[%d] error:%s", godID, err)
	}
	if orderResp != nil && orderResp.GetData() != nil {
		acceptNum = orderResp.GetData().GetCompletedHoursAmount() // 一单多小时算多单
	}
	var godGame model.GodGame
	var bs []byte
	c := dao.cpool.Get()
	defer c.Close()
	var godIconUrl string
	if bs, err = redis.Bytes(c.Do("GET", RKGodIcon(godID))); err == nil {
		var tmpGodIcon model.TmpGodIcon
		err = json.Unmarshal(bs, &tmpGodIcon)
		now := time.Now().Unix()
		if tmpGodIcon.Begin <= now && tmpGodIcon.End > now {
			godIconUrl, _ = redis.String(c.Do("HGET", RKGodIcons(), tmpGodIcon.ID))
		}
	}
	if bs, err := redis.Bytes(c.Do("GET", RKOneGodGameV1(godID, gameID))); err == nil {
		if err = json.Unmarshal(bs, &v1); err == nil {
			v1.AcceptNum = acceptNum
			v1.GodIcon = godIconUrl
			return v1, nil
		}
	}

	err = dao.dbr.Table("play_god_games").Where("userid=? AND gameid=? AND status=?", godID, gameID, constants.GOD_GAME_STATUS_PASSED).First(&godGame).Error
	if err != nil {
		return v1, err
	}
	v1.AcceptNum = acceptNum
	v1.GodID = godID
	v1.GameID = gameID
	v1.Level = godGame.GodLevel
	v1.HighestLevelID = godGame.HighestLevelID
	v1.GameScreenshot = godGame.GameScreenshot
	v1.Images = godGame.Images
	v1.Powers = godGame.Powers
	v1.Voice = godGame.Voice
	v1.VoiceDuration = godGame.VoiceDuration
	v1.Aac = godGame.Aac
	v1.Video = godGame.Video
	v1.Videos = godGame.Videos
	v1.Tags = godGame.Tags
	v1.Ext = godGame.Ext
	v1.Desc = godGame.Desc
	v1.PriceType = godGame.PeiwanPriceType
	v1.PeiWanPrice = godGame.PeiwanPrice
	v1.GrabStatus = godGame.GrabStatus
	v1.Recommend = godGame.Recommend
	v1.Status = godGame.Status
	if v1.Recommend == constants.RECOMMEND_YES {
		v1.Weight, _ = redis.Int64(c.Do("ZSCORE", RKUpperGodGames(), fmt.Sprintf("%d-%d", godID, gameID)))
	}
	comment, err := plcommentpb.GetGodGameComment(frame.TODO(), &plcommentpb.GetGodGameCommentReq{
		GodId:  godID,
		GameId: gameID,
	})
	if err != nil {
		icelog.Errorf("GetGodGameComment[%d] error:%s", godID, err)
	}
	if comment != nil && comment.GetData() != nil {
		v1.Score = comment.GetData().GetScore()
	}
	v1.GrabSwitch = constants.GRAB_SWITCH2_CLOSE
	v1.GrabSwitch2 = constants.GRAB_SWITCH2_CLOSE
	v1.GrabSwitch3 = constants.GRAB_SWITCH3_CLOSE
	v1.GrabSwitch4 = constants.GRAB_SWITCH4_CLOSE
	accpetOrderSetting, err := dao.GetGodSpecialAcceptOrderSetting(godID, gameID)
	if err == nil {
		v1.PriceID = accpetOrderSetting.PriceID
		v1.Regions = accpetOrderSetting.Regions
		v1.Levels = accpetOrderSetting.Levels
		v1.GrabSwitch = accpetOrderSetting.GrabSwitch
		v1.GrabSwitch2 = accpetOrderSetting.GrabSwitch2
		v1.GrabSwitch3 = accpetOrderSetting.GrabSwitch3
		v1.GrabSwitch4 = accpetOrderSetting.GrabSwitch4
	}
	v1.GodIcon = godIconUrl
	if bs, err := json.Marshal(v1); err == nil {
		c.Do("SET", RKOneGodGameV1(godID, gameID), string(bs), "EX", 86400)
	}
	return v1, nil
}

func (dao *Dao) GetHeadline(userID, offset int64) time.Time {
	c := dao.cpool.Get()
	defer c.Close()
	redisKey := fmt.Sprintf("U:{%d}:Headline", userID)
	if offset == 0 {
		now := time.Now()
		c.Do("SET", redisKey, now.Unix(), "EX", 300)
		return now
	}
	ts, _ := redis.Int64(c.Do("GET", redisKey))
	if ts > 0 {
		return time.Unix(ts, 0)
	}
	now := time.Now()
	c.Do("SET", redisKey, now.Unix(), "EX", 300)
	return now
}

// 获取【实力】陪玩满足条件的大神列表
// 接单设置满足game+region+startLevel的大神列表
func (dao *Dao) GetOrderGods(gameID, region2, startLevel1, endLevel1 int64) (gods []int64) {
	startLevel, err := gamepb.LevelAccepted(frame.TODO(), &gamepb.LevelAcceptedReq{
		GameId:   gameID,
		Level1Id: startLevel1,
	})
	if err != nil || startLevel == nil {
		return
	} else if startLevel.GetErrcode() != 0 {
		return
	}
	acceptID := startLevel.GetData().GetAcceptId()

	endLevel, err := gamepb.LevelAccepted(frame.TODO(), &gamepb.LevelAcceptedReq{
		GameId:   gameID,
		Level1Id: endLevel1,
	})
	if err != nil {
		log.Error(err.Error())
		return
	} else if endLevel.GetErrcode() != 0 {
		log.Error(endLevel.GetErrmsg())
		return
	}
	c := dao.cpool.Get()
	defer c.Close()

	redisKey := GodsRedisKey3(gameID, region2, acceptID)
	gods, _ = redis.Int64s(c.Do("ZRANGEBYSCORE", redisKey, endLevel.GetData().GetAcceptId(), "+inf"))
	return
}

// OM后台取消大神抢开黑单权限、冻结大神、冻结大神品类时，需要将大神从该游戏的开黑单大神池移除
func (dao *Dao) DisableGodGrabOrder(gameID, godID int64) {
	resp, err := gamepb.AcceptCfgV2(frame.TODO(), &gamepb.AcceptCfgV2Req{
		GameId: gameID,
	})
	if err != nil || resp.GetErrcode() != 0 {
		icelog.Errorf("DisableGodGrabOrder %d-%d error %s", gameID, godID, resp.GetErrmsg())
		return
	}
	var redisKey string
	c := dao.cpool.Get()
	defer c.Close()
	for region, _ := range resp.GetData().GetRegions() {
		for level1, _ := range resp.GetData().GetLevels() {
			redisKey = GodsRedisKey3(gameID, region, level1)
			c.Do("ZREM", redisKey, godID)
		}
	}
}

// 获取满足条件的即时约大神列表
// 按照最后登陆时间降序
// 只推送最近7天内登陆过的
func (dao *Dao) GetJSYOrderGods(gameID, gender int64) []int64 {
	c := dao.cpool.Get()
	defer c.Close()
	var gods []int64
	begin := time.Now().Unix()
	end := begin - 604800
	if gender == constants.GENDER_MALE || gender == constants.GENDER_FEMALE {
		redisKey := RKJSYGods(gameID, gender)
		gods, _ = redis.Int64s(c.Do("ZREVRANGEBYSCORE", redisKey, begin, end))
	} else {
		girls, _ := redis.Int64s(c.Do("ZREVRANGEBYSCORE", RKJSYGods(gameID, constants.GENDER_FEMALE), begin, end))
		boys, _ := redis.Int64s(c.Do("ZREVRANGEBYSCORE", RKJSYGods(gameID, constants.GENDER_MALE), begin, end))
		gods = append(girls, boys...)
	}
	return gods
}

// 从即时约大神池删除
func (dao *Dao) RemoveFromJSYGodPool(gameID, godID int64) {
	c := dao.cpool.Get()
	defer c.Close()
	c.Do("ZREM", RKJSYGods(gameID, constants.GENDER_FEMALE), godID)
	c.Do("ZREM", RKJSYGods(gameID, constants.GENDER_MALE), godID)
}

// 获取满足条件的派单大神列表
// 按照最后登陆时间降序
// 只推送最近7天内登陆过的
func (dao *Dao) GetJSYPaiDanGods(gameID, gender int64) []int64 {
	c := dao.cpool.Get()
	defer c.Close()
	var gods []int64
	begin := time.Now().Unix()
	end := begin - 604800
	if gender == constants.GENDER_MALE || gender == constants.GENDER_FEMALE {
		redisKey := RKJSYPaiDanGods(gameID, gender)
		gods, _ = redis.Int64s(c.Do("ZREVRANGEBYSCORE", redisKey, begin, end))
	} else {
		girls, _ := redis.Int64s(c.Do("ZREVRANGEBYSCORE", RKJSYPaiDanGods(gameID, constants.GENDER_FEMALE), begin, end))
		boys, _ := redis.Int64s(c.Do("ZREVRANGEBYSCORE", RKJSYPaiDanGods(gameID, constants.GENDER_MALE), begin, end))
		gods = append(girls, boys...)
	}
	return gods
}

// 从派单大神池删除
func (dao *Dao) RemoveFromJSYPaiDanGodPool(gameID, godID int64) {
	c := dao.cpool.Get()
	defer c.Close()
	c.Do("ZREM", RKJSYPaiDanGods(gameID, constants.GENDER_FEMALE), godID)
	c.Do("ZREM", RKJSYPaiDanGods(gameID, constants.GENDER_MALE), godID)
}

// 修改陪玩首页权重
func (dao *Dao) ModifyUpperGodGame(godID, gameID, weight int64) error {
	c := dao.cpool.Get()
	defer c.Close()
	val := fmt.Sprintf("%d-%d", godID, gameID)
	if weight == 0 {
		c.Do("ZREM", RKUpperGodGames(), val)
		return nil
	}
	if score, _ := redis.Int64(c.Do("ZSCORE", RKUpperGodGames(), val)); score > 0 {
		c.Do("ZADD", RKUpperGodGames(), weight, val)
		return nil
	}
	if cnt, _ := redis.Int(c.Do("ZCARD", RKUpperGodGames())); cnt >= 20 {
		return fmt.Errorf("置顶数达到上限 %d", cnt)
	}
	c.Do("ZADD", RKUpperGodGames(), weight, val)
	return nil
}

// 获取陪玩首页权重列表
func (dao *Dao) GetUpperGodGames() (map[string]int64, []string, error) {
	c := dao.cpool.Get()
	defer c.Close()
	ret1, err := redis.Int64Map(c.Do("ZREVRANGE", RKUpperGodGames(), 0, -1, "WITHSCORES"))
	ret2, err := redis.Strings(c.Do("ZREVRANGE", RKUpperGodGames(), 0, -1))
	return ret1, ret2, err
}

// 获取大神游戏首页置顶权重值
func (dao *Dao) GetGodGameWeight(godID, gameID int64) int64 {
	c := dao.cpool.Get()
	defer c.Close()
	weight, _ := redis.Int64(c.Do("ZSCORE", RKUpperGodGames(), fmt.Sprintf("%d-%d", godID, gameID)))
	return weight
}

// 修改陪玩信息
func (dao *Dao) ModifyGodGameInfo(godGame model.GodGame) error {
	err := dao.dbw.Save(&godGame).Error
	if err != nil {
		return err
	}
	c := dao.cpool.Get()
	defer c.Close()
	c.Do("DEL", RKGodGameInfo(godGame.UserID, godGame.GameID), RKOneGodGameV1(godGame.UserID, godGame.GameID), RKSimpleGodGamesKey(godGame.UserID))
	return nil
}

// 获取大神所有游戏的申请状态
// play_god_games表中有记录，则视为已通过；否则使用play_god_games_apply表中的状态
func (dao *Dao) GetGodGameStatus(godID int64) map[int64]int64 {
	var gameID, status, status2 int64
	ret := make(map[int64]int64)
	rows, err := dao.dbr.Table("play_god_games").Select("gameid, status").Where("userid=?", godID).Rows()
	if err == nil {
		for rows.Next() {
			err = rows.Scan(&gameID, &status)
			if err == nil {
				ret[gameID] = status
			}
		}
	}
	var ok bool
	rows, err = dao.dbr.Table("play_god_games_apply").Select("gameid, status").Where("userid=?", godID).Rows()
	if err == nil {
		for rows.Next() {
			err = rows.Scan(&gameID, &status)
			if err == nil {
				if status2, ok = ret[gameID]; ok {
					ret[gameID] = status2
				} else {
					ret[gameID] = status
				}
			}
		}
	}
	return ret
}

// OM后台修改大神信息
func (dao *Dao) ModifyGodInfo(godHistory model.GodsHistory) (model.GodsHistory, error) {
	godHistory.Status = 2

	godHistory.Createdtime = time.Now()
	err := dao.dbw.Table("play_gods_history").Where("god_id=? and status in (?, ?)", godHistory.GodID, 1, 2).Assign(godHistory).FirstOrCreate(&godHistory).Error
	return godHistory, err
}

// 修改大神自定义介绍内容
func (dao *Dao) ModifyGodDesc(godID int64, desc string) error {
	err := dao.dbw.Table("play_gods").Where("userid=?", godID).Updates(map[string]interface{}{
		"desc":        desc,
		"updatedtime": time.Now(),
	}).Error
	if err == nil {
		c := dao.cpool.Get()
		c.Do("DEL", RKGodInfo(godID))
		c.Do("SET", RKGodLastModifyDesc(godID), time.Now().Unix())
		c.Close()
	}
	return err
}

// OM后台审批大神修改申请
// status 1:打回 3:通过
func (dao *Dao) AuditModifyGodInfo(godID, status int64) error {
	var godHistory model.GodsHistory
	err := dao.dbw.Table("play_gods_history").Where("god_id=? and status=2", godID).First(&godHistory).Error
	if err != nil {
		return err
	}
	var godInfo model.God
	err = dao.dbr.Table("play_gods").Where("userid=?", godID).First(&godInfo).Error
	if err != nil {
		return err
	}
	if status == 1 {
		err = dao.dbw.Model(&godHistory).Update("status", 1).Error
	} else if status == 3 {
		// if godHistory.Alipay != "" {
		// 调用钱包服务，修改支付宝账号
		purseResp, err := purse_pb.Update(frame.TODO(), &purse_pb.UpdateReq{
			Mid:             godID,
			Phone:           godHistory.Phone,
			AccountName:     godHistory.RealName,
			IdentityCard:    godHistory.IDcard,
			WithdrawAccount: godHistory.Alipay,
		})
		if err != nil || purseResp.GetErrcode() != 0 {
			return fmt.Errorf("钱包账户更新失败")
		}
		// }
		godInfo.RealName = godHistory.RealName
		godInfo.IDcard = godHistory.IDcard
		godInfo.IDcardurl = godHistory.IDcardurl
		godInfo.Phone = godHistory.Phone
		godInfo.Gender = godHistory.Gender
		godInfo.LeaderSwitch = godHistory.LeaderSwitch
		godInfo.LeaderID = godHistory.LeaderID
		godInfo.Updatedtime = time.Now()
		tx := dao.dbw.Begin()
		err = tx.Error
		if err != nil {
			return err
		}
		err = tx.Save(&godInfo).Error
		if err != nil {
			tx.Rollback()
			return err
		}
		err = tx.Model(&godHistory).Update("status", 3).Error
		if err != nil {
			tx.Rollback()
			return err
		}
		err = tx.Commit().Error
		if err != nil {
			return err
		}
		c := dao.cpool.Get()
		c.Do("DEL", RKGodInfo(godID))
		c.Close()
	} else {
		return fmt.Errorf("Invalid status %d", status)
	}
	return err
}

// OM后台修改大神信息
func (dao *Dao) QueryGodHistory(status, godID, offset int64) ([]model.GodsHistory, error) {
	var ret []model.GodsHistory
	db := dao.dbr.Table("play_gods_history")
	if status > 0 {
		db = db.Where("status=?", status)
	}
	if godID > 0 {
		db = db.Where("god_id=?", godID)
	}
	if offset >= 0 {
		db = db.Offset(offset)
	}
	err := db.Limit(10).Order("createdtime desc").Find(&ret).Error
	return ret, err
}

// 检查大神是否被推荐到首页
func (dao *Dao) IsRecommendedGod(godID int64) bool {
	var cnt int
	dao.dbr.Table("play_god_games").Where("userid=? AND recommend=2 AND status=?", godID, constants.GOD_STATUS_PASSED).Count(&cnt)
	return cnt > 0
}

// 获取GodGameApply的状态
func (dao *Dao) GetGodGameApplyStatus(godID, gameID int64) int64 {
	var godGameApply model.GodGameApply
	// redisKey := RKGodGameApply(godID, gameID)
	// c := dao.cpool.Get()
	// defer c.Close()
	// bs, _ := redis.Bytes(c.Do("GET", redisKey))
	// err := json.Unmarshal(bs, &godGameApply)
	// if err == nil {
	// 	return godGameApply.Status
	// }
	err := dao.dbr.Table("play_god_games_apply").Where("userid=? AND gameid=?", godID, gameID).First(&godGameApply).Error
	if err == nil {
		return godGameApply.Status
	}
	return -1
}

// 获取大神所有被冻结的品类V1信息
func (dao *Dao) GetGodBlockedGameV1(godID int64) (model.GodGameV1sSortedByAcceptNum, error) {
	var err error
	var v1s model.GodGameV1sSortedByAcceptNum
	v1s = make(model.GodGameV1sSortedByAcceptNum, 0, 5)
	var games []model.GodGame
	err = dao.dbr.Table("play_god_games").Select("gameid").Where("userid=? AND status=?", godID, constants.GOD_GAME_STATUS_BLOCKED).Find(&games).Error
	if err != nil {
		return v1s, err
	}
	for _, game := range games {
		if v1, err := dao.GetGodSpecialBlockedGameV1(godID, game.GameID); err == nil {
			v1.GrabSwitch = constants.GRAB_SWITCH_CLOSE
			v1.GrabSwitch2 = constants.GRAB_SWITCH2_CLOSE
			v1.GrabSwitch3 = constants.GRAB_SWITCH3_CLOSE
			v1.GrabSwitch4 = constants.GRAB_SWITCH4_CLOSE
			v1s = append(v1s, v1)
		}
	}
	return v1s, nil
}

func (dao *Dao) GetGodSpecialBlockedGameV1(godID, gameID int64) (model.GodGameV1, error) {
	var v1 model.GodGameV1
	var godGame model.GodGame
	var err error
	err = dao.dbr.Table("play_god_games").Where("userid=? AND gameid=? AND status=?", godID, gameID, constants.GOD_GAME_STATUS_BLOCKED).First(&godGame).Error
	if err != nil {
		return v1, err
	}
	c := dao.cpool.Get()
	defer c.Close()
	v1.GodID = godID
	v1.GameID = gameID
	v1.Level = godGame.GodLevel
	v1.HighestLevelID = godGame.HighestLevelID
	v1.GameScreenshot = godGame.GameScreenshot
	v1.Images = godGame.Images
	v1.Voice = godGame.Voice
	v1.VoiceDuration = godGame.VoiceDuration
	v1.Aac = godGame.Aac
	v1.Video = godGame.Video
	v1.Tags = godGame.Tags
	v1.Ext = godGame.Ext
	v1.Desc = godGame.Desc
	v1.PriceType = godGame.PeiwanPriceType
	v1.PeiWanPrice = godGame.PeiwanPrice
	v1.GrabStatus = godGame.GrabStatus
	v1.Recommend = godGame.Recommend
	v1.Status = godGame.Status
	if v1.Recommend == constants.RECOMMEND_YES {
		v1.Weight, _ = redis.Int64(c.Do("ZSCORE", RKUpperGodGames(), fmt.Sprintf("%d-%d", godID, gameID)))
	}
	orderResp, err := plorderpb.Count(frame.TODO(), &plorderpb.CountReq{
		GodId:  godID,
		GameId: gameID,
	})
	if err != nil {
		icelog.Errorf("Get orderCount[%d] error:%s", godID, err)
	}
	if orderResp != nil && orderResp.GetData() != nil {
		v1.AcceptNum = orderResp.GetData().GetCompletedHoursAmount()
	}
	comment, err := plcommentpb.GetGodGameComment(frame.TODO(), &plcommentpb.GetGodGameCommentReq{
		GodId:  godID,
		GameId: gameID,
	})
	if err != nil {
		icelog.Errorf("GetGodGameComment[%d] error:%s", godID, err)
	}
	if comment != nil && comment.GetData() != nil {
		v1.Score = comment.GetData().GetScore()
	}
	accpetOrderSetting, err := dao.GetGodSpecialAcceptOrderSetting(godID, gameID)
	if err != nil {
		v1.GrabSwitch = constants.GRAB_SWITCH_CLOSE
		icelog.Errorf("GetGodSpecialAcceptOrderSettings[%d/%d] error:%s", godID, gameID, err)
	} else {
		v1.PriceID = accpetOrderSetting.PriceID
		v1.Regions = accpetOrderSetting.Regions
		v1.Levels = accpetOrderSetting.Levels
		v1.GrabSwitch = accpetOrderSetting.GrabSwitch
	}
	return v1, nil
}

func (dao *Dao) SimpleGodGames(godID int64, hidePirce bool) *godgamepb.SimpleGodGamesResp_Data {
	result := new(godgamepb.SimpleGodGamesResp_Data)
	var items []*godgamepb.SimpleGodGamesResp_Item
	if !dao.IsGod2(godID) {
		result.IsGod = false
		return result
	}
	result.IsGod = true
	redisKey := RKSimpleGodGamesKey(godID)
	c := dao.cpool.Get()
	defer c.Close()
	bs, _ := redis.Bytes(c.Do("GET", redisKey))
	if len(bs) > 0 {
		err := json.Unmarshal(bs, &items)
		if err == nil {
			if hidePirce {
				for idx, _ := range items {
					items[idx].Price = 0
				}
			}
			result.Items = items
			return result
		}
	}
	v1s, err := dao.GetGodAllGameV1(godID)
	if err != nil {
		return result
	}
	var uniprice int64
	sort.Sort(v1s)
	items = make([]*godgamepb.SimpleGodGamesResp_Item, 0, len(v1s))
	for _, v1 := range v1s {
		if v1.GrabSwitch != constants.GRAB_SWITCH_OPEN {
			continue
		}
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
		items = append(items, &godgamepb.SimpleGodGamesResp_Item{
			GameId: v1.GameID,
			Price:  uniprice,
		})
	}
	if len(items) > 0 {
		bs, _ = json.Marshal(items)
		c.Do("SET", redisKey, bs, "EX", 7200)
	}
	if hidePirce {
		for idx, _ := range items {
			items[idx].Price = 0
		}
	}
	result.Items = items
	return result
}

// SimpleGodGameIds 返回大神正在接单的品类ID列表，按品类ID升序
func (dao *Dao) SimpleGodGameIds(godID int64) []int64 {
	var gameIds []int64
	if !dao.IsGod2(godID) {
		return gameIds
	}
	v1s, err := dao.GetGodAllGameV1(godID)
	if err != nil {
		return gameIds
	}
	for _, v1 := range v1s {
		if v1.GrabSwitch != constants.GRAB_SWITCH_OPEN {
			continue
		}
		gameIds = append(gameIds, v1.GameID)
	}
	if len(gameIds) > 1 {
		sort.Slice(gameIds, func(i, j int) bool {
			return gameIds[i] < gameIds[j]
		})
	}
	return gameIds
}
