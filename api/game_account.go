package api

import (
	"iceberg/frame"
	"laoyuegou.pb/godgame/pb"
)

func (self *GodGame) GameAccount(c frame.Context) error {
	var req godgamepb.GameAccountReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.RetBadRequestError(err.Error())
	}
	return c.RetSuccess("", nil)
}

func (self *GodGame) GameAccounts(c frame.Context) error {

	return c.RetSuccess("", &godgamepb.GameAccountsResp_Data{
		Count: 10,
		Items: []*godgamepb.GameAccountsResp_Data_Item{
			&godgamepb.GameAccountsResp_Data_Item{
				Id:      1,
				Title:   "title1",
				Account: "account1",
			},
			&godgamepb.GameAccountsResp_Data_Item{
				Id:      2,
				Title:   "title2",
				Account: "account2",
			},
			&godgamepb.GameAccountsResp_Data_Item{
				Id:      3,
				Title:   "title3",
				Account: "account3",
			},
		},
	})
}

func (self *GodGame) DelGameAccount(c frame.Context) error {

	return c.RetSuccess("", nil)
}
