CREATE ALGORITHM=UNDEFINED SQL SECURITY DEFINER VIEW `v_category_summary` AS select `v_product_sales`.`category` AS `category`,sum(`v_product_sales`.`total_quantity`) AS `total_quantity`,sum(`v_product_sales`.`total_amount`) AS `total_sales`,count(0) AS `product_count` from `v_product_sales` group by `v_product_sales`.`category`
;
