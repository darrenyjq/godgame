 alter table play_god_accept_setting add column `grab_switch5` int(11) not null  DEFAULT '2'  comment '急速接单开关  1:开启 2:关闭';

 alter table play_god_accept_setting add column `price_discount` float not null  DEFAULT '1'  comment '单价折扣 ';