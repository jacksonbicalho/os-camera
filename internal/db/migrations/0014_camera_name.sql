ALTER TABLE cameras ADD COLUMN name TEXT NOT NULL DEFAULT '';
UPDATE cameras SET name = id WHERE name = '';
