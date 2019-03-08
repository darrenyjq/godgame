package api

import (
	"iceberg/frame"
	"laoyuegou.pb/game/pb"
	"laoyuegou.pb/godgame/model"
	"laoyuegou.pb/godgame/pb"
)

func (self *GodGame) GameAccount(c frame.Context) error {
	var req godgamepb.GameAccountReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.RetBadRequestError(err.Error())
	}
	account, err := self.dao.AddGameAccount(model.GameAccount{
		ID:       req.GetId(),
		UserID:   self.getCurrentUserID(c),
		GameID:   req.GetGameId(),
		RegionID: req.GetRegionId(),
		Account:  req.GetAccount(),
	})
	if err != nil {
		c.Error(err.Error())
		return c.RetInternalError("")
	}
	return c.RetSuccess("", account)
}

func (self *GodGame) GameAccounts(c frame.Context) error {
	accounts, err := self.dao.GetUserGameAccounts(self.getCurrentUserID(c))
	if err != nil {
		c.Error(err.Error())
		return c.RetSuccess("", nil)
	}
	items := make([]*godgamepb.GameAccountsResp_Data_Item, 0, len(accounts))
	for _, account := range accounts {
		resp, err := gamepb.Desc3(c, &gamepb.Desc3Req{
			GameId:   account.GameID,
			RegionId: account.RegionID,
		})
		if err != nil || resp.GetErrcode() != 0 {
			continue
		}
		items = append(items, &godgamepb.GameAccountsResp_Data_Item{
			Id:       account.ID,
			GameId:   account.GameID,
			RegionId: account.RegionID,
			Title:    resp.GetData(),
			Account:  account.Account,
		})
	}
	return c.RetSuccess("", &godgamepb.GameAccountsResp_Data{
		Count: int64(len(items)),
		Items: items,
	})
}

func (self *GodGame) DelGameAccount(c frame.Context) error {
	var req godgamepb.DelGameAccountReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.RetBadRequestError(err.Error())
	} else if req.GetId() == 0 {
		return c.RetBadRequestError("invalid id")
	}
	err = self.dao.DelUserGameAccounts(req.GetId(), self.getCurrentUserID(c))
	if err != nil {
		c.Error(err.Error())
		return c.RetInternalError("")
	}
	return c.RetSuccess("", nil)
}
