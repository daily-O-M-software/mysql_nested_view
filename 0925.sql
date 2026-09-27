-- --------------------------------------------------------
-- 主机:                           127.0.0.1
-- 服务器版本:                        8.0.20 - MySQL Community Server - GPL
-- 服务器操作系统:                      Win64
-- HeidiSQL 版本:                  12.11.0.7077
-- --------------------------------------------------------

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET NAMES utf8 */;
/*!50503 SET NAMES utf8mb4 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;


-- 导出 nested_demo 的数据库结构
CREATE DATABASE IF NOT EXISTS `nested_demo` /*!40100 DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci */ /*!80016 DEFAULT ENCRYPTION='N' */;
USE `nested_demo`;

-- 导出  表 nested_demo.order_details 结构
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

-- 正在导出表  nested_demo.order_details 的数据：~10 rows (大约)
INSERT INTO `order_details` (`id`, `order_no`, `product_name`, `category`, `quantity`, `unit_price`, `order_date`) VALUES
	(1, 'SO20260901-001', '无线鼠标', '数码配件', 2, 89.00, '2026-09-01'),
	(2, 'SO20260901-002', '机械键盘', '数码配件', 1, 399.00, '2026-09-01'),
	(3, 'SO20260902-003', '真无线耳机', '数码配件', 3, 149.00, '2026-09-02'),
	(4, 'SO20260902-004', '不锈钢保温杯', '生活日用', 5, 59.00, '2026-09-02'),
	(5, 'SO20260903-005', '洗衣液', '生活日用', 2, 35.90, '2026-09-03'),
	(6, 'SO20260903-006', '抽纸', '生活日用', 10, 12.50, '2026-09-03'),
	(7, 'SO20260904-007', '运动T恤', '服饰鞋包', 4, 129.00, '2026-09-04'),
	(8, 'SO20260904-008', '跑鞋', '服饰鞋包', 1, 459.00, '2026-09-04'),
	(9, 'SO20260905-009', '蓝牙音箱', '数码配件', 2, 259.00, '2026-09-05'),
	(10, 'SO20260905-010', '帆布背包', '服饰鞋包', 3, 169.00, '2026-09-05');

-- 导出  视图 nested_demo.v_category_summary 结构
-- 创建临时表以解决视图依赖性错误
CREATE TABLE `v_category_summary` (
	`category` VARCHAR(1) NOT NULL COMMENT '商品类目' COLLATE 'utf8mb4_0900_ai_ci',
	`total_quantity` DECIMAL(54,0) NULL,
	`total_sales` DECIMAL(64,2) NULL,
	`product_count` BIGINT NOT NULL
);

-- 导出  视图 nested_demo.v_order_detail 结构
-- 创建临时表以解决视图依赖性错误
CREATE TABLE `v_order_detail` (
	`id` INT NOT NULL,
	`order_no` VARCHAR(1) NOT NULL COMMENT '订单号' COLLATE 'utf8mb4_0900_ai_ci',
	`product_name` VARCHAR(1) NOT NULL COMMENT '商品名称' COLLATE 'utf8mb4_0900_ai_ci',
	`category` VARCHAR(1) NOT NULL COMMENT '商品类目' COLLATE 'utf8mb4_0900_ai_ci',
	`quantity` INT NOT NULL COMMENT '数量',
	`unit_price` DECIMAL(10,2) NOT NULL COMMENT '单价',
	`amount` DECIMAL(20,2) NOT NULL,
	`order_date` DATE NOT NULL COMMENT '下单日期'
);

-- 导出  视图 nested_demo.v_product_sales 结构
-- 创建临时表以解决视图依赖性错误
CREATE TABLE `v_product_sales` (
	`product_name` VARCHAR(1) NOT NULL COMMENT '商品名称' COLLATE 'utf8mb4_0900_ai_ci',
	`category` VARCHAR(1) NOT NULL COMMENT '商品类目' COLLATE 'utf8mb4_0900_ai_ci',
	`total_quantity` DECIMAL(32,0) NULL,
	`total_amount` DECIMAL(42,2) NULL,
	`order_count` BIGINT NOT NULL
);

-- 移除临时表并创建最终视图结构
DROP TABLE IF EXISTS `v_category_summary`;
CREATE ALGORITHM=UNDEFINED SQL SECURITY DEFINER VIEW `v_category_summary` AS select `v_product_sales`.`category` AS `category`,sum(`v_product_sales`.`total_quantity`) AS `total_quantity`,sum(`v_product_sales`.`total_amount`) AS `total_sales`,count(0) AS `product_count` from `v_product_sales` group by `v_product_sales`.`category`
;

-- 移除临时表并创建最终视图结构
DROP TABLE IF EXISTS `v_order_detail`;
CREATE ALGORITHM=UNDEFINED SQL SECURITY DEFINER VIEW `v_order_detail` AS select `order_details`.`id` AS `id`,`order_details`.`order_no` AS `order_no`,`order_details`.`product_name` AS `product_name`,`order_details`.`category` AS `category`,`order_details`.`quantity` AS `quantity`,`order_details`.`unit_price` AS `unit_price`,(`order_details`.`quantity` * `order_details`.`unit_price`) AS `amount`,`order_details`.`order_date` AS `order_date` from `order_details`
;

-- 移除临时表并创建最终视图结构
DROP TABLE IF EXISTS `v_product_sales`;
CREATE ALGORITHM=UNDEFINED SQL SECURITY DEFINER VIEW `v_product_sales` AS select `v_order_detail`.`product_name` AS `product_name`,`v_order_detail`.`category` AS `category`,sum(`v_order_detail`.`quantity`) AS `total_quantity`,sum(`v_order_detail`.`amount`) AS `total_amount`,count(0) AS `order_count` from `v_order_detail` group by `v_order_detail`.`product_name`,`v_order_detail`.`category`
;

/*!40103 SET TIME_ZONE=IFNULL(@OLD_TIME_ZONE, 'system') */;
/*!40101 SET SQL_MODE=IFNULL(@OLD_SQL_MODE, '') */;
/*!40014 SET FOREIGN_KEY_CHECKS=IFNULL(@OLD_FOREIGN_KEY_CHECKS, 1) */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40111 SET SQL_NOTES=IFNULL(@OLD_SQL_NOTES, 1) */;
