package giftid

import (
	"fmt"
	"strings"
	"unicode"
)

// Identity — каноническое gift-identity (как в Radar/mono / MRKT display names).
// Пример: Collection="Lunar Snake", Model="Albino", Backdrop="Black".
type Identity struct {
	Collection string
	Model      string
	Backdrop   string
}

// Trim возвращает identity с обрезанными полями.
func (id Identity) Trim() Identity {
	return Identity{
		Collection: strings.TrimSpace(id.Collection),
		Model:      strings.TrimSpace(id.Model),
		Backdrop:   strings.TrimSpace(id.Backdrop),
	}
}

// Fold — ключ сопоставления: только [a-z0-9], lower.
// "Lunar Snake" / "lunar snake" / "lunarsnake" → "lunarsnake".
func Fold(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// SameCollection — true, если имена коллекций совпадают после Fold (и явных алиасов).
func SameCollection(a, b string) bool {
	return PortalsCollection(a) == PortalsCollection(b)
}

// MRKTNames — имена для запросов к MRKT (display, как в каталоге/Radar).
func MRKTNames(id Identity) Identity {
	id = id.Trim()
	if alias, ok := mrktCollectionAlias[Fold(id.Collection)]; ok {
		id.Collection = alias
	}
	return id
}

// PortalsNames — имена для filter_by_* на Portals.
// Коллекция → short_name (fold); модель/фон оставляем display (как в attributes).
func PortalsNames(id Identity) Identity {
	id = id.Trim()
	id.Collection = PortalsCollection(id.Collection)
	return id
}

// PortalsCollection переводит каноническое / любое написание коллекции в Portals short_name.
func PortalsCollection(canonical string) string {
	canonical = strings.TrimSpace(canonical)
	if canonical == "" {
		return ""
	}
	key := Fold(canonical)
	if alias, ok := portalsCollectionAlias[key]; ok {
		return alias
	}
	return key
}

// portalsCollectionAlias — исключения, где fold(display) ≠ Portals short_name
// (на 2026-08 live probe: 0 расхождений среди ~118 коллекций; карта для явных опечаток/синонимов).
var portalsCollectionAlias = map[string]string{
	// Пример: если MRKT когда-то отдаст другое написание —
	// "durovscap": "durovscap" already fold-equal to “Durov’s Cap”.
}

// mrktCollectionAlias — если на вход пришёл Portals short_name, а MRKT ждёт display.
// Минимальный набор из research/фикстур; расширяется по мере расхождений.
var mrktCollectionAlias = map[string]string{
	"lunarsnake":   "Lunar Snake",
	"icecream":     "Ice Cream",
	"astralshard":  "Astral Shard",
	"plushpepe":    "Plush Pepe",
	"deskcalendar": "Desk Calendar",
}

// TelegramGiftID — slug для t.me/nft и Portals tg_id: "Fine Pen", 15525 → "FinePen-15525".
func TelegramGiftID(collection string, number int) string {
	collection = strings.TrimSpace(collection)
	if collection == "" || number < 0 {
		return ""
	}
	parts := strings.Fields(collection)
	var b strings.Builder
	for _, p := range parts {
		p = strings.Trim(p, "'\"")
		if p == "" {
			continue
		}
		p = strings.ReplaceAll(p, "'", "")
		p = strings.ReplaceAll(p, "’", "")
		r := []rune(strings.ToLower(p))
		if len(r) == 0 {
			continue
		}
		r[0] = unicode.ToUpper(r[0])
		b.WriteString(string(r))
	}
	if b.Len() == 0 {
		return ""
	}
	return fmt.Sprintf("%s-%d", b.String(), number)
}

// TelegramNFTURL — страница коллекционного подарка в Telegram (не Mini App Portals).
func TelegramNFTURL(collection string, number int) string {
	id := TelegramGiftID(collection, number)
	if id == "" {
		return ""
	}
	return "https://t.me/nft/" + id
}
