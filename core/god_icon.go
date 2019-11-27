package core

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	"laoyuegou.pb/godgame/model"
	"time"
)

// 新增大神认证图片
func (dao *Dao) AddGodIcon(godIcon model.GodIcon) (*model.GodIcon, error) {
	godIcon.Createdtime = time.Now()
	godIcon.Status = 2
	err := dao.dbw.Create(&godIcon).Error
	if err != nil {
		return nil, err
	}
	c := dao.Cpool.Get()
	defer c.Close()
	c.Do("HSET", RKGodIcons(), godIcon.ID, godIcon.Url)
	return &godIcon, nil
}

// 停用大神认证图片
func (dao *Dao) DisableGodIcon(id int64) error {
	var godIcon model.GodIcon
	dao.dbr.Where("id=?", id).First(&godIcon)
	if godIcon.ID == 0 {
		return fmt.Errorf("not found %d", id)
	} else if godIcon.Status == 1 {
		// 已停用
		return nil
	}
	err := dao.dbw.Table("god_icons").Where("id=?", id).Update("status", 1).Error
	if err != nil {
		return err
	}
	c := dao.Cpool.Get()
	defer c.Close()
	c.Do("HDEL", RKGodIcons(), id)
	return nil
}

// OM后台获取大神认证标签图片列表
func (dao *Dao) GetGodIconList(page int64) ([]*model.GodIcon, error) {
	offset := (page - 1) * 20
	limit := 20
	items := make([]*model.GodIcon, 0, 20)
	err := dao.dbr.Order("status desc").Order("createdtime desc").Offset(offset).Limit(limit).Find(&items).Error
	if err != nil {
		return items, err
	}
	return items, nil
}

// 修改大神认证标签
func (dao *Dao) ModifyGodIcon(godIcon model.GodIcon) (*model.GodIcon, error) {
	err := dao.dbw.Model(&godIcon).Updates(map[string]interface{}{
		"name": godIcon.Name,
		"url":  godIcon.Url,
	}).Error
	if err != nil {
		return nil, err
	}
	return &godIcon, nil
}

// OM后台配置定时展示大神认证标签
func (dao *Dao) SetGodIcon(godID, iconID, begin, end int64) error {
	c := dao.Cpool.Get()
	defer c.Close()
	if url, _ := redis.String(c.Do("HGET", RKGodIcons(), iconID)); url == "" {
		var godIcon model.GodIcon
		if err := dao.dbr.Where("id=? AND status=2", iconID).First(&godIcon).Error; err != nil {
			return fmt.Errorf("无效的标签ID")
		}
		c.Do("HSET", RKGodIcons, iconID, godIcon.Url)
	}
	tmpGodIcon := model.TmpGodIcon{
		ID:    iconID,
		Begin: begin,
		End:   end,
	}
	bs, err := json.Marshal(tmpGodIcon)
	if err != nil {
		return err
	}
	c.Do("SET", RKGodIcon(godID), string(bs), "EX", (end - time.Now().Unix()))
	return nil
}

func (dao *Dao) GetGodIcon(godID int64) (*model.TmpGodIcon, error) {
	var godIcon model.TmpGodIcon
	var err error
	c := dao.Cpool.Get()
	defer c.Close()
	bs, err := redis.Bytes(c.Do("GET", RKGodIcon(godID)))
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bs, &godIcon)
	if err != nil {
		return nil, err
	}
	return &godIcon, nil
}

func (dao *Dao) RemoveGodIcon(godID int64) {
	c := dao.Cpool.Get()
	defer c.Close()
	c.Do("DEL", RKGodIcon(godID))
}
