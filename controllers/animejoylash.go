package controllers

import (
	"fmt"
	"sort"
	"strings"

	"creator/models"

	"github.com/beego/beego/v2/client/orm" // yoki sizning ORM import'ingiz
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ============================================================
// VAQTINCHALIK XOTIRA
// ============================================================

type animeDraft struct {
	BotID     int64
	Name      string
	Code      string
	PhotoID   string
	AnimeID   int64
	NextOrder int
}

var animeDrafts = make(map[int64]*animeDraft)

// Joylash jarayonini tozalash
func clearAnimeDraft(userID int64) {
	delete(animeDrafts, userID)
	delete(adminState, userID) // adminState boshqa faylda bo'lsa ham ishlaydi
}

// ============================================================
// 1. Anime joylashni boshlash
// ============================================================
func StartAnimeUpload(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
	userID := msg.From.ID

	animeDrafts[userID] = &animeDraft{
		BotID:     b.Id,
		NextOrder: 1,
	}
	adminState[userID] = "anime_name"

	bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "🎬 Anime nomini kiriting:"))
}

// ============================================================
// 2. Anime nomi
// ============================================================
func HandleAnimeNameStep(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
	userID := msg.From.ID
	chatID := msg.Chat.ID

	name := strings.TrimSpace(msg.Text)
	if name == "" {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Nomi bo'sh bo'lishi mumkin emas. Iltimos, nom kiriting:"))
		return
	}

	draft, ok := animeDrafts[userID]
	if !ok {
		draft = &animeDraft{BotID: b.Id}
		animeDrafts[userID] = draft
	}

	draft.Name = name
	adminState[userID] = "anime_code"

	bot.Send(tgbotapi.NewMessage(chatID, "🆔 Endi anime kodini kiriting (faqat lotin harflari va raqamlar tavsiya qilinadi):"))
}

// ============================================================
// 3. Anime kodi + unique tekshirish
// ============================================================
func HandleAnimeCodeStep(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
	userID := msg.From.ID
	chatID := msg.Chat.ID

	code := strings.ToLower(strings.TrimSpace(msg.Text))
	if code == "" {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Kod bo'sh bo'lishi mumkin emas. Iltimos, kod kiriting:"))
		return
	}

	draft, ok := animeDrafts[userID]
	if !ok {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Jarayon topilmadi. Iltimos, /admin orqali qaytadan boshlang."))
		clearAnimeDraft(userID)
		return
	}

	// Shu bot uchun kod bandligini tekshirish
	o := orm.NewOrm()
	existing := models.Anime{}
	err := o.QueryTable(new(models.Anime)).
		Filter("Bot__Id", b.Id).
		Filter("Code", code).
		One(&existing)

	if err == nil {
		text := fmt.Sprintf("❌ Bu kod (%s) allaqachon mavjud!\n\nAnime: %s\n\nBoshqa kod kiriting:", code, existing.Name)
		bot.Send(tgbotapi.NewMessage(chatID, text))
		return
	}

	draft.Code = code
	adminState[userID] = "anime_photo"

	bot.Send(tgbotapi.NewMessage(chatID, "✅ Kod qabul qilindi.\n\n🌌 Endi anime muqovasi (rasm) yuboring:"))
}

// ============================================================
// 4. Muqova rasmi + DB ga saqlash
// ============================================================
func HandleAnimePhotoStep(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
	userID := msg.From.ID
	chatID := msg.Chat.ID

	if msg.Photo == nil || len(msg.Photo) == 0 {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Iltimos, anime muqovasi uchun rasm yuboring."))
		return
	}

	draft, ok := animeDrafts[userID]
	if !ok {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Jarayon topilmadi. Qaytadan boshlang."))
		clearAnimeDraft(userID)
		return
	}

	photoID := msg.Photo[len(msg.Photo)-1].FileID
	draft.PhotoID = photoID

	o := orm.NewOrm()
	animeRow := models.Anime{
		Bot:        b,
		Name:       draft.Name,
		Code:       draft.Code,
		PhotoID:    photoID,
		PartsCount: 0,
		IsActive:   true,
	}

	id, err := o.Insert(&animeRow)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Bazaga saqlashda xatolik yuz berdi. Qaytadan urinib ko'ring."))
		clearAnimeDraft(userID)
		return
	}

	draft.AnimeID = id
	draft.NextOrder = 1
	adminState[userID] = "anime_videos"

	text := fmt.Sprintf(
		"🎬 **Nom:** %s\n🆔 **Kod:** `%s`\n\n🌌 **Muqova saqlandi!**\n\nEndi videolar, fayllar yoki rasmlarni yuboring.\n\nTugatganingizda **/ok** deb yozing.",
		draft.Name, draft.Code,
	)

	msgConfig := tgbotapi.NewMessage(chatID, text)
	msgConfig.ParseMode = "Markdown"
	bot.Send(msgConfig)
}

