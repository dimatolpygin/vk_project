package handlers

import (
	"bytes"
	"html/template"
	"testing"
	"time"

	"vk_neuro_bot/internal/repository"
)

func TestBuildUserListItemViewUsesFallbackDisplayName(t *testing.T) {
	now := time.Date(2026, time.May, 8, 12, 0, 0, 0, time.Local)
	view := buildUserListItemView(&repository.AdminUserListItem{
		User: repository.User{
			VKID:     42,
			FreeGens: 2,
			PaidGens: 3,
			Status:   "free",
		},
		LastActivity: now,
	})

	if view.DisplayName != "Пользователь 42" {
		t.Fatalf("unexpected display name %q", view.DisplayName)
	}
	if view.TotalGens != 5 {
		t.Fatalf("expected total gens 5, got %d", view.TotalGens)
	}
	if view.ProfileURL != "https://vk.com/id42" {
		t.Fatalf("unexpected profile url %q", view.ProfileURL)
	}
}

func TestBuildUserDetailPageViewSummarizesOrders(t *testing.T) {
	savedPhotoURL := "https://example.com/saved.png"
	view := buildUserDetailPageView(&repository.AdminUserDetail{
		User: repository.User{
			VKID:           77,
			Username:       "demo",
			FirstName:      "Ира",
			FreeGens:       1,
			PaidGens:       4,
			Status:         "paid",
			SavedPhotoURLs: []string{savedPhotoURL},
			UseSavedPhoto:  true,
			PrefModel:      "google/nano-banana-pro",
			PrefResolution: "2k",
		},
		GeneratedCount:     6,
		CompletedGenCount:  5,
		ReferredCount:      2,
		ReferralBonusCount: 1,
		OrdersCount:        3,
		SuccessOrdersCount: 2,
		TotalSpent:         998,
	}, []*repository.Order{
		{ID: 10, TariffID: 2, Amount: 499, Status: "succeeded", CreatedAt: time.Date(2026, time.May, 8, 9, 0, 0, 0, time.Local)},
	})

	if view.DisplayName != "Ира" {
		t.Fatalf("unexpected display name %q", view.DisplayName)
	}
	if view.TotalGens != 5 {
		t.Fatalf("expected total gens 5, got %d", view.TotalGens)
	}
	if view.TotalSpent != "998 ₽" {
		t.Fatalf("unexpected total spent %q", view.TotalSpent)
	}
	if len(view.Orders) != 1 {
		t.Fatalf("expected one order, got %d", len(view.Orders))
	}
	if view.SavedPhotoLabel != "Сохранено и включено" {
		t.Fatalf("unexpected saved photo label %q", view.SavedPhotoLabel)
	}
}

func TestUsersTemplateRendersListAndDetail(t *testing.T) {
	listData := usersPageData{
		Title:  "Пользователи",
		Active: "users",
		Summary: usersSummaryView{
			TotalUsers:    10,
			PaidUsers:     4,
			NewUsersToday: 2,
			ActiveUsers7d: 8,
		},
		Users: []userListItemView{
			{
				VKID:         1,
				DisplayName:  "Аня",
				Subtitle:     "@anya · vk.com/id1",
				ProfileURL:   "https://vk.com/id1",
				StatusLabel:  "paid",
				StatusClass:  "success",
				TotalGens:    4,
				FreeGens:     1,
				PaidGens:     3,
				CreatedAt:    "08.05.2026 12:00",
				LastActivity: "08.05.2026 12:10",
				DetailURL:    "/admin/users/1",
			},
		},
		VisibleCount: 1,
		Total:        1,
		Page:         1,
	}

	detailData := usersPageData{
		Title:  "Пользователь",
		Active: "users",
		Detail: &userDetailPageView{
			VKID:            1,
			DisplayName:     "Аня",
			Subtitle:        "@anya · vk.com/id1",
			ProfileURL:      "https://vk.com/id1",
			StatusLabel:     "paid",
			StatusClass:     "success",
			TotalGens:       4,
			CreatedAt:       "08.05.2026 12:00",
			LastActivity:    "08.05.2026 12:10",
			Gender:          "Женский",
			ReferralCode:    "ref_test",
			SubscribedLabel: "Подписан",
			SavedPhotoLabel: "Нет сохранённого фото",
			PrefModel:       "Nano Banana Pro",
			PrefResolution:  "2k",
			PrefAspectRatio: "1:1",
			AddGensURL:      "/admin/users/1/add-gens",
			DeleteURL:       "/admin/users/1/delete",
			BackURL:         "/admin/users",
			TotalSpent:      "0 ₽",
		},
	}

	tmpl := template.Must(template.New("").Funcs(tmplFuncs).ParseFiles("../../../templates/layout.html", "../../../templates/users.html"))

	var listBuf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&listBuf, "layout", listData); err != nil {
		t.Fatalf("render list template: %v", err)
	}

	var detailBuf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&detailBuf, "layout", detailData); err != nil {
		t.Fatalf("render detail template: %v", err)
	}
}
