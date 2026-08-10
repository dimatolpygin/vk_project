package content

import (
	"sort"
	"strings"
	"unicode"
)

type ScreenSection struct {
	ID          string
	Title       string
	Description string
	Icon        string
	Order       int
}

type ScreenAdminMeta struct {
	SectionID   string
	Title       string
	Description string
	Order       int
	Keywords    []string
}

var screenSections = []ScreenSection{
	{ID: "onboarding", Title: "Онбординг", Description: "Первое знакомство, подписка и старт генерации.", Icon: "bi-stars", Order: 10},
	{ID: "navigation", Title: "Навигация", Description: "Главное меню и постоянные точки входа.", Icon: "bi-compass", Order: 20},
	{ID: "generation", Title: "Сценарии генерации", Description: "Экшены вокруг создания и редактирования фото.", Icon: "bi-magic", Order: 30},
	{ID: "styles", Title: "Готовые стили", Description: "Экраны выбора категорий и шаблонов.", Icon: "bi-grid-1x2", Order: 40},
	{ID: "saved_photo", Title: "Сохранённое фото", Description: "Сохранение базового фото и его использование.", Icon: "bi-bookmark-heart", Order: 50},
	{ID: "settings", Title: "Настройки", Description: "Баланс, модель, качество и формат генерации.", Icon: "bi-sliders2", Order: 60},
	{ID: "billing", Title: "Оплата", Description: "Тарифы, покупка генераций и успешная оплата.", Icon: "bi-credit-card-2-front", Order: 70},
	{ID: "referrals", Title: "Рефералы", Description: "Партнёрская программа и бонусные генерации.", Icon: "bi-gift", Order: 80},
	{ID: "system", Title: "Системные", Description: "Ошибки, пустые состояния и технические сообщения.", Icon: "bi-shield-exclamation", Order: 90},
	{ID: "other", Title: "Прочее", Description: "Экраны без явной категории.", Icon: "bi-three-dots", Order: 100},
}

var screenSectionsByID = func() map[string]ScreenSection {
	out := make(map[string]ScreenSection, len(screenSections))
	for _, section := range screenSections {
		out[section.ID] = section
	}
	return out
}()

