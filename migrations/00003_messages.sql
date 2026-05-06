-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS messages (
    key        TEXT PRIMARY KEY,
    text       TEXT NOT NULL,
    image_url  TEXT,
    buttons    JSONB,
    updated_at TIMESTAMPTZ DEFAULT now()
);

INSERT INTO messages (key, text, buttons) VALUES
    ('welcome',
     '👋 Привет! Я бот для создания нейрофотосессий.\n\nЯ превращу твои фото в профессиональные снимки с помощью AI.\n\nГотов попробовать?',
     '[{"label":"✨ Попробовать бесплатно","payload":"free_gen"},{"label":"🎁 Реферальная программа","payload":"referral"}]'),

    ('start_menu',
     '👋 Добро пожаловать! Выбери действие:',
     '[{"label":"✨ Попробовать бесплатно","payload":"free_gen"},{"label":"🎁 Реферальная программа","payload":"referral"}]'),

    ('free_gen_start',
     '✨ Отлично! Давай создадим твою первую нейрофотосессию!\n\nСначала нужно подписаться на нашу группу — это обязательное условие.',
     NULL),

    ('subscribe_cta',
     '📌 Подпишись на нашу группу и возвращайся!\n\nПосле подписки нажми кнопку «Я подписался»',
     '[{"label":"✅ Я подписался","payload":"check_sub"},{"label":"◀️ Назад","payload":"back"}]'),

    ('subscribed_check',
     '🔍 Проверяю подписку...',
     NULL),

    ('photo_requirements',
     '📸 Отлично! Теперь загрузи своё фото.\n\n✅ Требования к фото:\n• Лицо должно быть хорошо видно\n• Хорошее освещение\n• Без очков и головных уборов\n• Один человек на фото\n\nОтправь фото прямо в чат:',
     NULL),

    ('generating_wait',
     '⏳ Генерирую твоё фото...\n\nОбычно это занимает 1–2 минуты. Пожалуйста, подожди!',
     NULL),

    ('gen_result_warmup',
     '🎉 Вот твоя нейрофотосессия!\n\nХочешь ещё? У меня есть множество крутых стилей!',
     '[{"label":"✨ Ещё одно фото","payload":"free_gen"},{"label":"🚀 Активировать все функции","payload":"tariffs"},{"label":"◀️ Назад","payload":"back"}]'),

    ('referral_info',
     '🎁 Реферальная программа\n\nПриглашай друзей и получай бонусные генерации!\n\n• За каждого другa, который оплатит тариф — ты получишь 2 бесплатные генерации\n\nТвоя ссылка для приглашения:',
     NULL),

    ('tariffs',
     '💳 Выбери тариф для продолжения:',
     NULL),

    ('payment_success',
     '🎉 Оплата прошла успешно!\n\nДобро пожаловать в мир нейрофотосессий! Твои генерации зачислены.',
     '[{"label":"🎨 Начать генерацию","payload":"main_menu"}]'),

    ('main_menu',
     '🎨 Главное меню\n\nВыбери что хочешь сделать:',
     '[{"label":"🖼 Готовые промты","payload":"ready_prompts"},{"label":"✍️ Свой промт","payload":"custom_prompt"},{"label":"✏️ Изменить фото","payload":"edit_photo"},{"label":"👫 Парное фото","payload":"couple"},{"label":"💾 Запомнить фото","payload":"saved_photo"},{"label":"⚙️ Настройки","payload":"settings"}]'),

    ('ready_prompts_intro',
     '🖼 Готовые стили\n\nВыбери категорию:',
     NULL),

    ('custom_prompt_intro',
     '✍️ Опиши желаемый стиль фотосессии своими словами.\n\nПример: «деловой костюм на фоне небоскрёбов, профессиональное освещение»\n\nВведи описание:',
     NULL),

    ('edit_photo_intro',
     '✏️ Изменить фото\n\nОтправь фото которое хочешь изменить, и опиши что нужно сделать:',
     NULL),

    ('couple_intro',
     '👫 Парные и семейные фото\n\nОтправь фото двух людей для создания совместной нейрофотосессии:',
     NULL),

    ('saved_photo_intro',
     '💾 Запомнить моё фото\n\nОтправь фото, которое я буду использовать по умолчанию для всех генераций:',
     NULL),

    ('examples_collage',
     '🌟 Примеры наших работ',
     NULL),

    ('support_text',
     '📞 Техподдержка\n\nЕсли у вас возникли вопросы или проблемы — напишите нам:\n@support_username',
     NULL),

    ('gender_select',
     '👤 Выбери свой пол — это поможет подобрать подходящие стили:',
     '[{"label":"👨 Мужской","payload":"gender_male"},{"label":"👩 Женский","payload":"gender_female"}]'),

    ('no_gens_left',
     '😔 У тебя закончились генерации.\n\nПополни баланс для продолжения!',
     '[{"label":"💳 Выбрать тариф","payload":"tariffs"},{"label":"◀️ Назад","payload":"back"}]')

ON CONFLICT (key) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS messages;
