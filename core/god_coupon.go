package core

import "laoyuegou.pb/godgame/model"

func (dao *Dao) GetGodCouponConfig(godID int64) (coupon []model.GodCoupon, err error) {
	// c := dao.cpool.Get()
	// defer c.Close()
	// key :=
	err1 := dao.dbr.Table("god_coupon_config").
		Where("god_id=?", godID).
		Find(&coupon).Error
	if err1 == nil {
		return coupon, nil
	}
	return coupon, err
}
