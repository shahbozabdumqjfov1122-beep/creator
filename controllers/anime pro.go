package controllers

import (
	"creator/models"
	"encoding/json"
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type AdData struct {
	MediaType  string
	FileID     string
	Caption    string
	ButtonText string
	AnimeCode  string
	ChatID     int64
	MessageID  int
}

const (
	StateWaitingForAdMedia      = "waiting_for_ad_media"
	StateWaitingForAdCaption    = "waiting_for_ad_caption"
	StateWaitingForAdButtonText = "waiting_for_ad_btn_text"
	StateWaitingForAdAnimeCode  = "waiting_for_ad_anime_code"
)

var (
	adminStateMu sync.RWMutex
)
var (
	pendingAds   = make(map[int64]AdData)
	pendingAdsMu sync.RWMutex
)

func HandleAnimeBotMessagePro(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
	userID := msg.From.ID
	chatID := msg.Chat.ID
	botUser := GetOrCreateBotUser(b, userID, msg.From)
	if botUser.IsBlocked {
		sendUserBot(bot, chatID, "🚫 Siz botdan foydalanishdan bloklangansiz!")
		return
	}
	isNewCommand :=
		msg.Text == "/admin" ||
			msg.Text == "/addanime" ||
			msg.Text == "/addchannel" ||
			msg.Text == "/delchannel" ||
			msg.Text == "/ok" ||
			msg.Text == "/vipnarx" ||
			msg.Text == "➕ Anime joylash" ||
			msg.Text == "➕ Kanal qo‘shish" ||
			msg.Text == "➖ Kanall o‘chirish" ||
			msg.Text == "✏️ Animeni tahrirlash" ||
			msg.Text == "🗑 Animeni o‘chirish" ||
			msg.Text == "👥 Foydalanuvchilar" ||
			msg.Text == "📊 Statistika" ||
			msg.Text == "🗑 VIP narxni o'chirish" ||
			msg.Text == "✏️ VIP narxni tahrirlash" ||
			msg.Text == "⭐ VIP narx qo'shish" ||
			msg.Text == "✍️ Matn" ||
			msg.Text == "📬 Reklama yuborish" ||
			msg.Text == "📢 Kanal qo‘shish" ||
			msg.Text == "👥 Hammaga" ||
			msg.Text == "⭐ VIP'larga" ||
			msg.Text == "🗑 Kanal o'chirish" ||
			msg.Text == "👤 Oddiylarga" ||
			msg.Text == "👤 Adminlar" ||
			msg.Text == "➕ Admin qo'shish" ||
			msg.Text == "➖ Admin o'chirish" ||
			msg.Text == "📋 Adminlar ro'yxati" ||
			msg.Text == "📋 Blok ro'yxati" ||
			msg.Text == "📋 VIP ro'yxati" ||
			msg.Text == "⬅️ Orqaga" ||
			msg.Text == "Orqaga" ||
			msg.Text == "/delanime" ||
			msg.Text == "🎬 qismli anime joylash" ||
			msg.Text == "/editanime" ||
			msg.Text == "Kanall qo‘shish"

	if isNewCommand && msg.Text != "/ok" && isAdmin(b, userID) {
		clearAnimeDraft(userID)
		mu.Lock()
		delete(adminState, userID)
		delete(adminTempChannel, userID)
		mu.Unlock()
	}
	if isAdmin(b, userID) {
		mu.Lock()
		state, exists := adminState[userID]
		mu.Unlock()

		if exists {
			if RouteAnimeUploadState(bot, b, msg, state) {
				return
			}

			if RouteAnimeEditStatePro(bot, b, msg, state) {
				return
			}

			if RouteQuickAnimeStatePro(bot, b, msg, state) {
				return
			}

			if RouteUserManagementStatePro(bot, b, msg, state) {
				return
			}
			if RouteBotSettingsStatePro(bot, b, msg, state) {
				return
			}
			if HandleAdCreationSteps(bot, b, msg) {
				return
			}
			if strings.HasPrefix(state, "wait_vip_edit_name:") ||
				strings.HasPrefix(state, "wait_vip_edit_price:") ||
				state == "wait_channel" || state == "wait_link" ||
				state == "wait_vip_name" || state == "wait_vip_price" {
				HandleAdminCommands(bot, b, msg)
				return
			}

			if strings.HasPrefix(state, "waiting_broadcast_message:") {
				target := strings.TrimPrefix(state, "waiting_broadcast_message:")

				mu.Lock()
				delete(adminState, userID)
				mu.Unlock()

				RunBroadcastPro(bot, b, msg, target)
				return
			}
			if state == "wait_promo_channel" {
				HandlePromoChannelAdd(bot, b, msg)
				return
			}
			if msg.Text == "Orqaga" || msg.Text == "/cancel" {
				mu.Lock()
				delete(adminState, userID) // Admin holatini tozalaymiz
				mu.Unlock()

				sendUserBot(bot, chatID, "Amaliyot bekor qilindi va bosh menyuga qaytdingiz.")
				return
			}
		}
		switch msg.Text {
		case "/admin":
			if b.BotType.Code == "animepro" {
				showAdminPanelPropr(bot, chatID)
			} else {
				showAdminPanelPro(bot, chatID)
			}
			return

		case "➕ Anime joylash", "Anime joylash", "/addanime":
			StartAnimeUpload(bot, b, msg)
			return

		case "🎬 qismli anime joylash", "qismli anime joylash":
			StartQuickAnimeUploadPro(bot, b, msg)
			return

		case "➕ Kanal qo‘shish", "Kanall qo‘shish", "/addchannel":
			HandleAdminCommands(bot, b, msg)
			return

		case "➖ Kanal o‘chirish", "Kanall o'chirish", "/delchannel":
			ShowChannelsToDelete(bot, b, chatID)
			return

		case "💎 vip narx qo'shish", "vip narx qo'shish", "/vipnarx":
			HandleAdminCommands(bot, b, msg)
			return

		case "🗑 VIP narxni o'chirish", "VIP narxni o'chirish":
			showVipListForAction(bot, b, chatID, userID, "delete")
			return

		case "📢 Kanalga reklama yuborish", "Kanalga reklama yuborish":
			StartAdCreation(bot, chatID, userID)
			return

		case "✏️ VIP narxni tahrirlash", "VIP narxni tahrirlash":
			showVipListForAction(bot, b, chatID, userID, "edit")
			return

		case "✏️ Animeni tahrirlash", "Animeni tahrirlash", "/editanime":
			mu.Lock()
			adminState[userID] = "waiting_edit_code"
			mu.Unlock()

			sendUserBot(bot, chatID, "✏️Tahrirlash qismi\n\nO'zgartirmoqchi bo'lgan anime kodini yozib yuboring:")
			return

		case "📢 Kanal qo‘shish", "Kanal qo‘shish":
			mu.Lock()
			adminState[userID] = "wait_promo_channel"
			mu.Unlock()

			sendUserBot(
				bot,
				chatID,
				"📢 Reklama yuboriladigan kanal ID sini yuboring.\n\n"+
					"Masalan:\n"+
					"`-1001234567890`",
			)
			return

		case "🗑 Kanal o'chirish", "Kanal o'chirish":
			showPromoChannelsForDelete(bot, b, chatID)
			return

		case "🗑 Animeni o‘chirish", "Animeni o'chirish", "/delanime":
			mu.Lock()
			adminState[userID] = "waiting_delete_code"
			mu.Unlock()

			sendUserBot(bot, chatID, "🗑 *Animeni o'chirish*\n\nO'chirmoqchi bo'lgan anime **kodini** yozib yuboring:")
			return

		case "✍️ Matn sozlash", "Matn sozlash":
			log.Printf("🟦 '✍️ Matn' case ishga tushdi, UserID=%d, ChatID=%d", userID, chatID)
			showBotNoteMenuPro(bot, b, chatID, userID)
			return

		case "👥 Foydalanuvchilar", "Foydalanuvchilar":
			if b.BotType.Code == "animepro" {
				showUsersPanelPropr(bot, chatID)
			} else {
				showUsersPanelPro(bot, chatID)
			}
			return

		case "💎 vip sozlash", "vip sozlash":
			if b.BotType.Code == "animepro" {
				showVIPPanelPropr(bot, chatID)
			} else {
				showVIPPanelPro(bot, chatID)
			}
			return

		case "📢 Reklama sozlash", "Reklama sozlash":
			if b.BotType.Code == "animepro" {
				showkanalPanelPropr(bot, chatID)
			} else {
				showkanalPanelPro(bot, chatID)
			}
			return

		case "⭐ VIP qo'shish", "VIP qo'shish":
			startVipAddPro(bot, chatID, userID)
			return

		case "🚫 VIP o'chirish", "VIP o'chirish":
			startVipRemovePro(bot, chatID, userID)
			return

		case "📋 VIP ro'yxati", "VIP ro'yxati":
			showUserListPro(bot, chatID, b.Id, true, false)
			return

		case "📋 Blok ro'yxati", "Blok ro'yxati":
			showUserListPro(bot, chatID, b.Id, false, true)
			return

		case "⛔ Blok qo'shish", "Blok qo'shish":
			startBlockAddPro(bot, chatID, userID)
			return

		case "✅ Blok o'chirish", "Blok o'chirish":
			startBlockRemovePro(bot, chatID, userID)
			return

		case "⬅️ Orqaga", "Orqaga":
			if b.BotType.Code == "animepro" {
				showAdminPanelPropr(bot, chatID)
			} else {
				showAdminPanelPro(bot, chatID)
			}
			return

		case "📬 Reklama yuborish", "Reklama yuborish":
			if b.BotType.Code == "animepro" {
				startBroadcastPropr(bot, chatID)
			} else {
				startBroadcastPro(bot, chatID)
			}
			return

		case "👥 Hammaga", "Hammaga":
			mu.Lock()
			adminState[userID] = "waiting_broadcast_message:all"
			mu.Unlock()
			sendUserBot(bot, chatID, "✉️ Endi yubormoqchi bo'lgan xabarni jo'nating (matn, rasm, video yoki forward qilingan xabar):")
			return

		case "⭐ VIP'larga", "VIP'larga":
			mu.Lock()
			adminState[userID] = "waiting_broadcast_message:vip"
			mu.Unlock()
			sendUserBot(bot, chatID, "✉️ Endi yubormoqchi bo'lgan xabarni jo'nating (matn, rasm, video yoki forward qilingan xabar):")
			return

		case "👤 Oddiylarga", "Oddiylarga":
			mu.Lock()
			adminState[userID] = "waiting_broadcast_message:regular"
			mu.Unlock()
			sendUserBot(bot, chatID, "✉️ Endi yubormoqchi bo'lgan xabarni jo'nating (matn, rasm, video yoki forward qilingan xabar):")
			return

		case "➕ Admin qo'shish", "Admin qo'shish":
			startAdminAddPro(bot, chatID, userID)
			return

		case "➖ Admin o'chirish", "Admin o'chirish":
			showAdminsListPro(bot, b, chatID)
			return

		case "📋 Adminlar ro'yxati", "Adminlar ro'yxati":
			showAdminsListPro(bot, b, chatID)
			return

		case "/ok":
			sendUserBot(bot, chatID, "💡 Hozirda faol yuklash jarayoni mavjud emas.")
			return

		case "📊 Statistika", "Statistika":
			showStatisticsPro(bot, b, chatID)
			return

		case "📤 Anime sozlash", "Anime sozlash":
			if b.BotType.Code == "animepro" {
				startAnimeyuklashPropr(bot, chatID)
			} else {
				startAnimeyuklashPro(bot, chatID)
			}
			return

		case "📢 Kanallarni sozlash", "Kanallarni sozlash":
			if b.BotType.Code == "animepro" {
				showAdminsPanelPropr(bot, chatID)
			} else {
				showAdminsPanelPro(bot, chatID)
			}
			return

		case "🗄 Asosiy sozlamalar", "Asosiy sozlamalar":
			if b.BotType.Code == "animepro" {
				showAsosiyPanelPropr(bot, chatID)
			} else {
				showAsosiyPanelPro(bot, chatID)
			}
			return

		default:
			//if strings.HasPrefix(msg.Text, "/") {
			//	sendUserBot(bot, chatID, "/admin")
			//	return
			//}
		}
	}
	if !botUser.IsVip && !CheckSubscription(bot, b, userID) {
		mu.Lock()
		pendingSearch[userID] = msg.Text // 🎯 nima qidirgan bo'lsa saqlab qolamiz
		mu.Unlock()

		ShowMembership(bot, b, chatID, userID)
		return
	}
	if strings.HasPrefix(msg.Text, "/start") {
		handleAnimeStartPro(bot, b, msg)
		return
	}

	switch msg.Text {
	case "/start":
		handleAnimeStartPro(bot, b, msg)
		return

	case "/help":
		sendUserBot(bot, chatID, "🎌 Anime kodini yozing...")
		return
	case "reyting", "/reyting", "Reyting", "REYTING", "рейтинг", "Рейтинг":
		showTopAnimePro(bot, b, chatID)
		return
	default:
		if strings.HasPrefix(msg.Text, "/") {
			sendUserBot(bot, chatID, "/admin")
			return
		}
		handleAnimeByCodePro(bot, b, msg, msg.Text)
		return
	}
}

func StartQuickAnimeUploadPro(bot *tgbotapi.BotAPI, _ *models.CreatedBot, msg *tgbotapi.Message) {
	userID := msg.From.ID
	chatID := msg.Chat.ID

	mu.Lock()
	adminState[userID] = "waiting_quick_name"
	mu.Unlock()

	sendUserBot(bot, chatID, "Anime haqida malumot yozib yuboring:")
}

func RouteQuickAnimeStatePro(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message, state string) bool {
	userID := msg.From.ID
	chatID := msg.Chat.ID

	if state == "waiting_quick_name" {
		name := strings.TrimSpace(msg.Text)
		if name == "" {
			sendUserBot(bot, chatID, "❌ Iltimos, nomni matn ko'rinishida yuboring:")
			return true
		}

		mu.Lock()
		quickAnimeTemp[userID] = name
		adminState[userID] = "waiting_quick_file"
		mu.Unlock()

		sendUserBot(bot, chatID, "📎 Endi shu anime uchun video, rasm yoki hujjat yuboring:")
		return true
	}

	if state == "waiting_quick_file" {
		var fileID, kind, photoID, coverKind string

		if msg.Video != nil {
			fileID = msg.Video.FileID
			kind = "video"
			coverKind = ""
		} else if msg.Document != nil {
			fileID = msg.Document.FileID
			kind = "document"
			coverKind = ""
		} else if msg.Photo != nil && len(msg.Photo) > 0 {
			fileID = msg.Photo[len(msg.Photo)-1].FileID
			kind = "photo"
			photoID = fileID
			coverKind = "photo"
		} else {
			sendUserBot(bot, chatID, "❌ Iltimos, faqat video, rasm yoki hujjat yuboring:")
			return true
		}

		mu.Lock()
		name := quickAnimeTemp[userID]
		delete(quickAnimeTemp, userID)
		delete(adminState, userID)
		mu.Unlock()

		o := orm.NewOrm()

		var code string
		for {
			code = generateRandomCodePro(8)
			exist := o.QueryTable(new(models.Anime)).
				Filter("Bot__Id", b.Id).
				Filter("Code", code).
				Exist()
			if !exist {
				break
			}
		}

		anime := models.Anime{
			Bot:        b,
			Name:       name,
			Code:       code,
			PhotoID:    photoID,
			CoverKind:  coverKind,
			PartsCount: 1,
		}

		if _, err := o.Insert(&anime); err != nil {
			log.Printf("Tez joylashda anime yaratishda xatolik: %v", err)
			sendUserBot(bot, chatID, "❌ Anime yaratishda xatolik yuz berdi.")
			return true
		}

		part := models.AnimePart{
			Anime:     &anime,
			PartOrder: 1,
			FileID:    fileID,
			Kind:      kind,
		}
		if _, err := o.Insert(&part); err != nil {
			log.Printf("Tez joylashda qism qo'shishda xatolik: %v", err)
		}

		link := fmt.Sprintf("https://t.me/%s?start=%s", bot.Self.UserName, code)

		text := fmt.Sprintf(
			"✅ *Anime muvaffaqiyatli joylandi!*\n\n📌 Nomi: %s\n🎬 Qismlar: 1\n🔗 Havola: `%s`",
			name, link,
		)
		reply := tgbotapi.NewMessage(chatID, text)
		reply.ParseMode = "Markdown"
		bot.Send(reply)
		return true
	}

	return false
}

func generateRandomCodePro(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// odi
func showAdminPanelPro(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📊 Statistika"),
			tgbotapi.NewKeyboardButton("👥 Foydalanuvchilar"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📤 Anime sozlash"),
			tgbotapi.NewKeyboardButton("📢 Kanallarni sozlash"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🗄 Asosiy sozlamalar"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📬 Reklama yuborish"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "admin")
	msg.ReplyMarkup = keyboard

	if _, err := bot.Send(msg); err != nil {
		log.Printf("🔴 showAdminPanel: xabar yuborishda xatolik: %v", err)
	}
}

func showUsersPanelPro(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⛔ Blok qo'shish"),
			tgbotapi.NewKeyboardButton("✅ Blok o'chirish"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📋 Blok ro'yxati"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("➕ Admin qo'shish"),
			tgbotapi.NewKeyboardButton("➖ Admin o'chirish"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📋 Adminlar ro'yxati"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⬅️ Orqaga"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "👥 Foydalanuvchilarni boshqarish\n\nKerakli amalni tanlang:")
	msg.ReplyMarkup = keyboard

	if _, err := bot.Send(msg); err != nil {
		log.Printf("🔴 showUsersPanel: xabar yuborishda xatolik: %v", err)
	}
}

func startBroadcastPro(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("👥 Hammaga"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⭐ VIP'larga"),
			tgbotapi.NewKeyboardButton("👤 Oddiylarga"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⬅️ Orqaga"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "📢 Reklama yuborish\n\nKimlarga yuborilsin?")
	msg.ReplyMarkup = keyboard

	if _, err := bot.Send(msg); err != nil {
		log.Printf("🔴 startBroadcast: xabar yuborishda xatolik: %v", err)
	}
}

func startAnimeyuklashPro(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("➕ Anime joylash"),
			tgbotapi.NewKeyboardButton("🗑 Animeni o'chirish"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("✏️ Animeni tahrirlash"),
			tgbotapi.NewKeyboardButton("🎬 qismli anime joylash"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⬅️ Orqaga"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "📋 Quyidagilardan birini tanlang")
	msg.ReplyMarkup = keyboard

	if _, err := bot.Send(msg); err != nil {
		log.Printf("🔴 startAnimeyuklash: xabar yuborishda xatolik: %v", err)
	}
}

func showAdminsPanelPro(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("➕ Kanal qo‘shish"),
			tgbotapi.NewKeyboardButton("➖ Kanal o'chirish"),
		), tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⬅️ Orqaga"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "Quyidagilardan birini tanlang")
	msg.ReplyMarkup = keyboard

	if _, err := bot.Send(msg); err != nil {
		log.Printf("🔴 startAnimeyuklash: xabar yuborishda xatolik: %v", err)
	}
}

func showkanalPanelPro(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(

		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📢 Kanalga reklama yuborish"),
		), tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📢 Kanal qo‘shish"),
			tgbotapi.NewKeyboardButton("🗑 Kanal o'chirish"),
		), tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⬅️ Orqaga"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "Kerakli amalni tanlang:")
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

func showAsosiyPanelPro(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("✍️ Matn sozlash"),
			tgbotapi.NewKeyboardButton("💎 vip sozlash"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📢 Reklama sozlash"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⬅️ Orqaga"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "Kerakli amalni tanlang:")
	msg.ReplyMarkup = keyboard

	if _, err := bot.Send(msg); err != nil {
		log.Printf("🔴 showAsosiyPanel: xabar yuborishda xatolik: %v", err)
	}
}

func showVIPPanelPro(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(

		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("💎 vip narx qo'shish"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🗑 VIP narxni o'chirish"),
			tgbotapi.NewKeyboardButton("✏️ VIP narxni tahrirlash"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📋 VIP ro'yxati"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⭐ VIP qo'shish"),
			tgbotapi.NewKeyboardButton("🚫 VIP o'chirish"),
		),

		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⬅️ Orqaga"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "Kerakli amalni tanlang:")
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

// pr
func showUsersPanelPropr(bot *tgbotapi.BotAPI, chatID int64) {
	keyboardJSON := `{
		"keyboard": [
			[
				{"text": "Blok qo'shish", "icon_custom_emoji_id": "5944970130554359187"},
				{"text": "Blok o'chirish", "icon_custom_emoji_id": "5922612721244704425"}
			],
			[
				{"text": "Blok ro'yxati", "icon_custom_emoji_id": "5942877472163892475"}
			],
			[
				{"text": "Admin qo'shish", "icon_custom_emoji_id": "5920090136627908485"},
				{"text": "Admin o'chirish", "icon_custom_emoji_id": "5886496611835581345"}
			],
			[
				{"text": "Adminlar ro'yxati", "icon_custom_emoji_id": "5942877472163892475"}
			],
			[
				{"text": "Orqaga", "icon_custom_emoji_id": "5877629862306385808"}
			]
		],
		"resize_keyboard": true
	}`

	params := tgbotapi.Params{}
	params.AddNonEmpty("chat_id", strconv.FormatInt(chatID, 10))
	params.AddNonEmpty("text", "👥 Foydalanuvchilarni boshqarish\n\nKerakli amalni tanlang:")
	params.AddNonEmpty("reply_markup", keyboardJSON)

	if _, err := bot.MakeRequest("sendMessage", params); err != nil {
		log.Printf("🔴 showUsersPanelPro: xabar yuborishda xatolik: %v", err)
	}
}

func startBroadcastPropr(bot *tgbotapi.BotAPI, chatID int64) {
	keyboardJSON := `{
		"keyboard": [
			[
				{"text": "Hammaga", "icon_custom_emoji_id": "5942877472163892475"}
			],
			[
				{"text": "VIP'larga", "icon_custom_emoji_id": "5920090136627908485"},
				{"text": "Oddiylarga", "icon_custom_emoji_id": "5920344347152224466"}
			],
			[
				{"text": "Orqaga", "icon_custom_emoji_id": "5877629862306385808"}
			]
		],
		"resize_keyboard": true
	}`

	params := tgbotapi.Params{}
	params.AddNonEmpty("chat_id", strconv.FormatInt(chatID, 10))
	params.AddNonEmpty("text", "📢 Reklama yuborish\n\nKimlarga yuborilsin?")
	params.AddNonEmpty("reply_markup", keyboardJSON)

	if _, err := bot.MakeRequest("sendMessage", params); err != nil {
		log.Printf("🔴 startBroadcastPro: xabar yuborishda xatolik: %v", err)
	}
}

func showAdminPanelPropr(bot *tgbotapi.BotAPI, chatID int64) {
	keyboardJSON := `{
		"keyboard": [
			[
				{"text": "Statistika", "icon_custom_emoji_id": "6006038041448156880"},
				{"text": "Foydalanuvchilar", "icon_custom_emoji_id": "5886412370347036129"}
			],
			[
				{"text": "Anime sozlash", "icon_custom_emoji_id": "5877260593903177342"},
				{"text": "Kanallarni sozlash", "icon_custom_emoji_id": "5988023995125993550"}
			],
			[
				{"text": "Asosiy sozlamalar", "icon_custom_emoji_id": "5963312935148195483"}
			],
			[
				{"text": "Reklama yuborish", "icon_custom_emoji_id": "5967280668885913944"}
			]
		],
		"resize_keyboard": true
	}`

	params := tgbotapi.Params{}
	params.AddNonEmpty("chat_id", strconv.FormatInt(chatID, 10))
	params.AddNonEmpty("text", "admin")
	params.AddNonEmpty("reply_markup", keyboardJSON)

	if _, err := bot.MakeRequest("sendMessage", params); err != nil {
		log.Printf("🔴 showAdminPanelPro: xabar yuborishda xatolik: %v", err)
	}
}

func startAnimeyuklashPropr(bot *tgbotapi.BotAPI, chatID int64) {
	keyboardJSON := `{
		"keyboard": [
			[
				{"text": "Anime joylash", "icon_custom_emoji_id": "5877307202888273539"},
				{"text": "Animeni o'chirish", "icon_custom_emoji_id": "5879896690210639947"}
			],
			[
				{"text": "Animeni tahrirlash", "icon_custom_emoji_id": "5879841310902324730"},
				{"text": "qismli anime joylash", "icon_custom_emoji_id": "5994636050033545139"}
			],
			[
				{"text": "Orqaga", "icon_custom_emoji_id": "5877629862306385808"}
			]
		],
		"resize_keyboard": true
	}`

	params := tgbotapi.Params{}
	params.AddNonEmpty("chat_id", strconv.FormatInt(chatID, 10))
	params.AddNonEmpty("text", " Quyidagilardan birini tanlang")
	params.AddNonEmpty("reply_markup", keyboardJSON)

	if _, err := bot.MakeRequest("sendMessage", params); err != nil {
		log.Printf("🔴 startAnimeyuklashPro: xabar yuborishda xatolik: %v", err)
	}
}

func showkanalPanelPropr(bot *tgbotapi.BotAPI, chatID int64) {
	keyboardJSON := `{
		"keyboard": [
			[
				{"text": "Kanalga reklama yuborish", "icon_custom_emoji_id": "5771695636411847302"},
				{"text": "Kanal qo‘shish", "icon_custom_emoji_id": "5771868281212245617"}
			],
			[
				{"text": "Kanal o'chirish", "icon_custom_emoji_id": "5771511103141975115"}
			],[
				{"text": "Orqaga", "icon_custom_emoji_id": "5877629862306385808"}
			]
		],
		"resize_keyboard": true
	}`

	params := tgbotapi.Params{}
	params.AddNonEmpty("chat_id", strconv.FormatInt(chatID, 10))
	params.AddNonEmpty("text", "Kerakli amalni tanlang:")
	params.AddNonEmpty("parse_mode", "HTML")
	params.AddNonEmpty("reply_markup", keyboardJSON)

	if _, err := bot.MakeRequest("sendMessage", params); err != nil {
		log.Printf("🔴 showAsosiyPanelPro: xabar yuborishda xatolik: %v", err)
	}
}

func showAsosiyPanelPropr(bot *tgbotapi.BotAPI, chatID int64) {
	keyboardJSON := `{
		"keyboard": [
			[
				{"text": "Matn sozlash", "icon_custom_emoji_id": "5764779661028495989"},
				{"text": "vip sozlash", "icon_custom_emoji_id": "5807465992363710697"}
			],
			[
				{"text": "Reklama sozlash", "icon_custom_emoji_id": "5771695636411847302"}
			],
			[
				{"text": "Orqaga", "icon_custom_emoji_id": "5877629862306385808"}
			]
		],
		"resize_keyboard": true
	}`

	params := tgbotapi.Params{}
	params.AddNonEmpty("chat_id", strconv.FormatInt(chatID, 10))
	params.AddNonEmpty("text", "Kerakli amalni tanlang:")
	params.AddNonEmpty("parse_mode", "HTML")
	params.AddNonEmpty("reply_markup", keyboardJSON)

	if _, err := bot.MakeRequest("sendMessage", params); err != nil {
		log.Printf("🔴 showAsosiyPanelPro: xabar yuborishda xatolik: %v", err)
	}
}

func showAdminsPanelPropr(bot *tgbotapi.BotAPI, chatID int64) {
	keyboardJSON := `{
		"keyboard": [
			[
				{"text": "Kanall qo‘shish", "icon_custom_emoji_id": "5771868281212245617"},
				{"text": "Kanall o'chirish", "icon_custom_emoji_id": "5771511103141975115"}
			],
			[
				{"text": "Orqaga", "icon_custom_emoji_id": "5877629862306385808"}
			]
		],
		"resize_keyboard": true
	}`

	params := tgbotapi.Params{}
	params.AddNonEmpty("chat_id", strconv.FormatInt(chatID, 10))
	params.AddNonEmpty("text", "📢 Kanallarni sozlash / boshqarish\n\nKerakli amalni tanlang:")
	params.AddNonEmpty("reply_markup", keyboardJSON)

	if _, err := bot.MakeRequest("sendMessage", params); err != nil {
		log.Printf("🔴 showAdminsPanelPro: xabar yuborishda xatolik: %v", err)
	}
}

func showVIPPanelPropr(bot *tgbotapi.BotAPI, chatID int64) {
	keyboardJSON := `{
		"keyboard": [
			[
				{"text": "vip narx qo'shish", "icon_custom_emoji_id": "5807465992363710697"},
				{"text": "VIP narxni o'chirish", "icon_custom_emoji_id": "5807465992363710697"}
			],
			[
				{"text": "VIP narxni tahrirlash", "icon_custom_emoji_id": "5879841310902324730"}
			],
			[
				{"text": "VIP ro'yxati", "icon_custom_emoji_id": "5942877472163892475"}
			],
			[
				{"text": "VIP qo'shish", "icon_custom_emoji_id": "5920090136627908485"},
				{"text": "VIP o'chirish", "icon_custom_emoji_id": "5886496611835581345"}
			],
			[
				{"text": "Orqaga", "icon_custom_emoji_id": "5877629862306385808"}
			]
		],
		"resize_keyboard": true
	}`

	params := tgbotapi.Params{}
	params.AddNonEmpty("chat_id", strconv.FormatInt(chatID, 10))
	params.AddNonEmpty("text", "Kerakli amalni tanlang:")
	params.AddNonEmpty("parse_mode", "HTML")
	params.AddNonEmpty("reply_markup", keyboardJSON)

	if _, err := bot.MakeRequest("sendMessage", params); err != nil {
		log.Printf("🔴 showVIPPanelPropr: xabar yuborishda xatolik: %v", err)
	}
}

//qolgani

func showBotNoteMenuPro(bot *tgbotapi.BotAPI, b *models.CreatedBot, chatID int64, userID int64) {
	// Har safar bazadan eng so'nggi qiymatni olamiz — chunki delete_bot_note/edit_bot_note
	// callback'lari xotiradagi b obyektiga to'g'ridan-to'g'ri kira olmaydi.
	o := orm.NewOrm()
	var fresh models.CreatedBot
	if err := o.QueryTable(new(models.CreatedBot)).Filter("Id", b.Id).One(&fresh); err == nil {
		b.Note = fresh.Note
	} else {
		log.Printf("🔴 showBotNoteMenuPro: bazadan yangilashda xatolik: %v", err)
	}

	if b.Note == "" {
		mu.Lock()
		adminState[userID] = "waiting_bot_note"
		mu.Unlock()

		_, sendErr := bot.Send(tgbotapi.NewMessage(chatID, "✍️ Hozircha matn qo'shilmagan.\n\nBarcha anime'lar ostida doimiy chiqib turadigan matnni kiriting:"))
		if sendErr != nil {
			log.Printf("🔴 showBotNoteMenuPro: xabar yuborishda xatolik: %v", sendErr)
		}
		return
	}

	text := fmt.Sprintf("✍️ Joriy matn:\n\n%s", b.Note)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✏️ Tahrirlash", fmt.Sprintf("edit_bot_note:%d", b.Id)),
			tgbotapi.NewInlineKeyboardButtonData("🗑 O'chirish", fmt.Sprintf("delete_bot_note:%d", b.Id)),
		),
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard

	_, sendErr := bot.Send(msg)
	if sendErr != nil {
		log.Printf("🔴 showBotNoteMenuPro: matnli xabar yuborishda xatolik: %v", sendErr)
	}
}

func sendProtectedPhotoPro(bot *tgbotapi.BotAPI, chatID int64, fileID, caption string, keyboard *tgbotapi.InlineKeyboardMarkup, protect bool) error {
	params := tgbotapi.Params{}
	params.AddNonEmpty("chat_id", strconv.FormatInt(chatID, 10))
	params.AddNonEmpty("photo", fileID)
	params.AddNonEmpty("parse_mode", "HTML") // ← shu qatorni qo‘shing
	params.AddNonEmpty("caption", caption)
	if keyboard != nil {
		data, err := json.Marshal(keyboard)
		if err == nil {
			params.AddNonEmpty("reply_markup", string(data))
		}
	}
	return sendWithProtect(bot, "sendPhoto", params, protect)
}

func sendProtectedDocumentPro(bot *tgbotapi.BotAPI, chatID int64, fileID, caption string, protect bool) error {
	params := tgbotapi.Params{}
	params.AddNonEmpty("chat_id", strconv.FormatInt(chatID, 10))
	params.AddNonEmpty("document", fileID)
	params.AddNonEmpty("caption", caption)
	return sendWithProtect(bot, "sendDocument", params, protect)
}

func sendProtectedTextPro(bot *tgbotapi.BotAPI, chatID int64, text string, keyboard *tgbotapi.InlineKeyboardMarkup, protect bool) error {
	params := tgbotapi.Params{}
	params.AddNonEmpty("chat_id", strconv.FormatInt(chatID, 10))
	params.AddNonEmpty("parse_mode", "HTML") // ← shu qatorni qo‘shing
	params.AddNonEmpty("text", text)
	if keyboard != nil {
		data, err := json.Marshal(keyboard)
		if err == nil {
			params.AddNonEmpty("reply_markup", string(data))
		}
	}
	return sendWithProtect(bot, "sendMessage", params, protect)
}

func handleAnimeStartPro(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
	args := strings.TrimSpace(strings.TrimPrefix(msg.Text, "/start"))

	if args != "" {
		handleAnimeByCodePro(bot, b, msg, args)
		return
	}

	text := `<tg-emoji emoji-id="5960714428394507968">🏷</tg-emoji> /reyting - Top Anime
<tg-emoji emoji-id="5899757765743615694">📥</tg-emoji> /admin - Admin uchun 
<tg-emoji emoji-id="5899757765743615694"></tg-emoji>
<tg-emoji emoji-id="5987802868734760945">🆔</tg-emoji> Anime nomi yoki kodini kiriting:`

	sendUserBot(bot, msg.Chat.ID, text)
}

func handleAnimeByCodePro(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message, code string) {
	fmt.Println("========== ANIME SEARCH ==========")
	fmt.Println("BotID:", b.Id)
	fmt.Println("Code:", code)

	userID := msg.From.ID
	botUser := GetOrCreateBotUser(b, userID, msg.From)

	// 2. Protect qoida:
	//    - Admin → protect = false (uzata oladi)
	//    - VIP va oddiy → protect = true (uzata olmaydi)
	protectContent := !isAdmin(b, userID)

	o := orm.NewOrm()
	var anime models.Anime

	// 1. Avval animeni bazadan qidirib topamiz
	err := o.QueryTable(new(models.Anime)).
		Filter("Bot__Id", b.Id).
		Filter("Code", strings.ToLower(strings.TrimSpace(code))).
		One(&anime)

	if err != nil {
		fmt.Println("ANIME NOT FOUND BY CODE, FALLBACK TO SEARCH:", err)
		// Kod bo‘yicha topilmasa → nom bo‘yicha qidiruv
		handleAnimeSearchPro(bot, b, msg, code)
		return
	}

	// 2. Anime bazadan topilgandan keyin VIP ekanligini tekshiramiz:
	if anime.IsVipOnly && !botUser.IsVip && !isAdmin(b, userID) {
		sendUserBot(bot, msg.Chat.ID, "🔒 Bu anime faqat VIP foydalanuvchilar uchun.\n\nVIP bo‘lish uchun admin bilan bog‘laning.")
		return
	}

	fmt.Println("FOUND ANIME:")
	fmt.Println("ID:", anime.Id)
	fmt.Println("Name:", anime.Name)
	fmt.Println("Code:", anime.Code)
	fmt.Println("Parts:", anime.PartsCount)
	fmt.Println("CoverKind:", anime.CoverKind)
	fmt.Println("PhotoID:", anime.PhotoID)

	// 1. Birinchi o'rinda ko'rishlar sonini oshirib olamiz
	anime.ViewsCount++
	o.Update(&anime, "ViewsCount")

	// 2. Bir qismli anime bo'lsa
	if anime.PartsCount == 1 {
		sendSinglePartAnimePro(bot, o, msg.Chat.ID, &anime, b.Note, protectContent)
		return
	}

	// 3. Ko'p qismli anime uchun matn tayyorlaymiz
	caption := fmt.Sprintf(
		"%s\n\n"+
			`<tg-emoji emoji-id="5960714428394507968">🏷</tg-emoji>Jami qismlar: %d`+"\n"+
			`<tg-emoji emoji-id="5899757765743615694">📥</tg-emoji>Yuklab olishlar: %d`+"\n"+
			`<tg-emoji emoji-id="5987802868734760945">🆔</tg-emoji>Anime kodi: %s`,
		anime.Name,
		anime.PartsCount,
		anime.ViewsCount,
		anime.Code,
	)
	if b.Note != "" {
		caption += fmt.Sprintf("\n\n%s", b.Note)
	}

	var keyboard tgbotapi.InlineKeyboardMarkup
	hasKeyboard := false

	if anime.PartsCount > 0 {
		keyboard = buildAnimePartsKeyboardPro(anime.Id, anime.PartsCount, 1)
		hasKeyboard = true
	} else {
		caption += "\n⚠️ Tez orada qismlar joylanadi!"
	}

	var sendErr error

	if anime.PhotoID != "" {
		var kb *tgbotapi.InlineKeyboardMarkup
		if hasKeyboard {
			kb = &keyboard
		}
		sendErr = sendProtectedPhotoPro(bot, msg.Chat.ID, anime.PhotoID, caption, kb, protectContent)

		if sendErr != nil {
			fmt.Println("SEND PHOTO ERROR (fallback to text):", sendErr)
			sendErr = sendProtectedTextPro(bot, msg.Chat.ID, caption, kb, protectContent)
		}
	} else {
		var kb *tgbotapi.InlineKeyboardMarkup
		if hasKeyboard {
			kb = &keyboard
		}
		sendErr = sendProtectedTextPro(bot, msg.Chat.ID, caption, kb, protectContent)
	}

	if sendErr != nil {
		fmt.Println("SEND ANIME ERROR:", sendErr)
		fallback := tgbotapi.NewMessage(msg.Chat.ID, "❌ Kechirasiz, ma'lumotni yuborishda texnik xatolik yuz berdi. Iltimos, keyinroq qayta urinib ko'ring yoki admin bilan bog'laning.")
		if _, fbErr := bot.Send(fallback); fbErr != nil {
			fmt.Println("SEND FALLBACK MESSAGE ERROR:", fbErr)
		}
	}
}

func handleAnimeSearchPro(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message, query string) {
	fmt.Println("========== ANIME NAME SEARCH ==========")
	fmt.Println("BotID:", b.Id)
	fmt.Println("Query:", query)

	query = strings.TrimSpace(query)
	if query == "" {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "🔍 Qidiruv uchun biror so‘z yoki kod yozing.")
		bot.Send(reply)
		return
	}

	o := orm.NewOrm()
	var animes []models.Anime

	// Nom bo‘yicha qisman mos keladigan animelarni qidiramiz (case-insensitive)
	_, err := o.QueryTable(new(models.Anime)).
		Filter("Bot__Id", b.Id).
		Filter("Name__icontains", query). // Beego ORM da icontains = ILIKE
		OrderBy("Name").
		Limit(20). // ortiqcha ko‘p chiqmasin
		All(&animes)

	if err != nil {
		fmt.Println("SEARCH ERROR:", err)
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Qidiruvda xatolik yuz berdi. Keyinroq qayta urinib ko‘ring.")
		bot.Send(reply)
		return
	}

	if len(animes) == 0 {
		text := fmt.Sprintf("🔍 «%s» bo‘yicha hech qanday anime topilmadi.\n\nBoshqa so‘z yoki kod bilan qidirib ko‘ring.", query)
		reply := tgbotapi.NewMessage(msg.Chat.ID, text)
		bot.Send(reply)
		return
	}

	// Natijalar topildi — inline tugmalar bilan ro‘yxat chiqaramiz
	// Kodi faqat harflardan iborat bo'lgan animelar qidiruvda ko'rsatilmaydi
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, a := range animes {
		if isCodeOnlyLetters(a.Code) {
			continue
		}
		btnText := a.Name
		if a.Code != "" {
			btnText = fmt.Sprintf("%s  [%s]", a.Name, a.Code)
		}
		// Callback data: anime_select_{id}
		btn := tgbotapi.NewInlineKeyboardButtonData(btnText, fmt.Sprintf("anime_select_%d", a.Id))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
	}

	if len(rows) == 0 {
		text := fmt.Sprintf("🔍 «%s» bo‘yicha hech qanday anime topilmadi.\n\nBoshqa so‘z yoki kod bilan qidirib ko‘ring.", query)
		reply := tgbotapi.NewMessage(msg.Chat.ID, text)
		bot.Send(reply)
		return
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	text := fmt.Sprintf("«%s» bo‘yicha topilgan animelar (%d ta):\n\n", query, len(rows))
	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ReplyMarkup = keyboard

	if _, sendErr := bot.Send(reply); sendErr != nil {
		fmt.Println("SEND SEARCH RESULTS ERROR:", sendErr)
	}
}

func buildAnimePartsKeyboardPro(animeID int64, totalParts int, page int) tgbotapi.InlineKeyboardMarkup {
	if totalParts <= 0 {
		return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}}
	}

	start := (page-1)*10 + 1
	end := start + 9

	if end > totalParts {
		end = totalParts
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton

	for i := start; i <= end; i++ {
		btn := tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%d", i),
			fmt.Sprintf("anime_part:%d:%d", animeID, i),
		)
		row = append(row, btn)

		if len(row) == 5 {
			rows = append(rows, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}

	if len(row) > 0 {
		rows = append(rows, row)
	}

	var nav []tgbotapi.InlineKeyboardButton
	if page > 1 {
		nav = append(nav,
			tgbotapi.NewInlineKeyboardButtonData(
				"<",
				fmt.Sprintf("anime_page:%d:%d", animeID, page-1),
			),
		)
	}

	if end < totalParts {
		nav = append(nav,
			tgbotapi.NewInlineKeyboardButtonData(
				">",
				fmt.Sprintf("anime_page:%d:%d", animeID, page+1),
			),
		)
	}

	if len(nav) > 0 {
		rows = append(rows, nav)
	}

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func sendSinglePartAnimePro(bot *tgbotapi.BotAPI, o orm.Ormer, chatID int64, anime *models.Anime, botNote string, protectContent bool) {
	var part models.AnimePart
	err := o.QueryTable(new(models.AnimePart)).
		Filter("Anime__Id", anime.Id).
		Filter("PartOrder", 1).
		One(&part)

	if err != nil {
		fmt.Println("SINGLE PART NOT FOUND:", err)
		fallback := tgbotapi.NewMessage(chatID, "❌ Fayl topilmadi. Iltimos, admin bilan bog'laning.")
		bot.Send(fallback)
		return
	}

	caption := anime.Name
	if botNote != "" {
		caption += fmt.Sprintf("\n\n%s", botNote)
	}

	var sendErr error

	switch part.Kind {
	case "video":
		sendErr = sendProtectedVideoPro(bot, chatID, part.FileID, caption, protectContent)
	case "document":
		sendErr = sendProtectedDocumentPro(bot, chatID, part.FileID, caption, protectContent)
	case "photo":
		sendErr = sendProtectedPhotoPro(bot, chatID, part.FileID, caption, nil, protectContent)
	default:
		sendErr = fmt.Errorf("noma'lum fayl turi: %s", part.Kind)
	}

	if sendErr != nil {
		fmt.Println("SEND SINGLE PART ERROR:", sendErr)
		fallback := tgbotapi.NewMessage(chatID, "❌ Faylni yuborishda texnik xatolik yuz berdi.")
		bot.Send(fallback)
	}
}

func sendWithProtect(bot *tgbotapi.BotAPI, method string, params tgbotapi.Params, protect bool) error {
	if protect {
		params["protect_content"] = "true"
	}

	fmt.Println(">>> SEND:", method)
	fmt.Println(">>> protect:", protect)
	fmt.Println(">>> params:", params)

	resp, err := bot.MakeRequest(method, params)
	if err != nil {
		fmt.Println(">>> ERROR:", err)
		return err
	}

	fmt.Println(">>> OK:", resp.Ok)
	fmt.Println(">>> RESULT:", string(resp.Result)) // shu yerda has_protected_content ko'rinadi

	return nil
}

func sendProtectedVideoPro(bot *tgbotapi.BotAPI, chatID int64, fileID, caption string, protect bool) error {
	params := tgbotapi.Params{
		"chat_id": strconv.FormatInt(chatID, 10),
		"video":   fileID,
		"caption": caption,
	}
	if protect {
		params["protect_content"] = "true"
	}

	fmt.Println("VIDEO PARAMS:", params)
	resp, err := bot.MakeRequest("sendVideo", params)
	if err != nil {
		return err
	}
	fmt.Println("VIDEO RESULT:", string(resp.Result))
	return nil
}

func handleAnimePagePro(bot *tgbotapi.BotAPI, cb *tgbotapi.CallbackQuery) {

	parts := strings.Split(cb.Data, ":")

	animeID, _ := strconv.ParseInt(parts[1], 10, 64)
	page, _ := strconv.Atoi(parts[2])

	o := orm.NewOrm()

	var anime models.Anime
	anime.Id = animeID

	if err := o.Read(&anime); err != nil {
		return
	}

	kb := buildAnimePartsKeyboardPro(
		animeID,
		anime.PartsCount,
		page,
	)

	edit := tgbotapi.NewEditMessageReplyMarkup(
		cb.Message.Chat.ID,
		cb.Message.MessageID,
		kb,
	)

	bot.Send(edit)
}

func RouteBotSettingsStatePro(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message, state string) bool {
	userID := msg.From.ID
	chatID := msg.Chat.ID
	o := orm.NewOrm()

	// Yangi matn qo'shish (hali matn bo'lmaganda)
	if state == "waiting_bot_note" {
		newNote := msg.Text

		_, err := o.QueryTable(new(models.CreatedBot)).
			Filter("Id", b.Id).
			Update(orm.Params{"Note": newNote})

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()

		if err != nil {
			log.Printf("Bot Note saqlashda xatolik: %v", err)
			sendUserBot(bot, chatID, "❌ Matnni saqlashda xatolik yuz berdi.")
			showAsosiyPanelPro(bot, chatID)
			return true
		}

		b.Note = newNote
		sendUserBot(bot, chatID, "✅ Matn muvaffaqiyatli qo'shildi!")
		showAsosiyPanelPro(bot, chatID)
		return true
	}

	// Mavjud matnni tahrirlash (inline tugma orqali kelgan holat: "waiting_bot_note:<botID>")
	if strings.HasPrefix(state, "waiting_bot_note:") {
		newNote := msg.Text

		_, err := o.QueryTable(new(models.CreatedBot)).
			Filter("Id", b.Id).
			Update(orm.Params{"Note": newNote})

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()

		if err != nil {
			log.Printf("Bot Note tahrirlashda xatolik: %v", err)
			sendUserBot(bot, chatID, "❌ Matnni saqlashda xatolik yuz berdi.")
			showAsosiyPanelPro(bot, chatID)
			return true
		}

		b.Note = newNote
		sendUserBot(bot, chatID, "✅ Matn muvaffaqiyatli tahrirlandi!")
		return true
	}

	return false
}

func RouteAnimeEditStatePro(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message, state string) bool {
	userID := msg.From.ID
	chatID := msg.Chat.ID
	o := orm.NewOrm()

	if state == "waiting_edit_code" {
		code := strings.ToLower(strings.TrimSpace(msg.Text))
		var anime models.Anime
		err := o.QueryTable(new(models.Anime)).
			Filter("Bot__Id", b.Id).
			Filter("Code", code).
			One(&anime)

		if err != nil {
			sendUserBot(bot, chatID, "❌ Ushbu botga tegishli bunday kodli anime topilmadi.\n\nQaytadan to'g'ri kod kiriting:")
			return true
		}
		vipBtnText := "🔒 Faqat VIP'larga"
		if anime.IsVipOnly {
			vipBtnText = "🔓 Hammaga ochish"
		}
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔑 Kodni o'zgartirish", fmt.Sprintf("edit_code:%d", anime.Id)),
				tgbotapi.NewInlineKeyboardButtonData("📝 Nomni o'zgartirish", fmt.Sprintf("edit_name:%d", anime.Id)),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("➕ Qism qo'shish", fmt.Sprintf("edit_addpart:%d", anime.Id)),
				tgbotapi.NewInlineKeyboardButtonData("➖ Qism o'chirish", fmt.Sprintf("edit_delpart:%d", anime.Id)),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🖼 Rasmini o'zgartirish", fmt.Sprintf("edit_photo:%d", anime.Id)),
				tgbotapi.NewInlineKeyboardButtonData(vipBtnText, fmt.Sprintf("toggle_vip_only:%d", anime.Id)),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🗑 Butunlay o'chirish", fmt.Sprintf("delete_anime:%d", anime.Id)),
			),
		)

		caption := fmt.Sprintf("✅ Anime topildi:\n\n📌 *%s*\n🔑 Kod: `%s`\n🎬 Qismlar: %d\n\nNima qilmoqchisiz?",
			anime.Name, anime.Code, anime.PartsCount)

		if anime.PhotoID != "" {
			photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileID(anime.PhotoID))
			photo.Caption = caption
			photo.ParseMode = "Markdown"
			photo.ReplyMarkup = keyboard

			if _, sendErr := bot.Send(photo); sendErr != nil {
				fmt.Println("SEND EDIT ANIME PHOTO ERROR (fallback to text):", sendErr)
				reply := tgbotapi.NewMessage(chatID, caption)
				reply.ParseMode = "Markdown"
				reply.ReplyMarkup = keyboard
				bot.Send(reply)
			}
		} else {
			reply := tgbotapi.NewMessage(chatID, caption)
			reply.ParseMode = "Markdown"
			reply.ReplyMarkup = keyboard
			bot.Send(reply)
		}

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()
		return true
	}

	if state == "waiting_delete_code" {
		code := strings.ToLower(strings.TrimSpace(msg.Text))
		var anime models.Anime
		err := o.QueryTable(new(models.Anime)).
			Filter("Bot__Id", b.Id).
			Filter("Code", code).
			One(&anime)

		if err != nil {
			sendUserBot(bot, chatID, "❌ Bunday kodli anime topilmadi.\nQaytadan kiriting:")
			return true
		}

		animeName := anime.Name

		_, err = o.QueryTable(new(models.AnimePart)).Filter("Anime__Id", anime.Id).Delete()
		if err != nil {
			log.Printf("AnimePart o'chirishda xatolik: %v", err)
		}

		_, err = o.Delete(&anime)
		if err != nil {
			sendUserBot(bot, chatID, "❌ Animeni o'chirishda xatolik yuz berdi.")
			return true
		}

		log.Printf("🗑️ Anime o'chirildi! ID: %d, Nomi: %s", anime.Id, animeName)

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()

		sendUserBot(bot, chatID, fmt.Sprintf("✅ \"%s\" anime butunlay o'chirildi!", animeName))
		return true
	}
	if strings.HasPrefix(state, "waiting_new_code:") {
		animeID, _ := strconv.ParseInt(strings.TrimPrefix(state, "waiting_new_code:"), 10, 64)
		newCode := strings.ToLower(strings.TrimSpace(msg.Text))

		o.QueryTable(new(models.Anime)).
			Filter("Id", animeID).
			Filter("Bot__Id", b.Id).
			Update(orm.Params{"Code": newCode})

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()

		sendUserBot(bot, chatID, "✅ Anime kodi muvaffaqiyatli o'zgartirildi!")
		return true
	}

	if strings.HasPrefix(state, "waiting_new_name:") {
		animeID, _ := strconv.ParseInt(strings.TrimPrefix(state, "waiting_new_name:"), 10, 64)

		o.QueryTable(new(models.Anime)).
			Filter("Id", animeID).
			Filter("Bot__Id", b.Id).
			Update(orm.Params{"Name": msg.Text})

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()

		sendUserBot(bot, chatID, "✅ Anime nomi muvaffaqiyatli o'zgartirildi!")
		return true
	}

	if strings.HasPrefix(state, "waiting_new_part_file:") {
		animeID, _ := strconv.ParseInt(strings.TrimPrefix(state, "waiting_new_part_file:"), 10, 64)

		var anime models.Anime
		err := o.QueryTable(new(models.Anime)).
			Filter("Id", animeID).
			Filter("Bot__Id", b.Id).
			One(&anime)

		if err != nil {
			sendUserBot(bot, chatID, "❌ Xatolik: Anime topilmadi.")
			return true
		}

		var fileID, kind string
		if msg.Video != nil {
			fileID = msg.Video.FileID
			kind = "video"
		} else if msg.Document != nil {
			fileID = msg.Document.FileID
			kind = "document"
		} else if msg.Photo != nil && len(msg.Photo) > 0 {
			fileID = msg.Photo[len(msg.Photo)-1].FileID
			kind = "photo"
		} else {
			sendUserBot(bot, chatID, "❌ Iltimos, faqat video, rasm yoki hujjat yuboring:"+
				"yoki  /admin")

			return true
		}

		newPartOrder := anime.PartsCount + 1

		newPart := models.AnimePart{
			Anime:     &anime,
			PartOrder: newPartOrder,
			FileID:    fileID,
			Kind:      kind,
		}

		if _, err := o.Insert(&newPart); err == nil {
			anime.PartsCount = newPartOrder
			o.Update(&anime, "PartsCount")

			mu.Lock()
			delete(adminState, userID)
			mu.Unlock()

			sendUserBot(bot, chatID, fmt.Sprintf("✅ Yangi %d-qism muvaffaqiyatli qo'shildi!", newPartOrder))
		}
		return true
	}

	if strings.HasPrefix(state, "waiting_del_part_num:") {
		animeID, _ := strconv.ParseInt(strings.TrimPrefix(state, "waiting_del_part_num:"), 10, 64)
		partNum, err := strconv.Atoi(strings.TrimSpace(msg.Text))

		if err != nil || partNum <= 0 {
			sendUserBot(bot, chatID, "❌ Noto'g'ri qism raqami. Faqat musbat raqam kiriting:")
			return true
		}

		var anime models.Anime
		if o.QueryTable(new(models.Anime)).
			Filter("Id", animeID).
			Filter("Bot__Id", b.Id).
			One(&anime) != nil {
			sendUserBot(bot, chatID, "❌ Anime topilmadi.")
			return true
		}

		// 1. Kerakli qismni o'chiramiz
		num, err := o.QueryTable(new(models.AnimePart)).
			Filter("Anime__Id", animeID).
			Filter("PartOrder", partNum).
			Delete()

		if err != nil || num == 0 {
			sendUserBot(bot, chatID, "❌ Bunday raqamli qism topilmadi. Qaytadan urinib ko'ring:")
			return true
		}

		// 2. O'chirilgan raqamdan keyingi barcha qismlarni 1 ta pastga siljitamiz
		// Masalan: 4 o'chirilsa → 5→4, 6→5, 7→6 ...
		var partsToShift []models.AnimePart
		_, _ = o.QueryTable(new(models.AnimePart)).
			Filter("Anime__Id", animeID).
			Filter("PartOrder__gt", partNum).
			OrderBy("PartOrder").
			All(&partsToShift)

		for _, p := range partsToShift {
			p.PartOrder = p.PartOrder - 1
			o.Update(&p, "PartOrder")
		}

		// 3. PartsCount ni yangilaymiz
		if anime.PartsCount > 0 {
			anime.PartsCount -= 1
			o.Update(&anime, "PartsCount")
		}

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()

		sendUserBot(bot, chatID, fmt.Sprintf("✅ %d-qism muvaffaqiyatli o'chirildi!\nQolgan qismlar qayta raqamlandi.", partNum))
		return true
	}
	if strings.HasPrefix(state, "waiting_new_photo:") {
		if msg.Photo == nil || len(msg.Photo) == 0 {
			sendUserBot(bot, chatID, "❌ Iltimos, faqat rasm yuboring:")
			return true
		}
		animeID, _ := strconv.ParseInt(strings.TrimPrefix(state, "waiting_new_photo:"), 10, 64)
		newPhotoID := msg.Photo[len(msg.Photo)-1].FileID

		o.QueryTable(new(models.Anime)).
			Filter("Id", animeID).
			Filter("Bot__Id", b.Id).
			Update(orm.Params{"PhotoID": newPhotoID})

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()

		sendUserBot(bot, chatID, "✅ Anime bosh sahifa surati muvaffaqiyatli o'zgartirildi!")
		return true
	}

	return false
}

func showUserListPro(bot *tgbotapi.BotAPI, chatID int64, botID int64, vipOnly bool, blockedOnly bool) {
	o := orm.NewOrm()

	qs := o.QueryTable(new(models.BotUser)).Filter("Bot__Id", botID)

	if vipOnly {
		qs = qs.Filter("IsVip", true)
	}
	if blockedOnly {
		qs = qs.Filter("IsBlocked", true)
	}

	var users []models.BotUser
	_, err := qs.All(&users)

	if err != nil {
		log.Printf("showUserListPro xatolik: %v", err)
		sendUserBot(bot, chatID, "❌ Ro'yxatni yuklashda texnik xatolik yuz berdi.")
		return
	}

	if len(users) == 0 {
		sendUserBot(bot, chatID, "📭 Ro'yxat hozircha bo'sh.")
		return
	}

	title := "📋 Foydalanuvchilar ro'yxati:"
	if vipOnly {
		title = "VIP foydalanuvchilar ro'yxati:"
	} else if blockedOnly {
		title = "🚫 Bloklanganlar ro'yxati:"
	}

	text := title + "\n\n"
	now := time.Now()

	for i, u := range users {
		uname := u.Username
		if uname == "" {
			uname = "noma'lum"
		} else {
			uname = strings.ReplaceAll(uname, "_", "\\_")
		}

		// Oddiy qator formati
		userLine := fmt.Sprintf("%d. ID: `%d` — @%s\n", i+1, u.TgId, uname)

		// Agar foydalanuvchi VIP bo'lsa va vaqti ko'rsatilgan bo'lsa
		if u.IsVip {
			if u.VipUntil.IsZero() {
				userLine += " (VIP: Cheksiz)\n"
			} else if u.VipUntil.Before(now) {
				userLine += " (VIP: Tugagan)\n"
			} else {
				// Qolgan vaqtni hisoblaymiz
				diff := u.VipUntil.Sub(now)
				days := int(diff.Hours() / 24)
				hours := int(diff.Hours()) % 24

				if days > 365 { // 1 yildan ko'p bo'lsa cheksiz deb ko'rsatiladi
					userLine += " (VIP: Cheksiz)"
				} else if days > 0 {
					userLine += fmt.Sprintf(" (VIP: %d kun %d soat qoldi)\n", days, hours)
				} else {
					minutes := int(diff.Minutes()) % 60
					userLine += fmt.Sprintf(" (VIP: %d soat %d daqiqa qoldi)\n", hours, minutes)
				}
			}
		}

		text += userLine + "\n"
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"

	_, sendErr := bot.Send(msg)
	if sendErr != nil {
		log.Printf("Ro'yxatni yuborishda Telegram xatoligi (ehtimol Markdown parse error): %v", sendErr)
		msg.ParseMode = ""
		bot.Send(msg)
	}
}
func startVipAddPro(bot *tgbotapi.BotAPI, chatID int64, userID int64) {
	mu.Lock()
	adminState[userID] = "waiting_vip_add"
	mu.Unlock()
	sendUserBot(bot, chatID, "VIP qilmoqchi bo'lgan foydalanuvchining Telegram ID raqamini yuboring:")
}

func startVipRemovePro(bot *tgbotapi.BotAPI, chatID int64, userID int64) {
	mu.Lock()
	adminState[userID] = "waiting_vip_remove"
	mu.Unlock()
	sendUserBot(bot, chatID, "🚫 VIP'dan chiqarmoqchi bo'lgan foydalanuvchining Telegram ID raqamini yuboring:")
}

func startBlockAddPro(bot *tgbotapi.BotAPI, chatID int64, userID int64) {
	mu.Lock()
	adminState[userID] = "waiting_block_add"
	mu.Unlock()
	sendUserBot(bot, chatID, "⛔ Bloklamoqchi bo'lgan foydalanuvchining Telegram ID raqamini yuboring:")
}

func startBlockRemovePro(bot *tgbotapi.BotAPI, chatID int64, userID int64) {
	mu.Lock()
	adminState[userID] = "waiting_block_remove"
	mu.Unlock()
	sendUserBot(bot, chatID, "✅ Blokdan chiqarmoqchi bo'lgan foydalanuvchining Telegram ID raqamini yuboring:")
}

func RouteUserManagementStatePro(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message, state string) bool {
	userID := msg.From.ID
	chatID := msg.Chat.ID
	o := orm.NewOrm()

	if state == "waiting_admin_add" {
		tgID, err := strconv.ParseInt(strings.TrimSpace(msg.Text), 10, 64)
		if err != nil {
			sendUserBot(bot, chatID, "❌ Noto'g'ri ID. Faqat raqam kiriting:\n\n /admin")
			return true
		}

		var bu models.BotUser
		err = o.QueryTable(new(models.BotUser)).
			Filter("Bot__Id", b.Id).
			Filter("TgId", tgID).
			One(&bu)

		if err != nil {
			log.Printf("🔴 ADMIN ADD: foydalanuvchi topilmadi. tgID=%d, botID=%d, err=%v", tgID, b.Id, err)
			sendUserBot(bot, chatID, "❌ Bu foydalanuvchi botda topilmadi.")
		} else {
			log.Printf("🟡 ADMIN ADD: topildi. bu.Id=%d, bu.TgId=%d, eski IsAdmin=%v", bu.Id, bu.TgId, bu.IsAdmin)
			bu.IsAdmin = true
			affected, updErr := o.Update(&bu, "IsAdmin")
			log.Printf("🟢 ADMIN ADD: Update natijasi -> affected=%d, err=%v, yangi IsAdmin=%v", affected, updErr, bu.IsAdmin)
			sendUserBot(bot, chatID, fmt.Sprintf("✅ ID %d admin qilindi!", tgID))
		}

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()
		return true

	}

	if state == "waiting_admin_remove" {
		tgID, err := strconv.ParseInt(strings.TrimSpace(msg.Text), 10, 64)
		if err != nil {
			sendUserBot(bot, chatID, "❌ Noto'g'ri ID. Faqat raqam kiriting:\n\n /admin")
			return true
		}

		if isOwner(b, tgID) {
			sendUserBot(bot, chatID, "❌ Botning asl egasini adminlikdan chiqarib bo'lmaydi.")
			mu.Lock()
			delete(adminState, userID)
			mu.Unlock()
			return true
		}

		var bu models.BotUser
		err = o.QueryTable(new(models.BotUser)).
			Filter("Bot__Id", b.Id).
			Filter("TgId", tgID).
			One(&bu)

		if err != nil {
			sendUserBot(bot, chatID, "❌ Bu foydalanuvchi botda topilmadi.")
		} else {
			bu.IsAdmin = false
			o.Update(&bu, "IsAdmin")
			sendUserBot(bot, chatID, fmt.Sprintf("✅ ID %d adminlikdan chiqarildi!", tgID))
		}

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()
		return true
	}

	if state == "waiting_vip_add" {
		tgID, err := strconv.ParseInt(strings.TrimSpace(msg.Text), 10, 64)
		if err != nil {
			sendUserBot(bot, chatID, "❌ Noto'g'ri ID. Faqat raqam kiriting yoki orqaga qayting.")
			return true
		}

		var bu models.BotUser
		err = o.QueryTable(new(models.BotUser)).
			Filter("Bot__Id", b.Id).
			Filter("TgId", tgID).
			One(&bu)

		if err != nil {
			sendUserBot(bot, chatID, "❌ Bu foydalanuvchi botda topilmadi.")
			return true
		}

		// 🎯 ID ni state ichida saqlab, muddat tanlash bosqichiga o'tkazamiz
		mu.Lock()
		adminState[userID] = fmt.Sprintf("waiting_vip_duration:%d", tgID)
		mu.Unlock()

		// 🎛 Tugmalarni yaratamiz
		keyboard := tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("7 kun"),
				tgbotapi.NewKeyboardButton("10 kun"),
				tgbotapi.NewKeyboardButton("15 kun"),
			),
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("1 oy"),
				tgbotapi.NewKeyboardButton("3 oy"),
				tgbotapi.NewKeyboardButton("Cheksiz"),
			),
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Orqaga"),
			),
		)
		keyboard.ResizeKeyboard = true

		msgResponse := tgbotapi.NewMessage(chatID, fmt.Sprintf("👤 ID: %d botdan topildi!\n\n⏳ Iltimos, VIP muddatini tanlang:", tgID))
		msgResponse.ReplyMarkup = keyboard
		bot.Send(msgResponse)

		return true
	}

	if strings.HasPrefix(state, "waiting_vip_duration:") {
		tgIDStr := strings.TrimPrefix(state, "waiting_vip_duration:")
		tgID, _ := strconv.ParseInt(tgIDStr, 10, 64)

		durationText := strings.TrimSpace(msg.Text)

		var vipUntil time.Time
		now := time.Now()

		switch durationText {
		case "7 kun":
			vipUntil = now.AddDate(0, 0, 7)
		case "10 kun":
			vipUntil = now.AddDate(0, 0, 10)
		case "15 kun":
			vipUntil = now.AddDate(0, 0, 15)
		case "1 oy":
			vipUntil = now.AddDate(0, 1, 0)
		case "3 oy":
			vipUntil = now.AddDate(0, 3, 0)
		case "Cheksiz":
			vipUntil = now.AddDate(100, 0, 0) // 100 yil
		default:
			sendUserBot(bot, chatID, "❌ Iltimos, pastdagi tugmalardan birini tanlang:")
			return true
		}

		var bu models.BotUser
		err := o.QueryTable(new(models.BotUser)).
			Filter("Bot__Id", b.Id).
			Filter("TgId", tgID).
			One(&bu)

		if err == nil {
			bu.IsVip = true
			bu.VipUntil = vipUntil // 👈 Vaqtni saqlaymiz

			// Orm orqali har ikkala ustunni update qilamiz
			o.Update(&bu, "IsVip", "VipUntil")

			successMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ ID %d uchun VIP (%s) berildi!\n\nTugash vaqti: %s", tgID, durationText, vipUntil.Format("02.01.2006 15:04")))
			successMsg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
			bot.Send(successMsg)
		}

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()
		return true

	}

	if state == "waiting_vip_remove" {
		tgID, err := strconv.ParseInt(strings.TrimSpace(msg.Text), 10, 64)
		if err != nil {
			sendUserBot(bot, chatID, "❌ Noto'g'ri ID. Faqat raqam kiriting:\n\n /admin")
			return true
		}

		var bu models.BotUser
		err = o.QueryTable(new(models.BotUser)).
			Filter("Bot__Id", b.Id).
			Filter("TgId", tgID).
			One(&bu)

		if err != nil {
			sendUserBot(bot, chatID, "❌ Bu foydalanuvchi botda topilmadi.")
		} else {
			bu.IsVip = false
			o.Update(&bu, "IsVip")
			sendUserBot(bot, chatID, fmt.Sprintf("✅ ID %d VIP'dan chiqarildi!", tgID))
		}

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()
		return true

	}

	if state == "waiting_block_add" {
		tgID, err := strconv.ParseInt(strings.TrimSpace(msg.Text), 10, 64)
		if err != nil {
			sendUserBot(bot, chatID, "❌ Noto'g'ri ID. Faqat raqam kiriting:\n\n /admin")
			return true
		}

		var bu models.BotUser
		err = o.QueryTable(new(models.BotUser)).
			Filter("Bot__Id", b.Id).
			Filter("TgId", tgID).
			One(&bu)

		if err != nil {
			sendUserBot(bot, chatID, "❌ Bu foydalanuvchi botda topilmadi.")
		} else {
			bu.IsBlocked = true
			o.Update(&bu, "IsBlocked")
			sendUserBot(bot, chatID, fmt.Sprintf("✅ ID %d bloklandi!", tgID))
		}

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()
		return true
	}

	if state == "waiting_block_remove" {
		tgID, err := strconv.ParseInt(strings.TrimSpace(msg.Text), 10, 64)
		if err != nil {
			sendUserBot(bot, chatID, "❌ Noto'g'ri ID. Faqat raqam kiriting:\n\n /admin")
			return true
		}

		var bu models.BotUser
		err = o.QueryTable(new(models.BotUser)).
			Filter("Bot__Id", b.Id).
			Filter("TgId", tgID).
			One(&bu)

		if err != nil {
			sendUserBot(bot, chatID, "❌ Bu foydalanuvchi botda topilmadi.")
		} else {
			bu.IsBlocked = false
			o.Update(&bu, "IsBlocked")
			sendUserBot(bot, chatID, fmt.Sprintf("✅ ID %d blokdan chiqarildi!", tgID))
		}

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()

	}

	return false
}

func StartVipCleanerTask() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			cleanExpiredVips()
		}
	}()
}

func cleanExpiredVips() {
	o := orm.NewOrm()
	now := time.Now()

	var expiredUsers []models.BotUser
	_, err := o.QueryTable(new(models.BotUser)).
		Filter("IsVip", true).
		Filter("VipUntil__lt", now).
		All(&expiredUsers)

	if err != nil || len(expiredUsers) == 0 {
		return
	}

	for _, u := range expiredUsers {
		u.IsVip = false
		if _, err := o.Update(&u, "IsVip"); err != nil {
			log.Printf("VIP tozalashda xatolik (TgID: %d): %v", u.TgId, err)
			continue
		}
		log.Printf("VIP muddati tugadi va o'chirildi: TgID %d", u.TgId)
	}
}

func IsUserVip(user *models.BotUser) bool {
	if !user.IsVip {
		return false
	}

	// Agar IsVip true bo'lsa-yu, lekin vaqti o'tib ketgan bo'lsa
	if !user.VipUntil.IsZero() && user.VipUntil.Before(time.Now()) {
		user.IsVip = false
		orm.NewOrm().Update(user, "IsVip")
		return false
	}

	return true
}

func showStatisticsPro(bot *tgbotapi.BotAPI, b *models.CreatedBot, chatID int64) {
	o := orm.NewOrm()
	now := time.Now()

	totalAnime, _ := o.QueryTable(new(models.Anime)).
		Filter("Bot__Id", b.Id).
		Count()

	totalUsers, _ := o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", b.Id).
		Count()

	vipCount, _ := o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", b.Id).
		Filter("IsVip", true).
		Count()

	blockedCount, _ := o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", b.Id).
		Filter("IsBlocked", true).
		Count()

	newToday, _ := o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", b.Id).
		Filter("JoinedAt__gte", now.AddDate(0, 0, -1)).
		Count()

	new7Days, _ := o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", b.Id).
		Filter("JoinedAt__gte", now.AddDate(0, 0, -7)).
		Count()

	new1Month, _ := o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", b.Id).
		Filter("JoinedAt__gte", now.AddDate(0, -1, 0)).
		Count()

	new2Months, _ := o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", b.Id).
		Filter("JoinedAt__gte", now.AddDate(0, -2, 0)).
		Count()

	new3Months, _ := o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", b.Id).
		Filter("JoinedAt__gte", now.AddDate(0, -3, 0)).
		Count()

	activeToday, _ := o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", b.Id).
		Filter("UpdatedAt__gte", now.AddDate(0, 0, -1)).
		Count()

	active7Days, _ := o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", b.Id).
		Filter("UpdatedAt__gte", now.AddDate(0, 0, -7)).
		Count()

	active1Month, _ := o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", b.Id).
		Filter("UpdatedAt__gte", now.AddDate(0, -1, 0)).
		Count()

	active2Months, _ := o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", b.Id).
		Filter("UpdatedAt__gte", now.AddDate(0, -2, 0)).
		Count()

	active3Months, _ := o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", b.Id).
		Filter("UpdatedAt__gte", now.AddDate(0, -3, 0)).
		Count()

	text := fmt.Sprintf(
		" Bot Statistikasi\n\n"+
			"• Jami anime: `%d`\n"+
			"• Jami user: `%d`\n"+
			"• VIP user: `%d`\n"+
			"• Ban user: `%d`\n\n"+
			"• Qo'shilgan foydalanuvchilar\n"+
			"├ 1 kun: `%d`\n"+
			"├ 7 kun: `%d`\n"+
			"├ 30 kun: `%d`\n"+
			"├ 60 kun: `%d`\n"+
			"└ 90 kun: `%d`\n\n"+
			"• Faol foydalanuvchilar\n"+
			"├ 1 kun: `%d`\n"+
			"├ 7 kun: `%d`\n"+
			"├ 30 kun: `%d`\n"+
			"├ 60 kun: `%d`\n"+
			"└ 90 kun: `%d`\n",
		totalAnime,
		totalUsers,
		vipCount,
		blockedCount,
		newToday, new7Days, new1Month, new2Months, new3Months,
		activeToday, active7Days, active1Month, active2Months, active3Months,
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}

func RunBroadcastPro(bot *tgbotapi.BotAPI, b *models.CreatedBot, srcMsg *tgbotapi.Message, target string) {
	chatID := srcMsg.Chat.ID
	o := orm.NewOrm()

	qs := o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", b.Id).
		Filter("IsBlocked", false)

	switch target {
	case "vip":
		qs = qs.Filter("IsVip", true)
	case "regular":
		qs = qs.Filter("IsVip", false)
	}

	var users []models.BotUser
	_, err := qs.All(&users)
	if err != nil || len(users) == 0 {
		bot.Send(tgbotapi.NewMessage(chatID, "📭 Yuborish uchun foydalanuvchi topilmadi."))
		return
	}

	statusMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("⏳ Yuborilmoqda... (jami: %d)", len(users)))
	bot.Send(statusMsg)

	success := 0
	failed := 0

	for _, u := range users {
		copyCfg := tgbotapi.CopyMessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID: u.TgId,
			},
			FromChatID: srcMsg.Chat.ID,
			MessageID:  srcMsg.MessageID,
		}

		_, err := bot.Send(copyCfg)
		if err != nil {
			failed++
		} else {
			success++
		}

		time.Sleep(50 * time.Millisecond)
	}

	report := fmt.Sprintf(
		"✅ Reklama yuborish yakunlandi\n\n"+
			"👥 Jami: `%d`\n"+
			"✅ Yetib bordi: `%d`\n"+
			"❌ Yetmadi: `%d`",
		len(users), success, failed,
	)

	resultMsg := tgbotapi.NewMessage(chatID, report)
	resultMsg.ParseMode = "Markdown"
	bot.Send(resultMsg)

}

