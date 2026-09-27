CREATE TABLE IF NOT EXISTS `order_details` (
  `id` int NOT NULL AUTO_INCREMENT,
  `order_no` varchar(20) NOT NULL COMMENT '订单号',
  `product_name` varchar(50) NOT NULL COMMENT '商品名称',
  `category` varchar(30) NOT NULL COMMENT '商品类目',
  `quantity` int NOT NULL COMMENT '数量',
  `unit_price` decimal(10,2) NOT NULL COMMENT '单价',
  `order_date` date NOT NULL COMMENT '下单日期',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=11 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='订单明细表';
