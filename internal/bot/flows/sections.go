package flows

import (
	"context"

	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/repository"
)

// Универсальный навигатор по дереву разделов. До этапа 5 каждый раздел ходил по
// своим категориям сам («Фото для себя» — через ListReadyPromptCategories,
// «Парное фото» — через ListActiveCouple), и добавление подменю означало новый
// хендлер. Теперь уровень меню строится одинаково: есть активные дети — показываем
// их, нет — показываем промты узла.

// sectionSpec описывает раздел: чем он отличается от остальных на уровне корня.
// Ниже корня разделы неразличимы, там работает общий код.
type sectionSpec struct {
	section string
	// promptType уезжает в State и решает, какой сценарий генерации сработает
	// после выбора промта.
	promptType string
	// rootScreen — экран верхнего уровня раздела.
	rootScreen string
	// rootPager — тип листалки корневого уровня. Для старых разделов это прежние
	// значения: клавиатуры, уже отправленные пользователям, продолжают работать.
	rootPager   string
	nodesStep   string
	promptsStep string
	// genderOverride — пол для отбора промтов, если он задаётся разделом,
	// а не полом пользователя.
	genderOverride string
	// requireContent прячет пустые узлы. Разделы с заглушкой «Скоро» (этап 7)
	// ставят false и показывают узел даже без промтов.
	requireContent bool
}

var sectionSpecs = map[string]sectionSpec{
	repository.SectionSelf: {
		section:        repository.SectionSelf,
		promptType:     "ready_prompt",
		rootScreen:     "ready_prompts_intro",
		rootPager:      "ready_prompts_page",
		nodesStep:      StepReadyPromptsCategories,
		promptsStep:    StepReadyPromptsPrompts,
		requireContent: true,
	},
	repository.SectionCouple: {
		section:        repository.SectionCouple,
		promptType:     "couple",
		rootScreen:     "couple_submenu",
		rootPager:      "couple_page",
		nodesStep:      StepCoupleCategories,
		promptsStep:    StepCouplePrompts,
		genderOverride: repository.GenderCouple,
	},
	repository.SectionKids: {
		section:     repository.SectionKids,
		promptType:  "kids",
		rootScreen:  "kids_intro",
		rootPager:   "kids_page",
		nodesStep:   StepKidsCategories,
		promptsStep: StepKidsPrompts,
		// Пол берётся с узла («Мальчик»/«Девочка»), а не с пользователя, и
		// requireContent = false: раздел включается пустым и показывает «Скоро».
	},
}

// specByStep — обратный индекс: по шагу состояния находим раздел. Без него
// каждый новый раздел означал бы правку switch'ей в «Назад» и в реестре.
var specByStep = func() map[string]sectionSpec {
	byStep := make(map[string]sectionSpec, len(sectionSpecs)*2)
	for _, spec := range sectionSpecs {
		byStep[spec.nodesStep] = spec
		byStep[spec.promptsStep] = spec
	}
	return byStep
}()

// IsSectionStep отвечает, находится ли пользователь внутри дерева разделов.
func IsSectionStep(step string) bool {
	_, ok := specByStep[step]
	return ok
}

func isPromptsStep(step string) bool {
	spec, ok := specByStep[step]
	return ok && spec.promptsStep == step
}

func specForSection(section string) sectionSpec {
	if spec, ok := sectionSpecs[section]; ok {
		return spec
	}
	return sectionSpecs[repository.SectionSelf]
}

// HandleOpenCategory — вход в узел дерева. Обслуживает и новый callback
// open_category, и старый select_category.
func HandleOpenCategory(ctx context.Context, fc *Context, d *Deps) {
	categoryID := fc.Callback.CategoryID
	if categoryID == 0 {
		categoryID = fc.State.CategoryID
	}
	if categoryID == 0 {
		if err := showSectionRoots(ctx, fc, d, specForState(fc), normalizePage(fc.State.CategoryPage)); err != nil {
			log.Error().Err(err).Int64("vk_id", fc.VkID).Msg("ошибка возврата к корню раздела")
		}
		return
	}
	if err := openCategoryNode(ctx, fc, d, categoryID, 1); err != nil {
		log.Error().Err(err).Int64("vk_id", fc.VkID).Int("category_id", categoryID).Msg("ошибка открытия узла раздела")
	}
}

// HandleOpenCategoryPage листает подменю внутри узла: category_id в payload —
// это родитель показываемого списка.
func HandleOpenCategoryPage(ctx context.Context, fc *Context, d *Deps) {
	parentID := fc.Callback.CategoryID
	page := normalizePage(fc.Callback.Page)
	if parentID == 0 {
		if err := showSectionRoots(ctx, fc, d, specForState(fc), page); err != nil {
			log.Error().Err(err).Int64("vk_id", fc.VkID).Int("page", page).Msg("ошибка листалки корня раздела")
		}
		return
	}
	if err := openCategoryNode(ctx, fc, d, parentID, page); err != nil {
		log.Error().Err(err).Int64("vk_id", fc.VkID).Int("category_id", parentID).Int("page", page).Msg("ошибка листалки подменю")
	}
}

