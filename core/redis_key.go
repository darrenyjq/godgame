package core

import (
	"fmt"
)

// RKFillGodListLock 填充大神列表锁
func RKFillGodListLock() string {
	return "FillGodListLock"
}

// .RKGodListByGender 大神列表按性别划分
func RKGodListByGender(gameID, gender int64) string {
	return fmt.Sprintf("GL:{%d}:G:{%d}", gameID, gender)
}

// RKGodListByNew 大神列表按新鲜度划分
func RKGodListByNew(gameID int64) string {
	return fmt.Sprintf("GLN:{%d}", gameID)
}

// RKGodListByOrderCnt 大神列表按接单数划分
func RKGodListByOrderCnt(gameID int64) string {
	return fmt.Sprintf("GLO:{%d}", gameID)
}

func RKFeedRooms() string {
	return "FeedRooms"
}

// title为推荐栏标题的md5值
func RKFeedGods(title string) string {
	return fmt.Sprintf("FG:{%s}", title)
}

// gender 1:男 2:女 0:所有
func RKFeedGodsByGender(gameID, gender int64) string {
	return fmt.Sprintf("G:{%d}:FG:{%d}", gameID, gender)
}

// 语聊大神列表，Sorted Set，随机开关打开：score=1 随机开关关闭：score=2
func RKVoiceCallGods() string {
	return "VoiceGods"
}

// 申请大神时，手机验证码
func RKAuthCodeForPhone(phone string) string {
	return fmt.Sprintf("PH:{%s}:AUTH", phone)
}

// 整体时间线
func RKFeedTimeLine() string {
	return "Global:Timeline"
}

// 大神最后一次修改自定义介绍的时间
func RKGodLastModifyDesc(godID int64) string {
	return fmt.Sprintf("God:{%d}:LMD", godID)
}

// 大神认证标签图片，Hash  id:url
func RKGodIcons() string {
	return "GodIcons"
}

// 某个大神的认证图标
func RKGodIcon(godID int64) string {
	return fmt.Sprintf("God:{%d}:Icon", godID)
}

// 大神信息
func RKGodInfo(godID int64) string {
	return fmt.Sprintf("God:{%d}", godID)
}

// 大神陪玩品类信息
func RKGodGameInfo(godID, gameID int64) string {
	return fmt.Sprintf("God:{%d}:Game:{%d}", godID, gameID)
}

func RKOneGodGameV1(godID, gameID int64) string {
	return fmt.Sprintf("GGV1:{%d}:{%d}", godID, gameID)
}

// 大神品类申请缓存
func RKGodGameApply(godID, gameID int64) string {
	return fmt.Sprintf("GGA:{%d}:{%d}", godID, gameID)
}

// DaiLianSuperGods sorted set 代练大神列表
func DaiLianSuperGods() string {
	return "PL_dalian_gods"
}

// 段位正在接单的大神集合，set
func GodsRedisKey(game, region, acceptID int64) string {
	return fmt.Sprintf("PL_Game[%d]:Region[%d]:LevelId[%d]:Gods2", game, region, acceptID)
}

// 段位正在接单的大神集合，sorted set score：大神最高段位 value：大神ID
func GodsRedisKey3(game, region, acceptID int64) string {
	return fmt.Sprintf("PL_Game[%d]:Region[%d]:LevelId[%d]:Gods3", game, region, acceptID)
}

// 大神接单设置
// 老的是PL_god_ordersetting_v2，由于结构变更，新的改成v3
func GodAcceptOrderSettingKey(godID int64) string {
	return fmt.Sprintf("PL_god_ordersetting_v3[%d]", godID)
}

// 即时约正在接单的大神集合，按游戏+性别分组
func RKJSYGods(gameID, gender int64) string {
	return fmt.Sprintf("JSY:{%d}:{%d}:Gods", gameID, gender)
}

// 即时约正在接派单的大神集合，按游戏+性别分组
func RKJSYPaiDanGods(gameID, gender int64) string {
	return fmt.Sprintf("JSYPaiDan:{%d}:{%d}:Gods", gameID, gender)
}

// GodConfig 大神配置
func GodConfig(godID int64) string {
	return fmt.Sprintf("GOD:{%d}:Config", godID)
}

// 陪玩首页权重
func RKUpperGodGames() string {
	return "PW_List_Up_God_Games"
}

// 会长信息缓存
func RKGodLeaderInfo(leaderID int64) string {
	return fmt.Sprintf("Leader:{%d}", leaderID)
}

// 大神上一次游戏资料修改成功的时间，用于处理每周只可修改一次的限制
func RKLastModifyInfoDate(godID int64) string {
	return fmt.Sprintf("LMD:{%d}", godID)
}

// RKSimpleGodGamesKey 大神当前正在接单的品类简介，主要是给Feed使用
func RKSimpleGodGamesKey(godID int64) string {
	return fmt.Sprintf("SGG:{%d}", godID)
}

func RKQuickOrder(godId, game_id int64) string {
	return fmt.Sprintf("QUICK_ORDER:{%d}:{$d}", godId, game_id)
}

// 急速接单后台配置key
func RKGameQuickOrder() string {
	return "GAME:QUICK_ORDER"
}

// 私聊消息标识
func RKChatTimes(userId int64) string {
	return fmt.Sprintf("IM_CHAT_TIMES:{%d}", userId)
}

// 已经抢单的大神id集合
func RKGrabBedGodsOfBoss(userId int64) string {
	return fmt.Sprintf("PHP:God:{%d}:AutoGrabGames", userId)
}

// 自动抢单的大神 games集合
func RKGrabOrderGames(userId int64) string {
	return fmt.Sprintf("PHP:QuickOrder:Boss:{%d}:GrabbedGods", userId)
}