func startAdminAddPro(bot *tgbotapi.BotAPI, chatID int64, userID int64) {
	mu.Lock()
	adminState[userID] = "waiting_admin_add"
	mu.Unlock()
	sendUserBot(bot, chatID, "➕ Admin qilmoqchi bo'lgan foydalanuvchining Telegram ID raqamini yuboring:")
}

func showAdminsListPro(bot *tgbotapi.BotAPI, b *models.CreatedBot, chatID int64) {
	o := orm.NewOrm()

	var owner models.CreatedBot
	err := o.QueryTable(new(models.CreatedBot)).
		Filter("Id", b.Id).
		RelatedSel("Owner").
		One(&owner)

	var rows [][]tgbotapi.InlineKeyboardButton

	if err == nil && owner.Owner != nil {
		ownerBtn := tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("👑 Owner — ID: %d", owner.Owner.TgId),
			fmt.Sprintf("admin_info:%d:%d", b.Id, owner.Owner.TgId),
		)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{ownerBtn})
	}

	var admins []models.BotUser
	_, err = o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", b.Id).
		Filter("IsAdmin", true).
		All(&admins)

	if err == nil {
		for _, a := range admins {
			uname := a.Username
			if uname == "" {
				uname = "noma'lum"
			}
			btn := tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("🛡 @%s — ID: %d", uname, a.TgId),
				fmt.Sprintf("admin_info:%d:%d", b.Id, a.TgId),
			)
			rows = append(rows, []tgbotapi.InlineKeyboardButton{btn})
		}
	}

	text := "👤 *Adminlar ro'yxati:*\n\nKo'proq ma'lumot uchun admin ustiga bosing:"
	if len(rows) == 0 {
		text = "📭 Hozircha adminlar mavjud emas."
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	if len(rows) > 0 {
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	}

	_, sendErr := bot.Send(msg)
	if sendErr != nil {
		log.Printf("🔴 ADMIN LIST: xabar yuborishda xatolik: %v", sendErr)
	}
}

