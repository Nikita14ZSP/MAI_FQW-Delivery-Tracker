DELETE FROM menu_items
 WHERE category_id IN (
   SELECT id FROM menu_categories
    WHERE name IN ('Закуски', 'Горячее', 'Напитки', 'Десерты')
 );

DELETE FROM menu_categories
 WHERE name IN ('Закуски', 'Горячее', 'Напитки', 'Десерты');
