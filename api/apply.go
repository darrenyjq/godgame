package api

import (
	"encoding/json"
	"iceberg/frame"
	game_const "laoyuegou.pb/game/constants"
	"laoyuegou.pb/game/pb"
	"laoyuegou.pb/godgame/constants"
	"laoyuegou.pb/godgame/model"
	"laoyuegou.pb/godgame/pb"
	"laoyuegou.pb/lfs/pb"
	"play/common/util"
)

// 获取可以申请的游戏列表及大神游戏状态
func (gg *GodGame) ApplyGames(c frame.Context) error {
	currentUser := gg.getCurrentUser(c)
	if currentUser.UserID == 0 {
		return c.JSON2(StatusOK_V3, "", nil)
	}
	listResp, err := gamepb.ListV2(c, &gamepb.ListV2Req{})
	if err != nil || listResp.GetErrcode() != 0 {
		return c.JSON2(ERR_CODE_INTERNAL, "", nil)
	}
	map1 := gg.dao.GetGodGameStatus(currentUser.UserID)
	data := make([]*godgamepb.ApplyGamesResp_Data, 0, len(listResp.GetData()))
	var ok bool
	var status int64
	for _, game := range listResp.GetData() {
		if status, ok = map1[game.GetGameId()]; !ok {
			data = append(data, &godgamepb.ApplyGamesResp_Data{
				GameId:     game.GetGameId(),
				GameName:   game.GetGameName(),
				GameAvatar: game.GetGameAvatar(),
				Status:     constants.GOD_GAME_STATUS_UNAUTHED,
			})
		} else {
			data = append(data, &godgamepb.ApplyGamesResp_Data{
				GameId:     game.GetGameId(),
				GameName:   game.GetGameName(),
				GameAvatar: game.GetGameAvatar(),
				Status:     status,
			})
		}

	}
	return c.JSON2(StatusOK_V3, "", data)
}

// 申请大神
func (gg *GodGame) GodApply(c frame.Context) error {
	var req godgamepb.GodApplyReq
	if err := c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	currentUser := gg.getCurrentUser(c)
	if currentUser.UserID == 0 {
		return c.JSON2(ERR_CODE_FORBIDDEN, "", nil)
	} else if gg.dao.IsGod(currentUser.UserID) {
		return c.JSON2(ERR_CODE_FORBIDDEN, "您已经是大神身份，请勿重复申请", nil)
	} else if oldGod := gg.dao.GetGodByIDCard(req.GetIdcard()); oldGod != nil && oldGod.UserID != currentUser.UserID {
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "身份证号码已被注册", nil)
	} else if oldGod := gg.dao.GetGodByPhone(req.GetPhone()); oldGod != nil && oldGod.UserID != currentUser.UserID {
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "手机号已被注册", nil)
	}
	gender, birthday, err := util.GetGenderAndBirthdayByIDCardNumber(req.GetIdcard())
	if err != nil {
		gender = constants.GENDER_MALE
		birthday = currentUser.Birthday
	} else if currentUser.Birthday > 0 {
		birthday = currentUser.Birthday
	}
	godApply := model.GodApply{
		UserID:     currentUser.UserID,
		RealName:   req.GetRealname(),
		IDcard:     req.GetIdcard(),
		IDcardurl:  req.GetIdcardUrl(),
		IDcardtype: req.GetIdcardType(),
		Phone:      req.GetPhone(),
		Gender:     gender,
		Birthday:   birthday,
	}
	err = gg.dao.GodApply(godApply)
	if err != nil {
		c.Error("GodApply error:%s", err)
		return c.JSON2(ERR_CODE_INTERNAL, "", nil)
	}
	return c.JSON2(StatusOK_V3, "", nil)
}