func showAdminInfoPro(bot *tgbotapi.BotAPI, chatID int64, botID int64, tgID int64) {
	o := orm.NewOrm()

	var createdBot models.CreatedBot
	isOwnerHere := false
	if err := o.QueryTable(new(models.CreatedBot)).
		Filter("Id", botID).
		RelatedSel("Owner").
		One(&createdBot); err == nil && createdBot.Owner != nil && createdBot.Owner.TgId == tgID {
		isOwnerHere = true
	}

	var bu models.BotUser
	err := o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", botID).
		Filter("TgId", tgID).
		One(&bu)

	linkLabel := fmt.Sprintf("%d", tgID)
	if err == nil && bu.Username != "" {
		linkLabel = bu.Username
	}

	text := fmt.Sprintf("👤 [%s](tg://user?id=%d)", linkLabel, tgID)

	if isOwnerHere {
		text += "\n\n👑 Bu botning asosiy egasi — o'chirib bo'lmaydi."
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "Markdown"
		bot.Send(msg)
		return
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🗑 O'chirish", fmt.Sprintf("admin_remove:%d:%d", botID, tgID)),
		),
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

func performAdminRemovePro(bot *tgbotapi.BotAPI, chatID int64, botID int64, tgID int64) {
	o := orm.NewOrm()

	var createdBot models.CreatedBot
	if err := o.QueryTable(new(models.CreatedBot)).
		Filter("Id", botID).
		RelatedSel("Owner").
		One(&createdBot); err == nil && createdBot.Owner != nil && createdBot.Owner.TgId == tgID {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Botning asl egasini adminlikdan chiqarib bo'lmaydi."))
		return
	}

	var bu models.BotUser
	err := o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", botID).
		Filter("TgId", tgID).
		One(&bu)

	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Bu foydalanuvchi botda topilmadi."))
		return
	}

	bu.IsAdmin = false
	o.Update(&bu, "IsAdmin")

	bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ ID %d adminlikdan chiqarildi!", tgID)))

	if createdBot.Id != 0 {
		showAdminsListPro(bot, &createdBot, chatID)
	}
}

