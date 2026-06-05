-- Удаляем старый уникальный индекс
DROP INDEX IF EXISTS idx_users_yandex_id;

-- Удаляем записи с пустым yandex_id
UPDATE users SET yandex_id = NULL WHERE yandex_id = '';

-- Создаем новый уникальный индекс, игнорирующий NULL значения
CREATE UNIQUE INDEX idx_users_yandex_id ON users (yandex_id) WHERE yandex_id IS NOT NULL;