// 申请游戏
func (gg *GodGame) GodGameApply(c frame.Context) error {
	var req godgamepb.GodGameApplyReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	if req.GetDesc() == "" {
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "陪玩介绍不能为空", nil)
	} else if len(req.GetImages()) < 2 {
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "形象照至少2张", nil)
	} else if len(req.GetImages()) > 6 {
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "形象照最多6张", nil)
	}
	currentUser := gg.getCurrentUser(c)
	if currentUser.UserID == 0 {
		return c.JSON2(ERR_CODE_FORBIDDEN, "", nil)
	}
	gameStateResp, err := gamepb.State(frame.TODO(), &gamepb.StateReq{
		GameId: req.GameId,
	})
	if err == nil && gameStateResp.GetErrcode() == 0 {
		if gameStateResp.GetData().GetState() == game_const.GAME_STATE_NO {
			return c.JSON2(ERR_CODE_DISPLAY_ERROR, "品类已下架", nil)
		}
	}
	apply := model.GodGameApply{
		UserID:         currentUser.UserID,
		GameID:         req.GameId,
		RegionID:       req.RegionId,
		HighestLevelID: req.HighestLevelId,
		GameScreenshot: req.GameScreenshot,
		Voice:          req.Voice,
		VoiceDuration:  req.VoiceDuration,
		Aac:            req.Aac,
		Video:          req.Video,
		Ext:            req.Ext,
		Desc:           req.Desc,
	}
	var bs []byte
	bs, err = json.Marshal(req.Images)
	if err != nil {
		c.Error("GodGameApply error:%s. images:%v", err, req.Images)
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	apply.Images = string(bs)
	bs, err = json.Marshal(req.Tags)
	if err != nil {
		c.Error("GodGameApply error:%s. tags:%v", err, req.Tags)
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	}
	apply.Tags = string(bs)
	err = gg.dao.GodGameApply(apply)
	if err != nil {
		c.Error("GodGameApply error:%s", err)
		return c.JSON2(ERR_CODE_INTERNAL, "", nil)
	}
	return c.JSON2(StatusOK_V3, "", nil)
}

// 修改视频
func (gg *GodGame) Videos(c frame.Context) error {
	var req godgamepb.VideosReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	} else if req.GetGameId() == 0 {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "参数错误[1]", nil)
	}
	currentUser := gg.getCurrentUser(c)
	if currentUser.UserID == 0 {
		return c.JSON2(ERR_CODE_FORBIDDEN, "", nil)
	}
	oldData, err := gg.dao.GetOldData(currentUser.UserID, req.GetGameId())
	if err != nil {
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "上传失败[1]", nil)
	}
	if len(req.GetVideos()) == 0 {
		oldData.Videos = ""
		oldData.Video = ""
		err = gg.dao.ModifyVideos(oldData)
		if err != nil {
			c.Errorf("%s", err.Error())
			return c.JSON2(ERR_CODE_DISPLAY_ERROR, "上传失败[2]", nil)
		}
		return c.JSON2(StatusOK_V3, "", nil)
	}
	resp, err := lfspb.BatchUpload(c, &lfspb.BatchUploadReq{
		Tid:  lfspb.THIRD_QINIU,
		Keys: req.GetVideos(),
	})
	if err != nil {
		c.Errorf("%s", err.Error())
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "上传失败[3]", nil)
	} else if resp.GetErrcode() != 0 {
		c.Errorf("%s", resp.GetErrmsg())
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "上传失败[4]", nil)
	} else if len(resp.GetData()) == 0 {
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "上传失败[5]", nil)
	}
	hashs := make([]string, 0, len(resp.GetData()))
	for _, key := range req.GetVideos() {
		if hash, _ := resp.GetData()[key]; hash != "" {
			hashs = append(hashs, hash)
		}
	}
	bs, err := json.Marshal(hashs)
	if err != nil {
		c.Errorf("%s", err.Error())
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "上传失败[6]", nil)
	}
	if oldData.Videos == string(bs) {
		return c.JSON2(StatusOK_V3, "", nil)
	}
	oldData.Video = hashs[0]
	oldData.Videos = string(bs)
	err = gg.dao.ModifyVideos(oldData)
	if err != nil {
		c.Errorf("%s", err.Error())
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "上传失败[7]", nil)
	}
	return c.JSON2(StatusOK_V3, "", nil)
}