// openCategoryNode показывает содержимое узла: подменю, если у него есть активные
// дети, иначе промты самого узла.
func openCategoryNode(ctx context.Context, fc *Context, d *Deps, categoryID, page int) error {
	cat, err := d.CatRepo.GetByID(ctx, categoryID)
	if err != nil || cat == nil {
		log.Warn().Err(err).Int64("vk_id", fc.VkID).Int("category_id", categoryID).Msg("узел раздела не найден")
		return sendScreen(ctx, d, fc.VkID, "prompts_empty", ScreenOptions{})
	}

	spec := specForSection(cat.Section)
	children, err := d.CatRepo.ListChildren(ctx, categoryID, repository.CategoryFilter{
		Gender:         resolveNodeGender(ctx, d, cat, fc),
		RequireContent: spec.requireContent,
	})
	if err != nil {
		log.Error().Err(err).Int("category_id", categoryID).Msg("ошибка получения детей узла")
	}

	if len(children) == 0 {
		return showPromptPage(ctx, fc, d, categoryID, normalizePage(fc.State.CategoryPage), 1)
	}

	rows, currentPage, _ := buildPaginatedRows(
		categoryButtons(children),
		page,
		"open_category_page",
		map[string]any{"category_id": categoryID},
	)
	state := copyPrefs(&State{
		Step:         spec.nodesStep,
		PromptType:   spec.promptType,
		Section:      cat.Section,
		SectionID:    categoryID,
		CategoryPage: currentPage,
		PromptPage:   1,
	}, fc.State)
	fc.State = state
	_ = d.State.Set(ctx, fc.VkID, state)

	return sendScreen(ctx, d, fc.VkID, nodeScreenKey(cat, spec), ScreenOptions{PrefixRows: rows})
}

// showSectionRoots показывает верхний уровень раздела.
func showSectionRoots(ctx context.Context, fc *Context, d *Deps, spec sectionSpec, page int) error {
	categories, err := d.CatRepo.ListRoots(ctx, spec.section, repository.CategoryFilter{
		Gender:         sectionGender(spec, fc),
		RequireContent: spec.requireContent,
	})
	if err != nil || len(categories) == 0 {
		if err != nil {
			log.Error().Err(err).Str("section", spec.section).Msg("ошибка получения корней раздела")
		}
		return sendScreen(ctx, d, fc.VkID, "categories_empty", ScreenOptions{})
	}

	rows, currentPage, _ := buildPaginatedRows(categoryButtons(categories), page, spec.rootPager, nil)
	state := copyPrefs(&State{
		Step:         spec.nodesStep,
		PromptType:   spec.promptType,
		Section:      spec.section,
		CategoryPage: currentPage,
		PromptPage:   1,
	}, fc.State)
	fc.State = state
	_ = d.State.Set(ctx, fc.VkID, state)

	return sendScreen(ctx, d, fc.VkID, spec.rootScreen, ScreenOptions{PrefixRows: rows})
}

// sectionBack обрабатывает «Назад» внутри дерева и отвечает, сработал ли он.
// Правило одно: с промтов — к родителю категории, с подменю — на уровень выше,
// с корня раздела — наружу, в главное меню.
func sectionBack(ctx context.Context, fc *Context, d *Deps) bool {
	if fc.State == nil || !IsSectionStep(fc.State.Step) {
		return false
	}
	if isPromptsStep(fc.State.Step) {
		if err := showParentOfCategory(ctx, fc, d, fc.State.CategoryID); err != nil {
			log.Error().Err(err).Int64("vk_id", fc.VkID).Int("category_id", fc.State.CategoryID).Msg("ошибка возврата на уровень выше")
		}
		return true
	}
	if fc.State.SectionID == 0 {
		return false // корень раздела — выходим в главное меню
	}
	if err := showParentOfCategory(ctx, fc, d, fc.State.SectionID); err != nil {
		log.Error().Err(err).Int64("vk_id", fc.VkID).Int("category_id", fc.State.SectionID).Msg("ошибка возврата из подменю")
	}
	return true
}

// showParentOfCategory открывает уровень, с которого пользователь пришёл в узел.
func showParentOfCategory(ctx context.Context, fc *Context, d *Deps, categoryID int) error {
	page := normalizePage(fc.State.CategoryPage)
	spec := specForState(fc)

	if categoryID == 0 {
		return showSectionRoots(ctx, fc, d, spec, page)
	}

	cat, err := d.CatRepo.GetByID(ctx, categoryID)
	if err != nil || cat == nil {
		return showSectionRoots(ctx, fc, d, spec, page)
	}
	if cat.ParentID == nil {
		return showSectionRoots(ctx, fc, d, specForSection(cat.Section), page)
	}
	return openCategoryNode(ctx, fc, d, *cat.ParentID, page)
}