// ============================================================
// 5. Video / Fayl / Rasm qismlari
// ============================================================
func HandleAnimeVideosStep(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
	userID := msg.From.ID
	chatID := msg.Chat.ID

	draft, ok := animeDrafts[userID]
	if !ok || draft.AnimeID == 0 {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Avval anime ma'lumotlarini to'ldiring."))
		clearAnimeDraft(userID)
		return
	}

	o := orm.NewOrm()

	// /ok - yakunlash
	if msg.Text == "/ok" {
		finishAnimeUpload(bot, o, chatID, userID, draft)
		return
	}

	// /cancel - bekor qilish
	if msg.Text == "/cancel" {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Anime joylash bekor qilindi."))
		clearAnimeDraft(userID)
		return
	}

	// Kontent turi
	var kind, fileID string
	msgID := msg.MessageID

	switch {
	case msg.Video != nil:
		kind = "video"
		fileID = msg.Video.FileID
	case msg.Document != nil:
		kind = "document"
		fileID = msg.Document.FileID
	case msg.Photo != nil && len(msg.Photo) > 0:
		kind = "photo"
		fileID = msg.Photo[len(msg.Photo)-1].FileID
	default:
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Faqat video, fayl yoki rasm yuboring!"))
		return
	}

	// DB ga saqlash
	part := models.AnimePart{
		Anime:     &models.Anime{Id: draft.AnimeID},
		Kind:      kind,
		FileID:    fileID,
		MessageID: msgID,
		PartOrder: draft.NextOrder,
	}

	if _, err := o.Insert(&part); err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Qismni saqlashda xatolik. Qaytadan yuboring."))
		return
	}

	draft.NextOrder++

	kindLabel := map[string]string{
		"video":    "🎥 Video",
		"document": "📄 Fayl",
		"photo":    "🖼 Rasm",
	}[kind]

	text := fmt.Sprintf("✅ **%d-qism** qabul qilindi!\nTur: %s\n\nDavom ettiring yoki **/ok** deb yozing.", draft.NextOrder-1, kindLabel)

	bot.Send(tgbotapi.NewMessage(chatID, text))
}

// ============================================================
// Yakunlash va tartiblash
// ============================================================
func finishAnimeUpload(bot *tgbotapi.BotAPI, o orm.Ormer, chatID, userID int64, draft *animeDraft) {
	var parts []models.AnimePart

	_, err := o.QueryTable(new(models.AnimePart)).
		Filter("Anime__Id", draft.AnimeID).
		All(&parts)

	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Qismlarni o'qishda xatolik yuz berdi."))
		clearAnimeDraft(userID)
		return
	}

	if len(parts) == 0 {
		bot.Send(tgbotapi.NewMessage(chatID, "⚠️ Hech qanday qism qo'shilmagan. Anime saqlanmadi."))
		clearAnimeDraft(userID)
		return
	}

	// MessageID bo'yicha tartiblash (Telegram yuborish tartibi)
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].MessageID < parts[j].MessageID
	})

	// PartOrder ni qayta raqamlash
	for i := range parts {
		parts[i].PartOrder = i + 1
		o.Update(&parts[i], "PartOrder")
	}

	total := len(parts)

	// Anime jadvalidagi PartsCount ni yangilash
	animeRow := models.Anime{Id: draft.AnimeID}
	if err := o.Read(&animeRow); err == nil {
		animeRow.PartsCount = total
		o.Update(&animeRow, "PartsCount")
	}

	successText := fmt.Sprintf("✅ Anime muvaffaqiyatli saqlandi!\n\n🎬 %s\n🆔 %s\n📊 Jami qism: %d ta", draft.Name, draft.Code, total)
	bot.Send(tgbotapi.NewMessage(chatID, successText))

	clearAnimeDraft(userID)
}

// ============================================================
// Marshrutlovchi
// ============================================================
func RouteAnimeUploadState(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message, state string) bool {
	switch state {
	case "anime_name":
		HandleAnimeNameStep(bot, b, msg)
	case "anime_code":
		HandleAnimeCodeStep(bot, b, msg)
	case "anime_photo":
		HandleAnimePhotoStep(bot, b, msg)
	case "anime_videos":
		HandleAnimeVideosStep(bot, b, msg)
	default:
		return false
	}
	return true
}
