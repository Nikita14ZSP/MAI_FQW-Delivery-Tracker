-- Seed menu catalog: 4 categories + 14 items.
-- Idempotency: keyed off category names; safe to re-run after down migration.

INSERT INTO menu_categories (name, sort_order) VALUES
  ('Закуски', 1),
  ('Горячее', 2),
  ('Напитки', 3),
  ('Десерты', 4);

-- Закуски (3 items)
INSERT INTO menu_items (category_id, name, price, image_url, available)
SELECT id, 'Цезарь с курицей', 390,
  'https://images.unsplash.com/photo-1550304943-4f24f54ddde9?w=400', true
FROM menu_categories WHERE name = 'Закуски';

INSERT INTO menu_items (category_id, name, price, image_url, available)
SELECT id, 'Том Ям', 450,
  'https://images.unsplash.com/photo-1548943487-a2e4e43b4853?w=400', true
FROM menu_categories WHERE name = 'Закуски';

INSERT INTO menu_items (category_id, name, price, image_url, available)
SELECT id, 'Брускетта', 280,
  'https://images.unsplash.com/photo-1572695157366-5e585ab2b69f?w=400', true
FROM menu_categories WHERE name = 'Закуски';

-- Горячее (5 items)
INSERT INTO menu_items (category_id, name, price, image_url, available)
SELECT id, 'Пицца Маргарита', 520,
  'https://images.unsplash.com/photo-1574071318508-1cdbab80d002?w=400', true
FROM menu_categories WHERE name = 'Горячее';

INSERT INTO menu_items (category_id, name, price, image_url, available)
SELECT id, 'Бургер классик', 480,
  'https://images.unsplash.com/photo-1568901346375-23c9450c58cd?w=400', true
FROM menu_categories WHERE name = 'Горячее';

INSERT INTO menu_items (category_id, name, price, image_url, available)
SELECT id, 'Паста карбонара', 440,
  'https://images.unsplash.com/photo-1588013273468-315fd88ea34c?w=400', true
FROM menu_categories WHERE name = 'Горячее';

INSERT INTO menu_items (category_id, name, price, image_url, available)
SELECT id, 'Стейк из говядины', 890,
  'https://images.unsplash.com/photo-1546964124-0cce460f38ef?w=400', true
FROM menu_categories WHERE name = 'Горячее';

INSERT INTO menu_items (category_id, name, price, image_url, available)
SELECT id, 'Гречневая каша с котлетой', 320,
  'https://images.unsplash.com/photo-1673139140820-039ad8bc2c0e?w=400', true
FROM menu_categories WHERE name = 'Горячее';

-- Напитки (3 items)
INSERT INTO menu_items (category_id, name, price, image_url, available)
SELECT id, 'Кола 0.5л', 120,
  'https://images.unsplash.com/photo-1648569883125-d01072540b4c?w=400', true
FROM menu_categories WHERE name = 'Напитки';

INSERT INTO menu_items (category_id, name, price, image_url, available)
SELECT id, 'Апельсиновый сок', 180,
  'https://images.unsplash.com/photo-1641659735894-45046caad624?w=400', true
FROM menu_categories WHERE name = 'Напитки';

INSERT INTO menu_items (category_id, name, price, image_url, available)
SELECT id, 'Вода негазированная', 80,
  'https://images.unsplash.com/photo-1602143407151-7111542de6e8?w=400', true
FROM menu_categories WHERE name = 'Напитки';

-- Десерты (3 items)
INSERT INTO menu_items (category_id, name, price, image_url, available)
SELECT id, 'Тирамису', 290,
  'https://images.unsplash.com/photo-1714385905983-6f8e06fffae1?w=400', true
FROM menu_categories WHERE name = 'Десерты';

INSERT INTO menu_items (category_id, name, price, image_url, available)
SELECT id, 'Чизкейк Нью-Йорк', 310,
  'https://images.unsplash.com/photo-1565958011703-44f9829ba187?w=400', true
FROM menu_categories WHERE name = 'Десерты';

INSERT INTO menu_items (category_id, name, price, image_url, available)
SELECT id, 'Мороженое ванильное', 160,
  'https://images.unsplash.com/photo-1560008581-09826d1de69e?w=400', true
FROM menu_categories WHERE name = 'Десерты';
