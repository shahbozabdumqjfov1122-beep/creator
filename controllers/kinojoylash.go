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

type kinoDraft struct {
	BotID     int64
	Name      string
	Code      string
	PhotoID   string
	KinoID    int64
	NextOrder int
}

var kinoDrafts = make(map[int64]*kinoDraft)

// Joylash jarayonini tozalash
func clearKinoDraft(userID int64) {
	delete(kinoDrafts, userID)
	delete(adminState, userID) // adminState boshqa faylda bo'lsa ham ishlaydi
}

// ============================================================
// 1. Kino joylashni boshlash
// ============================================================
func StartKinoUpload(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
	userID := msg.From.ID

	kinoDrafts[userID] = &kinoDraft{
		BotID:     b.Id,
		NextOrder: 1,
	}
	adminState[userID] = "kino_name"

	bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "🎬 Kino nomini kiriting:"))
}

// ============================================================
// 2. Kino nomi
// ============================================================
func HandleKinoNameStep(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
	userID := msg.From.ID
	chatID := msg.Chat.ID

	name := strings.TrimSpace(msg.Text)
	if name == "" {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Nomi bo'sh bo'lishi mumkin emas. Iltimos, nom kiriting:"))
		return
	}

	draft, ok := kinoDrafts[userID]
	if !ok {
		draft = &kinoDraft{BotID: b.Id}
		kinoDrafts[userID] = draft
	}

	draft.Name = name
	adminState[userID] = "kino_code"

	bot.Send(tgbotapi.NewMessage(chatID, "🆔 Endi kino kodini kiriting (faqat lotin harflari va raqamlar tavsiya qilinadi):"))
}

// ============================================================
// 3. Kino kodi + unique tekshirish
// ============================================================
func HandleKinoCodeStep(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
	userID := msg.From.ID
	chatID := msg.Chat.ID

	code := strings.ToLower(strings.TrimSpace(msg.Text))
	if code == "" {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Kod bo'sh bo'lishi mumkin emas. Iltimos, kod kiriting:"))
		return
	}

	draft, ok := kinoDrafts[userID]
	if !ok {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Jarayon topilmadi. Iltimos, /admin orqali qaytadan boshlang."))
		clearKinoDraft(userID)
		return
	}

	// Shu bot uchun kod bandligini tekshirish
	o := orm.NewOrm()
	existing := models.Kino{}
	err := o.QueryTable(new(models.Kino)).
		Filter("Bot__Id", b.Id).
		Filter("Code", code).
		One(&existing)

	if err == nil {
		text := fmt.Sprintf("❌ Bu kod (%s) allaqachon mavjud!\n\nKino: %s\n\nBoshqa kod kiriting:", code, existing.Name)
		bot.Send(tgbotapi.NewMessage(chatID, text))
		return
	}

	draft.Code = code
	adminState[userID] = "kino_photo"

	bot.Send(tgbotapi.NewMessage(chatID, "🌌 Kod qabul qilindi.\n\nEndi kino muqovasi (rasm) yuboring:"))
}

// ============================================================
// 4. Muqova rasmi + DB ga saqlash
// ============================================================
func HandleKinoPhotoStep(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
	userID := msg.From.ID
	chatID := msg.Chat.ID

	if msg.Photo == nil || len(msg.Photo) == 0 {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Iltimos, kino muqovasi uchun rasm yuboring."))
		return
	}

	draft, ok := kinoDrafts[userID]
	if !ok {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Jarayon topilmadi. Qaytadan boshlang."))
		clearKinoDraft(userID)
		return
	}

	photoID := msg.Photo[len(msg.Photo)-1].FileID
	draft.PhotoID = photoID

	o := orm.NewOrm()
	kinoRow := models.Kino{
		Bot:        b,
		Name:       draft.Name,
		Code:       draft.Code,
		PhotoID:    photoID,
		PartsCount: 0,
		IsActive:   true,
	}

	id, err := o.Insert(&kinoRow)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Bazaga saqlashda xatolik yuz berdi. Qaytadan urinib ko'ring."))
		clearKinoDraft(userID)
		return
	}

	draft.KinoID = id
	draft.NextOrder = 1
	adminState[userID] = "kino_videos"

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
func HandleKinoVideosStep(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
	userID := msg.From.ID
	chatID := msg.Chat.ID

	draft, ok := kinoDrafts[userID]
	if !ok || draft.KinoID == 0 {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Avval kino ma'lumotlarini to'ldiring."))
		clearKinoDraft(userID)
		return
	}

	o := orm.NewOrm()

	// /ok - yakunlash
	if msg.Text == "/ok" {
		finishKinoUpload(bot, o, chatID, userID, draft)
		return
	}

	// /cancel - bekor qilish
	if msg.Text == "/cancel" {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Kino joylash bekor qilindi."))
		clearKinoDraft(userID)
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
	part := models.KinoPart{
		Kino:      &models.Kino{Id: draft.KinoID},
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
func finishKinoUpload(bot *tgbotapi.BotAPI, o orm.Ormer, chatID, userID int64, draft *kinoDraft) {
	var parts []models.KinoPart

	_, err := o.QueryTable(new(models.KinoPart)).
		Filter("Kino__Id", draft.KinoID).
		All(&parts)

	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Qismlarni o'qishda xatolik yuz berdi."))
		clearKinoDraft(userID)
		return
	}

	if len(parts) == 0 {
		bot.Send(tgbotapi.NewMessage(chatID, "⚠️ Hech qanday qism qo'shilmagan. Kino saqlanmadi."))
		clearKinoDraft(userID)
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

	// Kino jadvalidagi PartsCount ni yangilash
	kinoRow := models.Kino{Id: draft.KinoID}
	if err := o.Read(&kinoRow); err == nil {
		kinoRow.PartsCount = total
		o.Update(&kinoRow, "PartsCount")
	}

	successText := fmt.Sprintf("✅ Kino muvaffaqiyatli saqlandi!\n\n🎬 %s\n🆔 %s\n📊 Jami qism: %d ta", draft.Name, draft.Code, total)
	bot.Send(tgbotapi.NewMessage(chatID, successText))

	clearKinoDraft(userID)
}

// ============================================================
// Marshrutlovchi
// ============================================================
func RouteKinoUploadState(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message, state string) bool {
	switch state {
	case "kino_name":
		HandleKinoNameStep(bot, b, msg)
	case "kino_code":
		HandleKinoCodeStep(bot, b, msg)
	case "kino_photo":
		HandleKinoPhotoStep(bot, b, msg)
	case "kino_videos":
		HandleKinoVideosStep(bot, b, msg)
	default:
		return false
	}
	return true
}
