package content

type ActionAdminMeta struct {
	SectionID   string
	Title       string
	Description string
	Order       int
}

var actionAdminMeta = map[string]ActionAdminMeta{
	"back": {
		SectionID:   "navigation",
		Title:       "Назад",
		Description: "Возврат к предыдущему шагу или главному сценарию.",
		Order:       10,
	},
	"free_gen": {
		SectionID:   "generation",
		Title:       "Запуск бесплатной генерации",
		Description: "Старт бесплатного сценария генерации.",
		Order:       20,
	},
	"gen_again": {
		SectionID:   "generation",
		Title:       "Сгенерировать ещё",
		Description: "Повтор генерации после результата.",
		Order:       30,
	},
	"check_sub": {
		SectionID:   "onboarding",
		Title:       "Проверка подписки",
		Description: "Подтверждение подписки на группу перед стартом.",
		Order:       40,
	},
	"gender_male": {
		SectionID:   "onboarding",
		Title:       "Выбор мужского пола",
		Description: "Выбор пола для подбора подходящих сценариев.",
		Order:       50,
	},
	"gender_female": {
		SectionID:   "onboarding",
		Title:       "Выбор женского пола",
		Description: "Выбор пола для подбора подходящих сценариев.",
		Order:       60,
	},
	"tariffs": {
		SectionID:   "billing",
		Title:       "Открыть тарифы",
		Description: "Переход к экрану выбора тарифа.",
		Order:       70,
	},
	"buy_gens": {
		SectionID:   "billing",
		Title:       "Купить генерации",
		Description: "Переход к покупке генераций через тарифы.",
		Order:       80,
	},
	"buy_tariff": {
		SectionID:   "billing",
		Title:       "Покупка тарифа",
		Description: "Выбор конкретного тарифа для оплаты.",
		Order:       90,
	},
	"referral": {
		SectionID:   "referrals",
		Title:       "Реферальная программа",
		Description: "Открытие раздела с реферальной ссылкой и бонусами.",
		Order:       100,
	},
	"main_menu": {
		SectionID:   "navigation",
		Title:       "Главное меню",
		Description: "Возврат на главный экран бота.",
		Order:       110,
	},
	"ready_prompts": {
		SectionID:   "styles",
		Title:       "Готовые стили",
		Description: "Переход в библиотеку готовых категорий и промптов.",
		Order:       120,
	},
	"select_category": {
		SectionID:   "styles",
		Title:       "Выбор категории",
		Description: "Открытие категории готовых стилей.",
		Order:       130,
	},
	"select_prompt": {
		SectionID:   "styles",
		Title:       "Выбор стиля",
		Description: "Выбор конкретного готового стиля или шаблона.",
		Order:       140,
	},
	"custom_prompt": {
		SectionID:   "generation",
		Title:       "Свой промпт",
		Description: "Переход к сценарию генерации по собственному описанию.",
		Order:       150,
	},
	"edit_photo": {
		SectionID:   "generation",
		Title:       "Изменить фото",
		Description: "Запуск режима редактирования существующего фото.",
		Order:       160,
	},
	"edit_result": {
		SectionID:   "generation",
		Title:       "Редактировать результат",
		Description: "Переход к правкам уже сгенерированного фото.",
		Order:       170,
	},
	"couple": {
		SectionID:   "generation",
		Title:       "Парное фото",
		Description: "Запуск сценария для парных или семейных фото.",
		Order:       180,
	},
	"saved_photo": {
		SectionID:   "saved_photo",
		Title:       "Сохранённое фото",
		Description: "Открытие меню сохранённого базового фото.",
		Order:       190,
	},
	"saved_photo_upload": {
		SectionID:   "saved_photo",
		Title:       "Загрузка сохранённого фото",
		Description: "Переход к загрузке или замене сохранённого фото.",
		Order:       200,
	},
	"toggle_saved_photo": {
		SectionID:   "saved_photo",
		Title:       "Переключить сохранённое фото",
		Description: "Включение или выключение использования сохранённого фото.",
		Order:       210,
	},
	"settings": {
		SectionID:   "settings",
		Title:       "Настройки",
		Description: "Открытие раздела настроек генерации.",
		Order:       220,
	},
	"quality": {
		SectionID:   "settings",
		Title:       "Качество",
		Description: "Открытие выбора качества генерации.",
		Order:       230,
	},
	"format": {
		SectionID:   "settings",
		Title:       "Формат",
		Description: "Открытие выбора соотношения сторон.",
		Order:       240,
	},
	"balance": {
		SectionID:   "settings",
		Title:       "Баланс генераций",
		Description: "Переход к информации об остатке генераций.",
		Order:       250,
	},
	"model": {
		SectionID:   "settings",
		Title:       "Модель",
		Description: "Открытие списка моделей генерации.",
		Order:       260,
	},
	"model_nbp": {
		SectionID:   "settings",
		Title:       "Выбор модели Nano Banana Pro",
		Description: "Сохранение модели Nano Banana Pro.",
		Order:       270,
	},
	"model_nb2": {
		SectionID:   "settings",
		Title:       "Выбор модели Nano Banana 2",
		Description: "Сохранение модели Nano Banana 2.",
		Order:       280,
	},
	"model_gpt2": {
		SectionID:   "settings",
		Title:       "Выбор модели GPT Image 2",
		Description: "Сохранение модели GPT Image 2.",
		Order:       290,
	},
	"quality_1k": {
		SectionID:   "settings",
		Title:       "Качество 1k",
		Description: "Выбор стандартного качества 1k.",
		Order:       300,
	},
	"quality_2k": {
		SectionID:   "settings",
		Title:       "Качество 2k",
		Description: "Выбор качества HD 2k.",
		Order:       310,
	},
	"quality_4k": {
		SectionID:   "settings",
		Title:       "Качество 4k",
		Description: "Выбор качества Ultra 4k.",
		Order:       320,
	},
	"ar_1_1": {
		SectionID:   "settings",
		Title:       "Формат 1:1",
		Description: "Выбор квадратного формата изображения.",
		Order:       330,
	},
	"ar_9_16": {
		SectionID:   "settings",
		Title:       "Формат 9:16",
		Description: "Выбор портретного формата изображения.",
		Order:       340,
	},
	"ar_16_9": {
		SectionID:   "settings",
		Title:       "Формат 16:9",
		Description: "Выбор пейзажного формата изображения.",
		Order:       350,
	},
	"support": {
		SectionID:   "navigation",
		Title:       "Техподдержка",
		Description: "Переход к экрану связи с поддержкой.",
		Order:       360,
	},
	"examples": {
		SectionID:   "navigation",
		Title:       "Примеры работ",
		Description: "Открытие экрана с примерами сгенерированных работ.",
		Order:       370,
	},
}

func ActionMeta(key string) ActionAdminMeta {
	meta, ok := actionAdminMeta[key]
	if !ok {
		return ActionAdminMeta{
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
	return meta
}
