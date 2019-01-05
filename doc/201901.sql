# 1v1版本
ALTER TABLE `play_god_accept_setting` ADD COLUMN `grab_switch4` int(11) NOT NULL DEFAULT 2 COMMENT '语聊随机模式开关1:开 2:关' AFTER `grab_switch3`;
