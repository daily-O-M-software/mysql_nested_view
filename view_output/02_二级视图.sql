CREATE ALGORITHM=UNDEFINED SQL SECURITY DEFINER VIEW `v_product_sales` AS select `v_order_detail`.`product_name` AS `product_name`,`v_order_detail`.`category` AS `category`,sum(`v_order_detail`.`quantity`) AS `total_quantity`,sum(`v_order_detail`.`amount`) AS `total_amount`,count(0) AS `order_count` from `v_order_detail` group by `v_order_detail`.`product_name`,`v_order_detail`.`category`
;