func HandleAnimeCallbackPro(bot *tgbotapi.BotAPI, cb *tgbotapi.CallbackQuery) {
	if cb.Message == nil {
		return
	}
	data := cb.Data
	chatID := cb.Message.Chat.ID
	userID := cb.From.ID

	bot.Request(tgbotapi.NewCallback(cb.ID, ""))

	log.Printf("Anime callback keldi: %s (Chat ID: %d)", data, chatID)

	switch {
	case strings.HasPrefix(data, "vip_prices_"):
		log.Printf("🟣 [VIP_PRICES] Callback keldi! Data: %s | ChatID: %d | UserID: %d", data, chatID, userID)

		botID, err := strconv.ParseInt(strings.TrimPrefix(data, "vip_prices_"), 10, 64)
		if err != nil {
			log.Printf("🔴 [VIP_PRICES] Bot ID parse xatosi: %v (raw data: %s)", err, data)
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Bot ID aniqlanmadi"))
			return
		}

		log.Printf("🟢 [VIP_PRICES] Bot ID aniqlandi: %d — HandleVipPricesCallback chaqirilyapti...", botID)

		HandleVipPricesCallback(bot, cb, botID)

		log.Printf("✅ [VIP_PRICES] HandleVipPricesCallback tugadi (BotID: %d)", botID)
		return
	case strings.HasPrefix(data, "toggle_vip_only:"):
		animeID, err := strconv.ParseInt(strings.TrimPrefix(data, "toggle_vip_only:"), 10, 64)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ ID aniqlanmadi"))
			return
		}

		o := orm.NewOrm()
		var anime models.Anime
		err = o.QueryTable(new(models.Anime)).Filter("Id", animeID).One(&anime)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Anime topilmadi"))
			return
		}

		anime.IsVipOnly = !anime.IsVipOnly
		_, _ = o.Update(&anime, "IsVipOnly")

		status := "🔓 Endi **hamma** ko‘ra oladi"
		if anime.IsVipOnly {
			status = "🔒 Endi **faqat VIP** lar ko‘ra oladi"
		}

		msg := tgbotapi.NewMessage(chatID, status)
		msg.ParseMode = "Markdown"
		bot.Send(msg)
		return

	case strings.HasPrefix(data, "top:"):
		parts := strings.Split(strings.TrimPrefix(data, "top:"), ":")
		if len(parts) != 2 {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Noto‘g‘ri format."))
			return
		}

		botID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Bot ID xato."))
			return
		}
		code := parts[1]

		o := orm.NewOrm()
		var createdBot models.CreatedBot
		if err := o.QueryTable(new(models.CreatedBot)).Filter("Id", botID).One(&createdBot); err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Bot topilmadi."))
			return
		}

		fakeMsg := &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: chatID},
			From: cb.From,
			Text: code,
		}
		handleAnimeByCodePro(bot, &createdBot, fakeMsg, code)
		return

	case strings.HasPrefix(data, "anime_select_"):
		idStr := strings.TrimPrefix(data, "anime_select_")
		animeID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Anime ID aniqlanmadi."))
			return
		}

		o := orm.NewOrm()
		var anime models.Anime
		err = o.QueryTable(new(models.Anime)).
			Filter("Id", animeID).
			RelatedSel("Bot").
			One(&anime)
		if err != nil || anime.Bot == nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Anime topilmadi."))
			return
		}

		botUser := GetOrCreateBotUser(anime.Bot, userID, cb.From)
		protectContent := !(botUser.IsVip || isAdmin(anime.Bot, userID))

		if anime.PartsCount == 1 {
			sendSinglePartAnimePro(bot, o, chatID, &anime, anime.Bot.Note, protectContent)
		} else {
			caption := fmt.Sprintf("%s\n\nJami qismlar - %d", anime.Name, anime.PartsCount)
			if anime.Bot.Note != "" {
				caption += fmt.Sprintf("\n\n%s", anime.Bot.Note)
			}
			keyboard := buildAnimePartsKeyboardPro(anime.Id, anime.PartsCount, 1)

			if anime.PhotoID != "" {
				_ = sendProtectedPhotoPro(bot, chatID, anime.PhotoID, caption, &keyboard, protectContent)
			} else {
				_ = sendProtectedTextPro(bot, chatID, caption, &keyboard, protectContent)
			}
		}
		return

	case strings.HasPrefix(data, "admin_info:"):
		parts := strings.Split(strings.TrimPrefix(data, "admin_info:"), ":")
		if len(parts) != 2 {
			return
		}
		botID, _ := strconv.ParseInt(parts[0], 10, 64)
		tgID, _ := strconv.ParseInt(parts[1], 10, 64)
		showAdminInfoPro(bot, chatID, botID, tgID)
		return

	case strings.HasPrefix(data, "admin_remove:"):
		parts := strings.Split(strings.TrimPrefix(data, "admin_remove:"), ":")
		if len(parts) != 2 {
			return
		}
		botID, _ := strconv.ParseInt(parts[0], 10, 64)
		tgID, _ := strconv.ParseInt(parts[1], 10, 64)
		performAdminRemovePro(bot, chatID, botID, tgID)
		return

	case strings.HasPrefix(data, "anime_page:"):
		handleAnimePagePro(bot, cb)

	case strings.HasPrefix(data, "anime_part:"):
		handleAnimePartPro(bot, cb)

	case strings.HasPrefix(data, "edit_code:"):
		animeID, _ := strconv.ParseInt(strings.TrimPrefix(data, "edit_code:"), 10, 64)
		mu.Lock()
		adminState[userID] = fmt.Sprintf("waiting_new_code:%d", animeID)
		mu.Unlock()
		bot.Send(tgbotapi.NewMessage(chatID, "🔑 Yangi kodni kiriting:"))
		return

	case strings.HasPrefix(data, "edit_name:"):
		animeID, _ := strconv.ParseInt(strings.TrimPrefix(data, "edit_name:"), 10, 64)
		mu.Lock()
		adminState[userID] = fmt.Sprintf("waiting_new_name:%d", animeID)
		mu.Unlock()
		bot.Send(tgbotapi.NewMessage(chatID, "📝 Yangi nomni kiriting:"))
		return

	case strings.HasPrefix(data, "edit_addpart:"):
		animeID, _ := strconv.ParseInt(strings.TrimPrefix(data, "edit_addpart:"), 10, 64)
		mu.Lock()
		adminState[userID] = fmt.Sprintf("waiting_new_part_file:%d", animeID)
		mu.Unlock()
		bot.Send(tgbotapi.NewMessage(chatID, "➕ Yangi qism uchun video, rasm yoki hujjat yuboring:"))
		return

	case strings.HasPrefix(data, "edit_delpart:"):
		animeID, _ := strconv.ParseInt(strings.TrimPrefix(data, "edit_delpart:"), 10, 64)
		mu.Lock()
		adminState[userID] = fmt.Sprintf("waiting_del_part_num:%d", animeID)
		mu.Unlock()
		bot.Send(tgbotapi.NewMessage(chatID, "➖ O'chirmoqchi bo'lgan qism raqamini kiriting:"))
		return

	case strings.HasPrefix(data, "edit_photo:"):
		animeID, _ := strconv.ParseInt(strings.TrimPrefix(data, "edit_photo:"), 10, 64)
		mu.Lock()
		adminState[userID] = fmt.Sprintf("waiting_new_photo:%d", animeID)
		mu.Unlock()
		bot.Send(tgbotapi.NewMessage(chatID, "🖼 Yangi rasmni yuboring:"))
		return

	case strings.HasPrefix(data, "delete_anime:"):
		animeID, err := strconv.ParseInt(strings.TrimPrefix(data, "delete_anime:"), 10, 64)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ ID aniqlanmadi"))
			return
		}

		o := orm.NewOrm()

		var anime models.Anime
		if err := o.QueryTable(new(models.Anime)).Filter("Id", animeID).One(&anime); err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Anime topilmadi yoki allaqachon o'chirilgan."))
			return
		}

		animeName := anime.Name

		_, err = o.QueryTable(new(models.AnimePart)).Filter("Anime__Id", animeID).Delete()
		if err != nil {
			log.Printf("AnimePart o'chirishda xatolik: %v", err)
		}

		_, err = o.Delete(&anime)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Animeni o'chirishda xatolik yuz berdi."))
			return
		}

		log.Printf("🗑️ Anime o'chirildi! ID: %d, Nomi: %s", animeID, animeName)

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ \"%s\" anime butunlay o'chirildi!", animeName))
		bot.Send(msg)
		return

	case strings.HasPrefix(data, "del_chan_"):
		chanRecID, err := strconv.ParseInt(strings.TrimPrefix(data, "del_chan_"), 10, 64)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Kanal ID aniqlanmadi"))
			return
		}

		o := orm.NewOrm()

		var channel models.BotChannel
		if err := o.QueryTable(new(models.BotChannel)).Filter("Id", chanRecID).One(&channel); err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Kanal topilmadi yoki allaqachon o'chirilgan."))
			return
		}

		channel.IsActive = false
		if _, err := o.Update(&channel, "IsActive"); err != nil {
			log.Printf("Kanalni o'chirishda xatolik: %v", err)
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Kanalni o'chirishda xatolik yuz berdi."))
			return
		}

		log.Printf("🗑️ Kanal o'chirildi (IsActive=false): ID: %d, ChannelID: %d", channel.Id, channel.ChannelID)

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Kanal (ID: %d) majburiy obuna ro'yxatidan olib tashlandi.", channel.ChannelID))
		bot.Send(msg)
		return

	case strings.HasPrefix(data, "edit_bot_note:"):
		botID, _ := strconv.ParseInt(strings.TrimPrefix(data, "edit_bot_note:"), 10, 64)
		mu.Lock()
		adminState[userID] = fmt.Sprintf("waiting_bot_note:%d", botID)
		mu.Unlock()
		bot.Send(tgbotapi.NewMessage(chatID, "✍️ Yangi matnni kiriting:"))
		return

	case strings.HasPrefix(data, "delete_bot_note:"):
		botID, err := strconv.ParseInt(strings.TrimPrefix(data, "delete_bot_note:"), 10, 64)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Bot ID aniqlanmadi"))
			return
		}

		o := orm.NewOrm()
		_, updErr := o.QueryTable(new(models.CreatedBot)).
			Filter("Id", botID).
			Update(orm.Params{"Note": ""})

		if updErr != nil {
			log.Printf("Bot Note o'chirishda xatolik: %v", updErr)
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Matnni o'chirishda xatolik yuz berdi."))
			return
		}

		bot.Send(tgbotapi.NewMessage(chatID, "✅ Matn o'chirildi!"))
		return

	case strings.HasPrefix(data, "delete_promo_channel:"):
		promoID, err := strconv.ParseInt(strings.TrimPrefix(data, "delete_promo_channel:"), 10, 64)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Kanal ID aniqlanmadi"))
			return
		}

		o := orm.NewOrm()

		var channel models.PromoChannel
		if err := o.QueryTable(new(models.PromoChannel)).Filter("Id", promoID).One(&channel); err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Kanal topilmadi yoki allaqachon o'chirilgan."))
			return
		}

		channel.IsActive = false
		if _, err := o.Update(&channel, "IsActive"); err != nil {
			log.Printf("PromoChannel o'chirishda xatolik: %v", err)
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Kanalni o'chirishda xatolik yuz berdi."))
			return
		}

		log.Printf("🗑️ Promo kanal o'chirildi (IsActive=false): ID: %d, ChannelID: %d", channel.Id, channel.ChannelID)

		title := channel.Title
		if title == "" {
			title = fmt.Sprintf("%d", channel.ChannelID)
		}

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Kanal (%s) reklama ro'yxatidan olib tashlandi.", title))
		bot.Send(msg)
		return

	default:
		log.Printf("Noma'lum anime callback data: %s", data)
		msg := tgbotapi.NewMessage(chatID, "Noma'lum anime buyruq: "+data)
		bot.Send(msg)

	}
}

