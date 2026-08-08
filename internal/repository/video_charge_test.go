package repository

import "testing"

// Видео стоит десятки генераций, и такое списание вполне может не поместиться
// целиком ни в платный баланс, ни в бесплатный. Раскладка обязана быть точной:
// вернуть при ошибке нужно ровно то, что снято, и ровно туда, откуда снято.
func TestSplitChargeSpendsPaidBalanceFirst(t *testing.T) {
	tests := []struct {
		name     string
		cost     int
		paid     int
		free     int
		wantPaid int
		wantFree int
		wantOK   bool
	}{
		{name: "хватает платного", cost: 40, paid: 100, free: 3, wantPaid: 40, wantFree: 0, wantOK: true},
		{name: "платного нет вовсе", cost: 40, paid: 0, free: 40, wantPaid: 0, wantFree: 40, wantOK: true},
		{name: "списание разъезжается по двум балансам", cost: 40, paid: 12, free: 30, wantPaid: 12, wantFree: 28, wantOK: true},
		{name: "в сумме не хватает одной", cost: 40, paid: 12, free: 27, wantOK: false},
		{name: "пустой баланс", cost: 1, paid: 0, free: 0, wantOK: false},
		{name: "нулевая цена считается одной генерацией", cost: 0, paid: 0, free: 1, wantPaid: 0, wantFree: 1, wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paid, free, ok := splitCharge(tt.cost, tt.paid, tt.free)
			if ok != tt.wantOK {
				t.Fatalf("splitCharge(%d, %d, %d) ok = %v, want %v", tt.cost, tt.paid, tt.free, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if paid != tt.wantPaid || free != tt.wantFree {
				t.Fatalf("splitCharge(%d, %d, %d) = (%d, %d), want (%d, %d)",
					tt.cost, tt.paid, tt.free, paid, free, tt.wantPaid, tt.wantFree)
			}
			want := tt.cost
			if want < 1 {
				want = 1
			}
			if paid+free != want {
				t.Fatalf("списано %d генераций вместо %d", paid+free, want)
			}
			if paid > tt.paid || free > tt.free {
				t.Fatalf("списано больше, чем было на балансе: paid %d/%d, free %d/%d", paid, tt.paid, free, tt.free)
			}
		})
	}
}

func TestVideoPromptIsRecognisedByMediaKind(t *testing.T) {
	// Цены у промта нет: её источник — тариф-видеопакет. От карточки требуется
	// только честно сказать, видео это или фото.
	if (&Prompt{MediaKind: MediaKindPhoto}).IsVideo() {
		t.Fatal("фото-промт опознан как видео")
	}
	if !(&Prompt{MediaKind: MediaKindVideo}).IsVideo() {
		t.Fatal("видео-промт не опознан")
	}
}