var screenAdminMeta = map[string]ScreenAdminMeta{
	"welcome": {
		SectionID:   "onboarding",
		Title:       "Приветственный экран",
		Description: "Первое сообщение бота с CTA на бесплатный старт и реферальную программу.",
		Order:       10,
		Keywords:    []string{"старт", "приветствие", "welcome"},
	},
	"subscribe_cta": {
		SectionID:   "onboarding",
		Title:       "Проверка подписки",
		Description: "Экран с просьбой подписаться на группу и подтвердить подписку.",
		Order:       20,
		Keywords:    []string{"подписка", "группа", "cta"},
	},
	"photo_requirements": {
		SectionID:   "onboarding",
		Title:       "Требования к фото",
		Description: "Инструкция перед загрузкой фото пользователя. Подсказывает, что можно отправить от 1 до 6 фото одним сообщением.",
		Order:       30,
		Keywords:    []string{"инструкция", "требования", "загрузка"},
	},
	"gender_select": {
		SectionID:   "onboarding",
		Title:       "Выбор пола",
		Description: "Экран выбора пола для подбора подходящих стилей.",
		Order:       40,
		Keywords:    []string{"пол", "мужской", "женский"},
	},
	"main_menu": {
		SectionID:   "navigation",
		Title:       "Главное меню",
		Description: "Основной экран навигации по возможностям бота после входа.",
		Order:       10,
		Keywords:    []string{"главное меню", "разделы", "навигация"},
	},
	"bottom_menu": {
		SectionID:   "navigation",
		Title:       "Нижнее постоянное меню",
		Description: "Reply-клавиатура, доступная пользователю внизу чата постоянно.",
		Order:       20,
		Keywords:    []string{"нижнее меню", "reply keyboard", "постоянные кнопки"},
	},
	"support_text": {
		SectionID:   "navigation",
		Title:       "Техподдержка",
		Description: "Контактный экран для связи с поддержкой.",
		Order:       30,
		Keywords:    []string{"поддержка", "support", "контакты"},
	},
	"examples_collage": {
		SectionID:   "navigation",
		Title:       "Примеры работ: меню",
		Description: "Экран-меню раздела примеров: шесть категорий и возврат назад.",
		Order:       40,
		Keywords:    []string{"примеры", "коллаж", "портфолио", "меню"},
	},
	"examples_self": {
		SectionID:   "navigation",
		Title:       "Примеры: фото для себя",
		Description: "Примеры одиночных портретов. Картинка-коллаж загружается в этот экран.",
		Order:       41,
		Keywords:    []string{"примеры", "портрет", "для себя"},
	},
	"examples_couple": {
		SectionID:   "navigation",
		Title:       "Примеры: парные фото",
		Description: "Примеры парных и семейных фотосессий. Картинка-коллаж загружается в этот экран.",
		Order:       42,
		Keywords:    []string{"примеры", "пара", "семья"},
	},
	"examples_kids": {
		SectionID:   "navigation",
		Title:       "Примеры: детские фото",
		Description: "Примеры детских фотосессий. Картинка-коллаж загружается в этот экран.",
		Order:       43,
		Keywords:    []string{"примеры", "дети", "детские"},
	},
	"examples_edit": {
		SectionID:   "navigation",
		Title:       "Примеры: изменение фото",
		Description: "Примеры до/после для режима редактирования фото. Картинка-коллаж загружается в этот экран.",
		Order:       44,
		Keywords:    []string{"примеры", "редактирование", "до после"},
	},
	"examples_greetings": {
		SectionID:   "navigation",
		Title:       "Примеры: поздравления",
		Description: "Примеры праздничных открыток и поздравлений. Картинка-коллаж загружается в этот экран.",
		Order:       45,
		Keywords:    []string{"примеры", "поздравления", "открытки"},
	},
	"examples_misc": {
		SectionID:   "navigation",
		Title:       "Примеры: разное",
		Description: "Прочие примеры работ. Картинка-коллаж загружается в этот экран.",
		Order:       46,
		Keywords:    []string{"примеры", "разное", "прочее"},
	},
	"custom_prompt_intro": {
		SectionID:   "generation",
		Title:       "Свой промт",
		Description: "Вводный экран перед созданием фото по собственному описанию. Пользователь может прислать описание отдельно или сразу вместе с 1–6 фото.",
		Order:       10,
		Keywords:    []string{"свой промт", "кастом", "описание"},
	},
	"edit_photo_intro": {
		SectionID:   "generation",
		Title:       "Изменить фото",
		Description: "Входной экран для режима редактирования существующего фото или группы до 6 фото. Описание правок можно прислать сразу в том же сообщении.",
		Order:       20,
		Keywords:    []string{"редактирование", "изменить фото", "edit"},
	},
	"edit_result_prompt": {
		SectionID:   "generation",
		Title:       "Описание правок",
		Description: "Экран, где пользователь пишет, что нужно поменять на фото.",
		Order:       30,
		Keywords:    []string{"правки", "описание изменений", "редактирование"},
	},
	"free_gen_prompt": {
		SectionID:   "generation",
		Title:       "Промпт первой бесплатной генерации",
		Description: "Системный AI-промпт для первой бесплатной генерации после загрузки фото. Можно использовать шаблоны {{.GenderLabel}} и {{.Gender}}.",
		Order:       35,
		Keywords:    []string{"free generation prompt", "wavespeed", "ai prompt", "первая генерация"},
	},
	"couple_intro": {
		SectionID:   "generation",
		Title:       "Парные и семейные фото",
		Description: "Интро-экран для загрузки от 1 до 6 фото пары или семьи. Категории показываются только после загрузки фото.",
		Order:       40,
		Keywords:    []string{"пара", "семья", "совместное фото"},
	},
	"couple_submenu": {
		SectionID:   "generation",
		Title:       "Парные фото: тип съёмки",
		Description: "Меню сразу после загрузки фото: парное фото, семейное фото, фото поколений. Пункты меняются в разделе «Категории и промты».",
		Order:       41,
		Keywords:    []string{"пара", "семья", "поколения", "подменю"},
	},
	"couple_categories": {
		SectionID:   "generation",
		Title:       "Парные фото: категории",
		Description: "Экран выбора категории внутри «Парного фото». Появляется после выбора типа съёмки.",
		Order:       42,
		Keywords:    []string{"пара", "семья", "категории", "совместное фото"},
	},
	"kids_intro": {
		SectionID:   "generation",
		Title:       "Детские фото: вход в раздел",
		Description: "Экран раздела «Детские фото»: картинка, текст и кнопки подразделов. Сами подразделы редактируются в «Категории и промты».",
		Order:       43,
		Keywords:    []string{"дети", "ребёнок", "мальчик", "девочка"},
	},
	"greetings_intro": {
		SectionID:   "generation",
		Title:       "Поздравления: вход в раздел",
		Description: "Экран выбора адресата поздравления. Сами адресаты и праздники редактируются в «Категории и промты».",
		Order:       44,
		Keywords:    []string{"поздравления", "открытка", "адресат", "праздник"},
	},
	"greetings_holidays": {
		SectionID:   "generation",
		Title:       "Поздравления: список праздников",
		Description: "Экран со списком праздников выбранного адресата.",
		Order:       45,
		Keywords:    []string{"поздравления", "праздник", "новый год", "день рождения"},
	},
	"trends_intro": {
		SectionID:   "generation",
		Title:       "Тренды: вход в раздел",
		Description: "Экран раздела «Тренды». Видео-подразделы заведены в «Категориях и промтах», но выключены — включатся вместе с видео-генерацией.",
		Order:       46,
		Keywords:    []string{"тренды", "популярное", "фото тренды"},
	},
	"video_generating_wait": {
		SectionID:   "generation",
		Title:       "Видео: ожидание",
		Description: "Экран сразу после запуска видео. Доступны {{.PromptName}} и {{.CostGens}} — название тренда и сколько генераций списано.",
		Order:       47,
		Keywords:    []string{"видео", "ожидание", "тренды", "seedance"},
	},
	"video_scene_ready": {
		SectionID:   "generation",
		Title:       "Видео: кадр собран",
		Description: "Промежуточное сообщение между двумя моделями: кадр готов, идёт анимация.",
		Order:       48,
		Keywords:    []string{"видео", "кадр", "сцена", "ожидание"},
	},
	"after_gen_video": {
		SectionID:   "generation",
		Title:       "Видео: результат",
		Description: "Финальный экран с готовым видео. Доступно {{.Duration}} — длительность в секундах.",
		Order:       49,
		Keywords:    []string{"видео", "результат", "готово"},
	},
	"generating_wait": {
		SectionID:   "generation",
		Title:       "Ожидание генерации",
		Description: "Экран ожидания, пока сервис генерирует результат.",
		Order:       50,
		Keywords:    []string{"ожидание", "генерация", "loading"},
	},
	"after_gen_free": {
		SectionID:   "generation",
		Title:       "Результат бесплатной генерации",
		Description: "Финальный экран после бесплатного фото с CTA на повтор и активацию тарифа.",
		Order:       60,
		Keywords:    []string{"бесплатно", "результат", "повтор"},
	},
	"after_gen_paid": {
		SectionID:   "generation",
		Title:       "Результат платной генерации",
		Description: "Финальный экран платной генерации со скачиванием и повтором.",
		Order:       70,
		Keywords:    []string{"платно", "скачать", "результат"},
	},
	"ready_prompts_intro": {
		SectionID:   "styles",
		Title:       "Категории готовых стилей",
		Description: "Экран входа в библиотеку готовых промтов.",
		Order:       10,
		Keywords:    []string{"готовые стили", "категории", "промты"},
	},
	"prompts_list": {
		SectionID:   "styles",
		Title:       "Список стилей",
		Description: "Экран выбора конкретного шаблона внутри категории.",
		Order:       20,
		Keywords:    []string{"список стилей", "шаблон", "выбор промта"},
	},
	"saved_photo_empty": {
		SectionID:   "saved_photo",
		Title:       "Сохранённое фото: пустое состояние",
		Description: "Экран, когда у пользователя ещё нет базового сохранённого фото.",
		Order:       10,
		Keywords:    []string{"сохранённое фото", "пусто", "загрузить"},
	},
	"saved_photo_filled": {
		SectionID:   "saved_photo",
		Title:       "Сохранённое фото: карточка",
		Description: "Основной экран сохранённого фото с переключателем использования.",
		Order:       20,
		Keywords:    []string{"сохранённое фото", "переключатель", "статус"},
	},
	"saved_photo_saved": {
		SectionID:   "saved_photo",
		Title:       "Фото сохранено",
		Description: "Подтверждение успешного сохранения базового фото.",
		Order:       30,
		Keywords:    []string{"успех", "фото сохранено", "saved"},
	},
	"saved_photo_upload_prompt": {
		SectionID:   "saved_photo",
		Title:       "Загрузка базового фото",
		Description: "Экран с инструкцией отправить одно базовое фото для сохранения по умолчанию.",
		Order:       40,
		Keywords:    []string{"загрузка", "базовое фото", "upload"},
	},
	"saved_photo_batch_not_supported": {
		SectionID:   "saved_photo",
		Title:       "Пачка фото не поддерживается в saved photo",
		Description: "Системное сообщение, которое объясняет, что в разделе сохранённого фото пока принимается только один базовый кадр.",
		Order:       45,
		Keywords:    []string{"saved photo", "одно фото", "пачка не поддерживается"},
	},
	"saved_photo_generation_wait": {
		SectionID:   "saved_photo",
		Title:       "Генерация с сохранённым фото",
		Description: "Экран ожидания при запуске генерации на основе сохранённого фото.",
		Order:       50,
		Keywords:    []string{"ожидание", "saved photo", "генерация"},
	},
	"settings_overview": {
		SectionID:   "settings",
		Title:       "Общий экран настроек",
		Description: "Сводка текущих настроек и остатка генераций.",
		Order:       10,
		Keywords:    []string{"настройки", "сводка", "баланс"},
	},
	"settings_model": {
		SectionID:   "settings",
		Title:       "Выбор модели",
		Description: "Экран выбора модели генерации изображений.",
		Order:       20,
		Keywords:    []string{"модель", "nano banana", "gpt image"},
	},
	"settings_quality": {
		SectionID:   "settings",
		Title:       "Выбор качества",
		Description: "Экран выбора качества генерации.",
		Order:       30,
		Keywords:    []string{"качество", "1k", "2k", "4k"},
	},
	"settings_format": {
		SectionID:   "settings",
		Title:       "Выбор формата",
		Description: "Экран выбора соотношения сторон изображения.",
		Order:       40,
		Keywords:    []string{"формат", "aspect ratio", "1:1"},
	},
	"settings_balance": {
		SectionID:   "settings",
		Title:       "Баланс генераций",
		Description: "Экран с общим доступным балансом генераций пользователя.",
		Order:       50,
		Keywords:    []string{"баланс", "генерации", "остаток"},
	},
	"tariffs": {
		SectionID:   "billing",
		Title:       "Тарифы",
		Description: "Экран выбора тарифа для покупки генераций.",
		Order:       10,
		Keywords:    []string{"тарифы", "покупка", "генерации"},
	},
	"payment_checkout": {
		SectionID:   "billing",
		Title:       "Переход к оплате",
		Description: "Экран выбранного тарифа со ссылкой на оплату.",
		Order:       20,
		Keywords:    []string{"оплата", "checkout", "ссылка"},
	},
	"payment_reminder": {
		SectionID:   "billing",
		Title:       "Напоминание об оплате",
		Description: "Автосообщение через 3 минуты после экрана тарифов, если оплаты не было. Доступны шаблоны {{.TariffName}}, {{.Price}}, {{.GensCount}}. Кнопка тарифа подставляется автоматически.",
		Order:       25,
		Keywords:    []string{"напоминание", "догон", "брошенная оплата"},
	},
	"payment_canceled": {
		SectionID:   "billing",
		Title:       "Оплата отменена",
		Description: "Уведомление о брошенной или отмененной оплате.",
		Order:       30,
		Keywords:    []string{"отмена", "оплата", "payment"},
	},
	"payment_success": {
		SectionID:   "billing",
		Title:       "Оплата успешна",
		Description: "Подтверждение успешной оплаты тарифа.",
		Order:       40,
		Keywords:    []string{"успех", "оплата", "payment"},
	},
	"broadcast_no_gens": {
		SectionID:   "billing",
		Title:       "Рассылка: нулевой баланс",
		Description: "Сообщение перед экраном тарифов после нажатия кнопки «Сделать такие фото» в рассылке.",
		Order:       35,
		Keywords:    []string{"рассылка", "нет генераций", "cta"},
	},
	"no_gens_left": {
		SectionID:   "billing",
		Title:       "Закончились генерации",
		Description: "Экран, когда у пользователя нет доступных генераций.",
		Order:       40,
		Keywords:    []string{"нет генераций", "баланс", "пополнить"},
	},
	"referral_status": {
		SectionID:   "referrals",
		Title:       "Реферальная программа",
		Description: "Экран со статистикой приглашений и реферальной ссылкой.",
		Order:       10,
		Keywords:    []string{"рефералы", "ссылка", "друзья"},
	},
	"referral_bonus_awarded": {
		SectionID:   "referrals",
		Title:       "Начислен реферальный бонус",
		Description: "Уведомление о бонусе после оплаты приглашённого друга.",
		Order:       20,
		Keywords:    []string{"рефералы", "бонус", "начисление"},
	},
	"categories_empty": {
		SectionID:   "system",
		Title:       "Нет категорий",
		Description: "Пустое состояние, когда в библиотеке ещё нет категорий.",
		Order:       10,
		Keywords:    []string{"пусто", "категории", "ошибка данных"},
	},
	"prompts_empty": {
		SectionID:   "system",
		Title:       "Нет шаблонов в категории",
		Description: "Пустое состояние выбранной категории без промтов.",
		Order:       20,
		Keywords:    []string{"пусто", "шаблоны", "промты"},
	},
	"section_soon": {
		SectionID:   "system",
		Title:       "Раздел скоро наполнится",
		Description: "Экран узла, который уже виден в меню, но ещё без промтов. Один на все разделы: пропадает сам, как только в узел добавят промты.",
		Order:       21,
		Keywords:    []string{"скоро", "пусто", "заглушка", "раздел"},
	},
	// Экран про баланс, а не про поломку: ищут его рядом с «Закончились
	// генерации», а не среди технических ошибок, где он лежал до этапа 11.
	"no_gens_for_video": {
		SectionID:   "billing",
		Title:       "Не хватает генераций на видео",
		Description: "Отказ до запуска видео-тренда: у пользователя есть генерации, но меньше цены видео. Доступны {{.CostGens}}, {{.UserGens}} и {{.MissingGens}} — цена, баланс и нехватка.",
		Order:       45,
		Keywords:    []string{"видео", "видео тренд", "баланс", "не хватает", "генерации", "оживить фото"},
	},
	"video_scene_failed": {
		SectionID:   "system",
		Title:       "Видео: кадр не собрался",
		Description: "Ошибка первого звена цепочки — фото-модель не отдала кадр. Генерации возвращены.",
		Order:       23,
		Keywords:    []string{"видео", "ошибка", "кадр", "сцена"},
	},
	"video_generation_failed": {
		SectionID:   "system",
		Title:       "Видео: ошибка генерации",
		Description: "Ошибка второго звена — видео-модель не отдала файл. Генерации возвращены.",
		Order:       24,
		Keywords:    []string{"видео", "ошибка", "seedance"},
	},
	"prompt_not_found": {
		SectionID:   "system",
		Title:       "Шаблон не найден",
		Description: "Системный экран, если выбранный промт отсутствует.",
		Order:       30,
		Keywords:    []string{"не найден", "промт", "ошибка"},
	},
	"edit_result_missing_photo": {
		SectionID:   "system",
		Title:       "Нет фото для редактирования",
		Description: "Ошибка, если пользователь пытается редактировать отсутствующий результат.",
		Order:       40,
		Keywords:    []string{"нет фото", "редактирование", "ошибка"},
	},
	"generation_error": {
		SectionID:   "system",
		Title:       "Ошибка генерации",
		Description: "Общий экран ошибки во время генерации.",
		Order:       50,
		Keywords:    []string{"ошибка", "генерация", "retry"},
	},
	"generation_start_error": {
		SectionID:   "system",
		Title:       "Ошибка запуска генерации",
		Description: "Экран ошибки, если генерация не смогла стартовать.",
		Order:       60,
		Keywords:    []string{"запуск", "ошибка", "генерация"},
	},
	"payment_invalid_tariff": {
		SectionID:   "system",
		Title:       "Некорректный тариф",
		Description: "Ошибка, если передан неверный идентификатор тарифа.",
		Order:       70,
		Keywords:    []string{"тариф", "invalid", "ошибка"},
	},
	"payment_link_error": {
		SectionID:   "system",
		Title:       "Не удалось создать ссылку на оплату",
		Description: "Ошибка при формировании ссылки на оплату.",
		Order:       80,
		Keywords:    []string{"оплата", "ссылка", "ошибка"},
	},
	"payment_order_error": {
		SectionID:   "system",
		Title:       "Ошибка создания заказа",
		Description: "Ошибка при создании заказа в платёжном сценарии.",
		Order:       90,
		Keywords:    []string{"оплата", "заказ", "ошибка"},
	},
	"payment_tariff_not_found": {
		SectionID:   "system",
		Title:       "Тариф не найден",
		Description: "Системный экран, если тариф удалён или недоступен.",
		Order:       100,
		Keywords:    []string{"тариф", "не найден", "ошибка"},
	},
	"payment_unavailable": {
		SectionID:   "system",
		Title:       "Тарифы временно недоступны",
		Description: "Сообщение, когда платёжный сценарий временно отключён.",
		Order:       110,
		Keywords:    []string{"оплата", "недоступно", "тарифы"},
	},
	"saved_photo_error": {
		SectionID:   "system",
		Title:       "Ошибка сохранения фото",
		Description: "Экран ошибки при сохранении базового фото пользователя.",
		Order:       120,
		Keywords:    []string{"сохранённое фото", "ошибка", "save"},
	},
	"unknown_command": {
		SectionID:   "system",
		Title:       "Неизвестная команда",
		Description: "Fallback-экран для команд, которые бот не распознал.",
		Order:       130,
		Keywords:    []string{"unknown", "команда", "fallback"},
	},
	"worker_generation_failed": {
		SectionID:   "system",
		Title:       "Worker: генерация завершилась ошибкой",
		Description: "Сообщение из воркера, когда задача генерации завершилась неуспешно.",
		Order:       140,
		Keywords:    []string{"worker", "ошибка", "generation failed"},
	},
	"worker_no_output": {
		SectionID:   "system",
		Title:       "Worker: нет результата",
		Description: "Сообщение из воркера, когда сервис не вернул итоговый файл.",
		Order:       150,
		Keywords:    []string{"worker", "no output", "ошибка"},
	},
	"worker_submit_error": {
		SectionID:   "system",
		Title:       "Worker: ошибка отправки задачи",
		Description: "Ошибка при отправке задачи на генерацию во внешний сервис.",
		Order:       160,
		Keywords:    []string{"worker", "submit", "ошибка"},
	},
	"worker_vk_upload_error": {
		SectionID:   "system",
		Title:       "Worker: ошибка загрузки в VK",
		Description: "Фото сгенерировано, но не получилось отправить результат обратно в VK.",
		Order:       170,
		Keywords:    []string{"worker", "vk", "upload error"},
	},
}