func handleAnimePartPro(bot *tgbotapi.BotAPI, cb *tgbotapi.CallbackQuery) {
	fmt.Println("===== handleAnimePartPro ISHGA TUSHDI =====")

	// Callback spinnerini o'chirish
	bot.Request(tgbotapi.NewCallback(cb.ID, ""))

	parts := strings.Split(cb.Data, ":")
	if len(parts) < 3 {
		fmt.Println("bad callback data:", cb.Data)
		return
	}

	animeID, _ := strconv.ParseInt(parts[1], 10, 64)
	partOrder, _ := strconv.Atoi(parts[2])
	chatID := cb.Message.Chat.ID
	userID := cb.From.ID

	o := orm.NewOrm()

	var animePart models.AnimePart
	err := o.QueryTable(new(models.AnimePart)).
		Filter("Anime__Id", animeID).
		Filter("PartOrder", partOrder).
		RelatedSel("Anime").
		One(&animePart)

	if err != nil {
		fmt.Println("PART ERROR:", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Qism topilmadi."))
		return
	}

	name := "Anime"
	if animePart.Anime != nil {
		name = animePart.Anime.Name
	}

	// 1. Defolt holatda saqlash va uzatish TAQIQLANADI
	protect := true
	var botNote string

	// 2. Anime va Bot ma'lumotlarini olish
	var anime models.Anime
	aerr := o.QueryTable(new(models.Anime)).
		Filter("Id", animeID).
		RelatedSel("Bot").
		One(&anime)

	if aerr == nil && anime.Bot != nil {
		// FAQAT Admin uchun cheklov O'CHIRILADI
		if isAdmin(anime.Bot, userID) {
			protect = false
		}
		botNote = anime.Bot.Note
	} else {
		log.Printf("🔴 BOT VA ANIME BOG'LANISHIDA XATOLIK: %v", aerr)
	}

	// Logging (Terminalda tekshirib olishingiz uchun)
	log.Printf("DEBUG: UserID: %d | IsProtect: %t | Kind: %s", userID, protect, animePart.Kind)

	// Caption tayyorlaymiz
	caption := fmt.Sprintf("%s - [%d-qism]", name, partOrder)
	if botNote != "" {
		caption += fmt.Sprintf("\n\n%s", botNote)
	}

	var sendErr error
	switch animePart.Kind {
	case "video":
		sendErr = sendProtectedVideoPro(bot, chatID, animePart.FileID, caption, protect)
	case "document":
		sendErr = sendProtectedDocumentPro(bot, chatID, animePart.FileID, caption, protect)
	case "photo":
		sendErr = sendProtectedPhotoPro(bot, chatID, animePart.FileID, caption, nil, protect)
	default:
		fmt.Println("UNKNOWN KIND:", animePart.Kind)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Noma'lum fayl turi: "+animePart.Kind))
		return
	}

	if sendErr != nil {
		fmt.Println(">>> SEND PART ERROR:", sendErr)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Faylni yuborishda xatolik yuz berdi."))
	}
}

func getPendingAd(userID int64) (AdData, bool) {
	pendingAdsMu.RLock()
	defer pendingAdsMu.RUnlock()
	ad, ok := pendingAds[userID]
	return ad, ok
}

func deletePendingAd(userID int64) {
	pendingAdsMu.Lock()
	defer pendingAdsMu.Unlock()
	delete(pendingAds, userID)
}

func setPendingAd(userID int64, ad AdData) {
	pendingAdsMu.Lock()
	defer pendingAdsMu.Unlock()
	pendingAds[userID] = ad
}

func HandlePromoChannelAdd(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
	userID := msg.From.ID
	chatID := msg.Chat.ID

	input := strings.TrimSpace(msg.Text)

	if input == "" {
		sendUserBot(
			bot,
			chatID,
			"Kanal ID sini yuboring.\n\n /admin",
		)
		return
	}

	channelID, err := strconv.ParseInt(input, 10, 64)
	if err != nil {
		sendUserBot(
			bot,
			chatID,
			"Kanal ID noto‘g‘ri.\n\n /admin",
		)
		return
	}

	chat, err := bot.GetChat(tgbotapi.ChatInfoConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: channelID,
		},
	})

	if err != nil {
		sendUserBot(
			bot,
			chatID,
			"❌ Kanalni topib bo‘lmadi.\n\n"+
				"Tekshiring:\n"+
				"1️⃣ Bot kanalga admin qilib qo‘shilganmi?\n"+
				"2️⃣ Kanal ID to‘g‘rimi?\n"+
				"3️⃣ Kanal ID `-100...` bilan boshlanganmi?\n\n /admin",
		)
		return
	}

	if chat.Type != "channel" {
		sendUserBot(
			bot,
			chatID,
			"❌ Bu ID kanalga tegishli emas.\n /admin",
		)
		return
	}

	o := orm.NewOrm()

	existing := &models.PromoChannel{ // models.PromoChannel deb o'zgartirildi
		Bot:       b,
		ChannelID: channelID,
	}

	err = o.Read(existing, "Bot", "ChannelID")

	if err == nil {
		sendUserBot(
			bot,
			chatID,
			"⚠️ Bu kanal allaqachon reklama kanallari ro‘yxatiga qo‘shilgan.",
		)
		return
	}

	promoChannel := &models.PromoChannel{ // models.PromoChannel deb o'zgartirildi
		Bot:       b,
		ChannelID: chat.ID,
		UserName:  chat.UserName,
		Title:     chat.Title,
		IsActive:  true,
	}

	_, err = o.Insert(promoChannel)

	if err != nil {
		log.Printf("❌ PromoChannel saqlashda xatolik: %v", err)
		sendUserBot(
			bot,
			chatID,
			"❌ Kanalni bazaga saqlashda xatolik yuz berdi.",
		)
		return
	}

	mu.Lock()
	delete(adminState, userID)
	mu.Unlock()

	username := ""
	if chat.UserName != "" {
		username = "\n🔗 @" + chat.UserName
	}

	sendUserBot(
		bot,
		chatID,
		fmt.Sprintf(
			"✅ Kanal muvaffaqiyatli qo‘shildi!\n\n"+
				"📢 Nomi: %s\n"+
				"🆔 ID: `%d`%s\n\n"+
				"📣 Endi bu kanalga reklama yuborishingiz mumkin.",
			chat.Title,
			chat.ID,
			username,
		),
	)
}