// specForState восстанавливает раздел из состояния пользователя — состояния,
// записанные до этапа 5, поля Section не имеют, поэтому смотрим и на PromptType.
func specForState(fc *Context) sectionSpec {
	if fc.State == nil {
		return specForSection(repository.SectionSelf)
	}
	if fc.State.Section != "" {
		return specForSection(fc.State.Section)
	}
	if spec, ok := specByStep[fc.State.Step]; ok {
		return spec
	}
	if fc.State.PromptType == "couple" {
		return specForSection(repository.SectionCouple)
	}
	return specForSection(repository.SectionSelf)
}

// nodeScreenKey — экран узла: свой, если задан в админке, иначе экран раздела.
func nodeScreenKey(cat *repository.Category, spec sectionSpec) string {
	if cat != nil && cat.ScreenKey != nil && *cat.ScreenKey != "" {
		return *cat.ScreenKey
	}
	return spec.rootScreen
}

// resolveNodeGender определяет, каким полом фильтровать промты узла: заданным
// разделом (парные фото, адресат поздравления) или полом пользователя.
func resolveNodeGender(ctx context.Context, d *Deps, cat *repository.Category, fc *Context) string {
	if cat != nil {
		if cat.PromptGender != nil && *cat.PromptGender != "" {
			return *cat.PromptGender
		}
		// Значение наследуется от предков, но лишний запрос делаем только для
		// вложенных узлов: у корней наследовать не от кого.
		if cat.ParentID != nil {
			path, err := d.CatRepo.Path(ctx, cat.ID)
			if err != nil {
				log.Error().Err(err).Int("category_id", cat.ID).Msg("ошибка получения пути узла")
			}
			for i := len(path) - 1; i >= 0; i-- {
				if path[i].PromptGender != nil && *path[i].PromptGender != "" {
					return *path[i].PromptGender
				}
			}
		}
		return userGender(fc)
	}
	return userGender(fc)
}

func sectionGender(spec sectionSpec, fc *Context) string {
	if spec.genderOverride != "" {
		return spec.genderOverride
	}
	return userGender(fc)
}

func userGender(fc *Context) string {
	if fc == nil || fc.User == nil {
		return repository.GenderAny
	}
	return fc.User.Gender
}

// ─── Детские фото ────────────────────────────────────────────────────────────

// HandleKidsMenu — вход в раздел из главного меню. Пол здесь задают кнопки
// «Мальчик»/«Девочка», поэтому пол пользователя, в отличие от «Фото для себя»,
// не спрашиваем.
func HandleKidsMenu(ctx context.Context, fc *Context, d *Deps) {
	if !fc.User.HasGens() {
		_ = sendScreen(ctx, d, fc.VkID, "no_gens_left", ScreenOptions{})
		return
	}
	if err := showSectionRoots(ctx, fc, d, specForSection(repository.SectionKids), 1); err != nil {
		log.Error().Err(err).Int64("vk_id", fc.VkID).Msg("ошибка показа детского раздела")
	}
}

func HandleKidsPage(ctx context.Context, fc *Context, d *Deps) {
	page := normalizePage(fc.Callback.Page)
	if err := showSectionRoots(ctx, fc, d, specForSection(repository.SectionKids), page); err != nil {
		log.Error().Err(err).Int64("vk_id", fc.VkID).Int("page", page).Msg("ошибка листалки детского раздела")
	}
}

// HandleSectionBrowse — повторная отправка текущего уровня, когда пользователь
// пишет текстом вместо нажатия кнопки. Общая для всех разделов дерева.
func HandleSectionBrowse(ctx context.Context, fc *Context, d *Deps) {
	spec := specForState(fc)
	if isPromptsStep(fc.State.Step) && fc.State.CategoryID != 0 {
		if err := showPromptPage(ctx, fc, d, fc.State.CategoryID, normalizePage(fc.State.CategoryPage), normalizePage(fc.State.PromptPage)); err != nil {
			log.Error().Err(err).Int64("vk_id", fc.VkID).Int("category_id", fc.State.CategoryID).Msg("ошибка повторной отправки шаблонов раздела")
		}
		return
	}
	if fc.State.SectionID != 0 {
		if err := openCategoryNode(ctx, fc, d, fc.State.SectionID, normalizePage(fc.State.CategoryPage)); err != nil {
			log.Error().Err(err).Int64("vk_id", fc.VkID).Int("category_id", fc.State.SectionID).Msg("ошибка повторной отправки подменю раздела")
		}
		return
	}
	if err := showSectionRoots(ctx, fc, d, spec, normalizePage(fc.State.CategoryPage)); err != nil {
		log.Error().Err(err).Int64("vk_id", fc.VkID).Msg("ошибка повторной отправки корня раздела")
	}
}