func ScreenSections() []ScreenSection {
	out := make([]ScreenSection, len(screenSections))
	copy(out, screenSections)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func ScreenSectionByID(id string) (ScreenSection, bool) {
	section, ok := screenSectionsByID[id]
	return section, ok
}

func ScreenMeta(key string) ScreenAdminMeta {
	meta, ok := screenAdminMeta[key]
	if !ok {
		return ScreenAdminMeta{
			SectionID: "other",
			Title:     humanizeScreenKey(key),
		}
	}
	if meta.SectionID == "" {
		meta.SectionID = "other"
	}
	if _, ok := ScreenSectionByID(meta.SectionID); !ok {
		meta.SectionID = "other"
	}
	if meta.Title == "" {
		meta.Title = humanizeScreenKey(key)
	}
	meta.Keywords = cloneKeywords(meta.Keywords)
	return meta
}

func cloneKeywords(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func humanizeScreenKey(key string) string {
	if key == "" {
		return "Без названия"
	}

	parts := strings.FieldsFunc(strings.TrimSpace(key), func(r rune) bool {
		return r == '_' || r == '-' || unicode.IsSpace(r)
	})
	if len(parts) == 0 {
		return "Без названия"
	}

	for i, part := range parts {
		runes := []rune(strings.ToLower(part))
		if len(runes) == 0 {
			continue
		}
		runes[0] = unicode.ToUpper(runes[0])
		parts[i] = string(runes)
	}

	return strings.Join(parts, " ")
}
