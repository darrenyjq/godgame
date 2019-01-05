package api

import (
	"net/http"
)

const (
	StatusOK_V3                           = 0
	ERR_CODE_DISPLAY_ERROR                = 1
	ERR_CODE_BAD_REQUEST                  = http.StatusBadRequest
	ERR_CODE_NOT_FOUND                    = http.StatusNotFound
	ERR_CODE_FORBIDDEN                    = http.StatusForbidden
	ERR_CODE_INTERNAL                     = http.StatusInternalServerError
	ERR_CODE_GOD_ACCEPT_SETTING_LOAD_FAIL = 5
	ERR_CODE_EMPTY_ACCEPT_SETTING         = 10000 // 开启接单开关，但是接单设置为空，自动跳转到接单设置页面
)

const (
	errLoadCfgMsg               = "获取配置信息失败"
	errParamMsg                 = "参数不合法"
	errCalcMsg                  = "获取价格信息失败"
	errUnSupportRegion          = "不支持的平台大区"
	errUnSupportGame            = "不支持的游戏"
	errUnsupportPWMsg           = "不支持的陪玩类型"
	errLoadGamesMsg             = "加载游戏列表失败"
	errLoadGameMsg              = "加载游戏数据失败"
	errUpdateGameMsg            = "更新游戏配置信息失败"
	errLoadClassMsg             = "加载陪玩分类列表失败"
	errGodAcceptSettingLoadFail = "大神未设置接单"
	errEmptyAcceptSettingMsg    = "请设置接单设置"
)