func sendAdToChannel(bot *tgbotapi.BotAPI, channelID int64, ad AdData) error {

	botUsername := bot.Self.UserName
	if botUsername == "" {
		botUsername = "animlar_uzbekcha_bot"
	}

	link := fmt.Sprintf(
		"https://t.me/%s?start=%s",
		botUsername,
		ad.AnimeCode,
	)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL(
				ad.ButtonText,
				link,
			),
		),
	)

	switch ad.MediaType {

	case "photo":
		msg := tgbotapi.NewPhoto(channelID, tgbotapi.FileID(ad.FileID))
		msg.Caption = ad.Caption
		msg.ReplyMarkup = keyboard

		_, err := bot.Send(msg)
		return err

	case "video":
		msg := tgbotapi.NewVideo(channelID, tgbotapi.FileID(ad.FileID))
		msg.Caption = ad.Caption
		msg.ReplyMarkup = keyboard

		_, err := bot.Send(msg)
		return err

	default:
		return fmt.Errorf("noma'lum media turi: %s", ad.MediaType)
	}
}

func showPromoChannelsForAd(bot *tgbotapi.BotAPI, b *models.CreatedBot, chatID int64) {
	o := orm.NewOrm()

	var channels []*models.PromoChannel // models.PromoChannel deb o'zgartirildi

	_, err := o.QueryTable(new(models.PromoChannel)). // models.PromoChannel deb o'zgartirildi
								Filter("Bot__Id", b.Id).
								Filter("IsActive", true).
								All(&channels)

	if err != nil {
		sendUserBot(bot, chatID, "❌ Kanallarni olishda xatolik.")
		return
	}

	if len(channels) == 0 {
		sendUserBot(bot, chatID, "❌ Hali reklama yuborish uchun kanal qo‘shilmagan.")
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton

	for _, ch := range channels {
		title := ch.Title

		if title == "" {
			title = fmt.Sprintf("📢 %d", ch.ChannelID)
		}

		callbackData := fmt.Sprintf("send_ad_channel:%d", ch.Id)

		rows = append(
			rows,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📢 "+title, callbackData),
			),
		)
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	msg := tgbotapi.NewMessage(chatID, "📢 Reklamani yuborish uchun kanalni tanlang:")
	msg.ReplyMarkup = keyboard

	bot.Send(msg)
}

func handleSendAdChannelCallback(bot *tgbotapi.BotAPI, b *models.CreatedBot, callback *tgbotapi.CallbackQuery) {
	if callback == nil {
		return
	}

	data := callback.Data

	if !strings.HasPrefix(data, "send_ad_channel:") {
		return
	}

	userID := callback.From.ID

	channelDBIDStr := strings.TrimPrefix(data, "send_ad_channel:")
	channelDBID, err := strconv.ParseInt(channelDBIDStr, 10, 64)

	if err != nil {
		bot.Request(tgbotapi.NewCallback(callback.ID, "❌ Kanal ID noto‘g‘ri!"))
		return
	}

	o := orm.NewOrm()

	channel := &models.PromoChannel{Id: channelDBID} // models.PromoChannel deb o'zgartirildi
	err = o.Read(channel)

	if err != nil {
		bot.Request(tgbotapi.NewCallback(callback.ID, "❌ Kanal topilmadi!"))
		return
	}

	if channel.Bot == nil {
		// LoadRelated (count int64, err error) 2 ta qiymat qaytaradi
		_, err = o.LoadRelated(channel, "Bot")
		if err != nil {
			bot.Request(tgbotapi.NewCallback(callback.ID, "❌ Kanal bot ma'lumotlari topilmadi!"))
			return
		}
	}

	if channel.Bot == nil || channel.Bot.Id != b.Id {
		bot.Request(tgbotapi.NewCallback(callback.ID, "❌ Bu kanal ushbu botga tegishli emas!"))
		return
	}

	if !channel.IsActive {
		bot.Request(tgbotapi.NewCallback(callback.ID, "❌ Bu kanal hozir faol emas!"))
		return
	}

	ad, ok := getPendingAd(userID)

	if !ok {
		bot.Request(tgbotapi.NewCallback(callback.ID, "❌ Reklama ma'lumotlari topilmadi yoki muddati tugagan!"))

		if callback.Message != nil {
			sendUserBot(
				bot,
				callback.Message.Chat.ID,
				"❌ Reklama ma'lumotlari topilmadi.\n\n📣 Reklamani qaytadan tayyorlang.",
			)
		}
		return
	}

	err = sendAdToChannel(bot, channel.ChannelID, ad)

	if err != nil {
		log.Printf(
			"❌ Reklama kanalga yuborilmadi | BotID=%d | ChannelID=%d | Error=%v",
			b.Id,
			channel.ChannelID,
			err,
		)

		bot.Request(tgbotapi.NewCallback(callback.ID, "❌ Reklama yuborilmadi!"))

		if callback.Message != nil {
			sendUserBot(
				bot,
				callback.Message.Chat.ID,
				"❌ Reklamani kanalga yuborishda xatolik yuz berdi.\n\n"+
					"📢 Kanal: "+channel.Title+"\n"+
					"🆔 ID: "+strconv.FormatInt(channel.ChannelID, 10)+"\n\n"+
					"Xatolik: "+err.Error(),
			)
		}
		return
	}

	bot.Request(tgbotapi.NewCallback(callback.ID, "✅ Reklama yuborildi!"))

	if callback.Message != nil {
		username := ""
		if channel.UserName != "" {
			username = "\n🔗 @" + channel.UserName
		}

		sendUserBot(
			bot,
			callback.Message.Chat.ID,
			fmt.Sprintf(
				"✅ Reklama muvaffaqiyatli yuborildi!\n\n"+
					"📢 Kanal: %s\n"+
					"🆔 Kanal ID: `%d`%s\n\n"+
					"🎬 Anime kodi: `%s`",
				channel.Title,
				channel.ChannelID,
				username,
				ad.AnimeCode,
			),
		)
	}

	deletePendingAd(userID)
}

func setAdminState(userID int64, state string) {
	adminStateMu.Lock()
	defer adminStateMu.Unlock()
	adminState[userID] = state
}

func getAdminState(userID int64) (string, bool) {
	adminStateMu.RLock()
	defer adminStateMu.RUnlock()
	st, ok := adminState[userID]
	return st, ok
}

func clearAdminState(userID int64) {
	adminStateMu.Lock()
	defer adminStateMu.Unlock()
	delete(adminState, userID)
}

func StartAdCreation(bot *tgbotapi.BotAPI, chatID int64, userID int64) {
	setPendingAd(userID, AdData{})
	setAdminState(userID, StateWaitingForAdMedia)

	sendUserBot(
		bot,
		chatID,
		"📢 Reklama yaratish bosqichi:\n\n"+
			"📸 Iltimos, reklama uchun Rasm yoki Video yuboring.",
	)
}

func HandleAdCreationSteps(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) bool {
	userID := msg.From.ID
	chatID := msg.Chat.ID

	state, exists := getAdminState(userID)
	if !exists {
		return false // Admin reklama yaratish rejimida emas
	}

	ad, _ := getPendingAd(userID)

	switch state {

	// --------------------------------------------------
	// A) MEDIA (Rasm yoki Video) QABUL QILISH
	// --------------------------------------------------
	case StateWaitingForAdMedia:
		if msg.Photo != nil && len(msg.Photo) > 0 {
			// Eng yuqori sifatli rasmni olamiz
			photos := msg.Photo
			ad.MediaType = "photo"
			ad.FileID = photos[len(photos)-1].FileID
		} else if msg.Video != nil {
			ad.MediaType = "video"
			ad.FileID = msg.Video.FileID
		} else {
			sendUserBot(
				bot,
				chatID,
				"❌ Noto‘g‘ri format!\n\nIltimos, faqat Rasm    yoki   Video yuboring.\n\n /admin",
			)
			return true
		}

		setPendingAd(userID, ad)
		setAdminState(userID, StateWaitingForAdCaption)

		sendUserBot(
			bot,
			chatID,
			"✍️ Endi reklama ostiga yoziladigan { Matn } (Caption) yuboring:",
		)
		return true

	// --------------------------------------------------
	// B) MATN (Caption) QABUL QILISH
	// --------------------------------------------------
	case StateWaitingForAdCaption:
		text := strings.TrimSpace(msg.Text)
		if text == "-" {
			ad.Caption = ""
		} else {
			ad.Caption = msg.Text
		}

		setPendingAd(userID, ad)
		setAdminState(userID, StateWaitingForAdButtonText)

		// 1. Pastda chiqadigan tayyor tugmalarni yaratamiz
		keyboard := tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Tomosha qilish"),
				tgbotapi.NewKeyboardButton("Yuklab olish"),
			),
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Orqaga"),
			),
		)
		keyboard.ResizeKeyboard = true

		// 2. Xabar obyektini tuzamiz
		responseMsg := tgbotapi.NewMessage(chatID,
			"Endi reklama ostidagi tugma matnini yuboring yoki pastdagilardan birini tanlang:",
		)
		responseMsg.ParseMode = "Markdown"
		responseMsg.ReplyMarkup = keyboard

		bot.Send(responseMsg)
		return true
	// --------------------------------------------------
	// C) TUGMA MATNI (Button Text) QABUL QILISH
	// --------------------------------------------------
	case StateWaitingForAdButtonText:
		btnText := strings.TrimSpace(msg.Text)
		if btnText == "" {
			sendUserBot(bot, chatID, "❌ Tugma matni bo‘sh bo‘lishi mumkin emas!")
			return true
		}

		ad.ButtonText = btnText
		setPendingAd(userID, ad)
		setAdminState(userID, StateWaitingForAdAnimeCode)

		sendUserBot(
			bot,
			chatID,
			"anime kodni yuboring",
		)
		return true

	// --------------------------------------------------
	// D) ANIME KODI VA KANALLARNI KO'RSATISH
	// --------------------------------------------------
	case StateWaitingForAdAnimeCode:
		code := strings.TrimSpace(msg.Text)
		if code == "" {
			sendUserBot(bot, chatID, "❌ Anime kodi bo‘sh bo‘lishi mumkin emas!")
			return true
		}

		ad.AnimeCode = code
		setPendingAd(userID, ad)

		// State ni tozalaymiz (Reklama tayyor bo'ldi)
		clearAdminState(userID)

		sendUserBot(
			bot,
			chatID,
			"",
		)

		// Kanallar ro'yxatini chiqarish funksiyasini chaqiramiz
		showPromoChannelsForAd(bot, b, chatID)
		return true
	}

	return false
}

