# 1v1版本
ALTER TABLE `play_god_accept_setting` ADD COLUMN `grab_switch4` int(11) NOT NULL DEFAULT 2 COMMENT '语聊随机模式开关1:开 2:关' AFTER `grab_switch3`;

# 首页改版
DROP TABLE IF EXISTS `play_game_account`;
CREATE TABLE `play_game_account`(
    `id` int(11) NOT NULL AUTO_INCREMENT PRIMARY KEY,
    `user_id` int(11) NOT NULL,
    `game_id` int(11) NOT NULL,
    `region_id` int(11) NOT NULL,
    `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `update_time` datetime NOT NULL ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY `uni_user_game_region`(`user_id`, `game_id`, `region_id`),
    KEY `idx_create_time`(`create_time`),
    KEY `idx_update_time`(`update_time`)
) ENGINE=InnoDB AUTO_INCREMENT=1 DEFAULT CHARSET=utf8mb4;
