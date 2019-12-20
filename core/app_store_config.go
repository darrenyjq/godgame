package core

import (
	"laoyuegou.pb/chatroom/model"
	"strconv"
)

// ios上架审核
func (dao *Dao) GetAppStoreConfig(appType, appVersion string) (arr map[int64]int64, err error) {
	var AppStoreConfig []model.AppStoreConfig
	// action 位置:1、性别 2、派单 3、分类 4、推荐位
	// status 开关 0正常，1开启屏蔽
	// type 平台类型1捞月狗 2兔兔 3偷星猫
	// cid 品类id
	// version_number 格式化提审版本号  30007
	// version   api格式化提审版本号  3.1.1
	db := dao.dbr.Table("app_store_config").Select("id")
	if err := db.
		Where("action=?", 3).
		Where("status=?", 1).
		Where("type=?", appType).
		Where("version=?", appVersion).Find(&AppStoreConfig).Error; err != nil {
		return nil, err
	}
	arr = make(map[int64]int64)
	for _, v := range AppStoreConfig {
		if gameId, err := strconv.ParseInt(v.Cid, 10, 64); err == nil {
			arr[gameId] = v.Status
		}
	}
	return arr, nil
}
