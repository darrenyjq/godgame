package api

import (
	"encoding/json"
	"fmt"
	"iceberg/frame"
	iconfig "iceberg/frame/config"
	"laoyuegou.com/http_api"
	game_const "laoyuegou.pb/game/constants"
	"laoyuegou.pb/game/pb"
	"laoyuegou.pb/godgame/constants"
	"laoyuegou.pb/godgame/model"
	"laoyuegou.pb/godgame/pb"
	"laoyuegou.pb/lfs/pb"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// 一键加入陪玩官方群
func (gg *GodGame) JoinGroup(c frame.Context) error {
	currentUser := gg.getCurrentUser(c)
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	appID := "1"
	appToken := "13f75ce60d1cdcbdec57e2868dcd6205"
	apiURL := "http://10.25.0.22:8080/api-v2/internal/peiwan/join"
	if gg.cfg.Env.Production() {
		apiURL = "http://172.16.163.180:8300/api-v2/internal/peiwan/join"
	} else if gg.cfg.Env.String() == "stag" {
		apiURL = "http://172.16.164.182:8400/api-v2/internal/peiwan/join"
	}
	params := url.Values{
		"appId":   []string{appID},
		"token":   []string{appToken},
		"user_id": []string{fmt.Sprint(currentUser.UserID)},
	}
	req, _ := http.NewRequest("POST", apiURL, strings.NewReader(params.Encode()))
	req.Header.Set("Accept", "application/json;charset=utf-8;")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8;")

	resp, err := client.Do(req)
	if err != nil {
		c.Errorf("%s", err.Error())
		return c.JSON2(StatusOK_V3, "", nil)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		var response http_api.ResponseV3
		err = json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			c.Errorf("%s", err.Error())
			return c.JSON2(StatusOK_V3, "", nil)
		}
		return c.JSON2(StatusOK_V3, "", response.Data)
	}
	return c.JSON2(StatusOK_V3, "", nil)
}

// 获取申请大神手机验证码
func (gg *GodGame) Code(c frame.Context) error {
	var req godgamepb.CodeReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	} else if req.GetPhone() == "" {
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "请输入手机号", nil)
	} else if err = gg.dao.SendApplyCode(req.GetPhone()); err != nil {
		c.Errorf("%s", err.Error())
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "请稍后再试", nil)
	}
	return c.JSON2(StatusOK_V3, "", nil)
}

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
	} else if req.GetPhone() == "" {
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "手机号不能为空", nil)
	}
	if req.GetValidateCode() != "" {
		if gg.cfg.Env == iconfig.ENV_PROD && !gg.dao.CheckApplyCode(req.GetValidateCode(), req.GetPhone()) {
			return c.JSON2(ERR_CODE_DISPLAY_ERROR, "验证码无效", nil)
		}
	}
	if oldGod := gg.dao.GetGodByPhone(req.GetPhone()); oldGod != nil && oldGod.UserID != currentUser.UserID {
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "手机号已被注册", nil)
	}
	if gg.dao.IsGod(currentUser.UserID) {
		return c.JSON2(ERR_CODE_FORBIDDEN, "您已经是大神身份，请勿重复申请", nil)
	} else if oldGod := gg.dao.GetGodByIDCard(req.GetIdcard()); oldGod != nil && oldGod.UserID != currentUser.UserID {
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "身份证号码已被注册", nil)
	}
	gender, birthday, err := GetGenderAndBirthdayByIDCardNumber(req.GetIdcard())
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
	} else if len(req.GetPowers()) > 4 {
		return c.JSON2(ERR_CODE_DISPLAY_ERROR, "实力照片最多4张", nil)
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
	if len(req.Powers) > 0 {
		bs, err = json.Marshal(req.Powers)
		if err != nil {
			c.Error("GodGameApply error:%s. powers:%v", err, req.Powers)
			return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
		}
		apply.Powers = string(bs)
	}
	oldData, err := gg.dao.GetOldData(currentUser.UserID, req.GetGameId())
	if err == nil {
		apply.Video = oldData.Video
		apply.Videos = oldData.Videos
	}
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
