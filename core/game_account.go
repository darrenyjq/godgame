package core

import (
	"laoyuegou.pb/godgame/model"
	"time"
)

func (dao *Dao) AddGameAccount(account model.GameAccount) (*model.GameAccount, error) {
	var err error
	account.Updatedtime = time.Now()
	if account.ID > 0 {
		err = dao.dbw.Save(&account).Error
	} else {
		account.Createdtime = time.Now()
		err = dao.dbw.Create(&account).Error
	}
	return &account, err
}

func (dao *Dao) GetUserGameAccounts(userID int64) ([]model.GameAccount, error) {
	var err error
	var accounts []model.GameAccount
	err = dao.dbr.Where("user_id=?", userID).Order("update_time desc").Find(&accounts).Error
	return accounts, err
}

func (dao *Dao) DelUserGameAccounts(id, userID int64) error {
	return dao.dbw.Where("id=? AND user_id=?", id, userID).Delete(model.GameAccount{}).Error
}