func SendAdToChannel(bot *tgbotapi.BotAPI, channelID int64, ad AdData) error {
	return sendAdToChannel(bot, channelID, ad)
}

func showPromoChannelsForDelete(bot *tgbotapi.BotAPI, b *models.CreatedBot, chatID int64) {
	o := orm.NewOrm()

	var channels []*models.PromoChannel

	_, err := o.QueryTable(new(models.PromoChannel)).
		Filter("Bot__Id", b.Id).
		Filter("IsActive", true).
		All(&channels)

	if err != nil {
		sendUserBot(bot, chatID, "❌ Kanallarni olishda xatolik.")
		return
	}

	if len(channels) == 0 {
		sendUserBot(bot, chatID, "❌ Hali faol reklama kanali yo'q.")
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton

	for _, ch := range channels {
		title := ch.Title
		if title == "" {
			title = fmt.Sprintf("📢 %d", ch.ChannelID)
		}

		callbackData := fmt.Sprintf("delete_promo_channel:%d", ch.Id)

		rows = append(
			rows,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🗑 "+title, callbackData),
			),
		)
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	msg := tgbotapi.NewMessage(chatID, "🗑 O'chirmoqchi bo'lgan kanalni tanlang:")
	msg.ReplyMarkup = keyboard

	bot.Send(msg)
}

func handleDeletePromoChannelCallback(bot *tgbotapi.BotAPI, b *models.CreatedBot, callback *tgbotapi.CallbackQuery) {
	if callback == nil {
		return
	}

	data := callback.Data

	if !strings.HasPrefix(data, "delete_promo_channel:") {
		return
	}

	channelDBIDStr := strings.TrimPrefix(data, "delete_promo_channel:")
	channelDBID, err := strconv.ParseInt(channelDBIDStr, 10, 64)

	if err != nil {
		bot.Request(tgbotapi.NewCallback(callback.ID, "❌ Kanal ID noto‘g‘ri!"))
		return
	}

	o := orm.NewOrm()

	channel := &models.PromoChannel{Id: channelDBID}
	err = o.Read(channel)

	if err != nil {
		bot.Request(tgbotapi.NewCallback(callback.ID, "❌ Kanal topilmadi!"))
		return
	}

	if channel.Bot == nil {
		_, err = o.LoadRelated(channel, "Bot")
		if err != nil {
			bot.Request(tgbotapi.NewCallback(callback.ID, "❌ Kanal bot ma'lumotlari topilmadi!"))
			return
		}
	}

	if channel.Bot == nil || channel.Bot.Id != b.Id {
		bot.Request(tgbotapi.NewCallback(callback.ID, "❌ Bu kanal ushbu botga tegishli emas!"))
		return
	}

	title := channel.Title
	if title == "" {
		title = fmt.Sprintf("%d", channel.ChannelID)
	}

	channel.IsActive = false
	_, err = o.Update(channel, "IsActive")

	if err != nil {
		log.Printf("❌ PromoChannel o‘chirishda xatolik: %v", err)
		bot.Request(tgbotapi.NewCallback(callback.ID, "❌ Kanalni o‘chirishda xatolik yuz berdi!"))
		return
	}

	bot.Request(tgbotapi.NewCallback(callback.ID, "✅ Kanal o‘chirildi!"))

	if callback.Message != nil {
		editMsg := tgbotapi.NewEditMessageText(
			callback.Message.Chat.ID,
			callback.Message.MessageID,
			fmt.Sprintf("✅ Kanal o‘chirildi: %s", title),
		)
		bot.Send(editMsg)
	}
}

func showTopAnimePro(bot *tgbotapi.BotAPI, b *models.CreatedBot, chatID int64) {
	o := orm.NewOrm()
	var animes []models.Anime

	_, err := o.QueryTable(new(models.Anime)).
		Filter("Bot__Id", b.Id).
		OrderBy("-ViewsCount").
		Limit(10).
		All(&animes)

	if err != nil || len(animes) == 0 {
		sendUserBot(bot, chatID, "❌ Hozircha hech qanday anime topilmadi.")
		return
	}

	text := "<b>Eng ko‘p ko‘rilgan 10 ta anime</b>\n\nKodni bosib tomosha qiling:"

	var rows [][]tgbotapi.InlineKeyboardButton

	for _, a := range animes {
		name := a.Name
		if len([]rune(name)) > 22 {
			name = string([]rune(name)[:22]) + "..."
		}
		btnText := fmt.Sprintf("[%d - marta]  %s", a.ViewsCount, name)

		// Endi callback: top:BOTID:CODE
		btn := tgbotapi.NewInlineKeyboardButtonData(btnText, fmt.Sprintf("top:%d:%s", b.Id, a.Code))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
	}
	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}
