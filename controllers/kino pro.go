package controllers

import (
	"creator/models"
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"strconv"
	"strings"
	"time"
)

func HandleKinoBotMessagePro(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
	userID := msg.From.ID
	chatID := msg.Chat.ID
	botUser := GetOrCreateBotUser(b, userID, msg.From)
	if botUser.IsBlocked {
		sendUserBot(bot, chatID, "🚫 Siz botdan foydalanishdan bloklangansiz!")
		return
	}
	isNewCommand :=
		msg.Text == "/admin" ||
			msg.Text == "/addkino" ||
			msg.Text == "/addchannel" ||
			msg.Text == "/delchannel" ||
			msg.Text == "/ok" ||
			msg.Text == "/vipnarx" ||
			msg.Text == "➕ Kino joylash" ||
			msg.Text == "➕ Kanal qo‘shish" ||
			msg.Text == "➖ Kanall o‘chirish" ||
			msg.Text == "✏️ Kinoni tahrirlash" ||
			msg.Text == "🗑 Kinoni o‘chirish" ||
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
			msg.Text == "/delkino" ||
			msg.Text == "🎬 qismli kino joylash" ||
			msg.Text == "/editkino" ||
			msg.Text == "Kanall qo‘shish"

	if isNewCommand && msg.Text != "/ok" && isAdmin(b, userID) {
		mu.Lock()
		delete(adminState, userID)
		delete(quickKinoTemp, userID)
		delete(adminTempChannel, userID)
		delete(adminKinoID, userID) // ← qo‘shing
		mu.Unlock()
	}
	if isAdmin(b, userID) {
		mu.Lock()
		state, exists := adminState[userID]
		mu.Unlock()

		if exists {
			if RouteKinoUploadState(bot, b, msg, state) {
				return
			}

			if RouteKinoQuickState(bot, b, msg, state) {
				return
			}

			if RouteKinoEditStatePro(bot, b, msg, state) {
				return
			}

			if RouteUserManagementState(bot, b, msg, state) {
				return
			}

			if state == "wait_channel" || state == "wait_link" {
				HandleAdminCommands(bot, b, msg)
				return
			}

			if strings.HasPrefix(state, "waiting_broadcast_message:") {
				target := strings.TrimPrefix(state, "waiting_broadcast_message:")
				mu.Lock()
				delete(adminState, userID)
				mu.Unlock()

				RunBroadcast(bot, b, msg, target)
				return
			}
			if strings.HasPrefix(state, "wait_vip_edit_name:") ||
				strings.HasPrefix(state, "wait_vip_edit_price:") ||
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
			if b.BotType.Code == "kinopro" {
				showAdminPanelKinoPropr(bot, chatID)
			} else {
				showAdminPanelKinoPro(bot, chatID)
			}
			return

		case "➕ Kino joylash", "Kino joylash", "/addkino":
			StartKinoUpload(bot, b, msg)
			return

		case "🎬 qismli kino joylash", "qismli kino joylash":
			StartQuickKinoUploadPro(bot, b, msg)
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

		case "✏️ Kinoni tahrirlash", "Kinoni tahrirlash", "/editkino":
			mu.Lock()
			adminState[userID] = "waiting_kino_edit_code"
			mu.Unlock()

			sendUserBot(bot, chatID, "✏️Tahrirlash qismi\n\nO'zgartirmoqchi bo'lgan kino kodini yozib yuboring:")
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

		case "🗑 Kinoni o‘chirish", "Kinoni o'chirish", "/delkino":
			mu.Lock()
			adminState[userID] = "waiting_kino_delete_code"
			mu.Unlock()

			sendUserBot(bot, chatID, "🗑 Kinoni o'chirish\n\nO'chirmoqchi bo'lgan kino kodini yozib yuboring:")
			return

		case "✍️ Matn sozlash", "Matn sozlash":
			log.Printf("🟦 '✍️ Matn' case ishga tushdi, UserID=%d, ChatID=%d", userID, chatID)
			showKinoNoteMenuPro(bot, b, chatID, userID)
			return

		case "👥 Foydalanuvchilar", "Foydalanuvchilar":
			if b.BotType.Code == "kinopro" {
				showUsersPanelPropr(bot, chatID)
			} else {
				showUsersPanelPro(bot, chatID)
			}
			return

		case "💎 vip sozlash", "vip sozlash":
			if b.BotType.Code == "kinopro" {
				showVIPPanelPropr(bot, chatID)
			} else {
				showVIPPanelPro(bot, chatID)
			}
			return

		case "📢 Reklama sozlash", "Reklama sozlash":
			if b.BotType.Code == "kinopro" {
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
			if b.BotType.Code == "kinopro" {
				showAdminPanelKinoPropr(bot, chatID)
			} else {
				showAdminPanelKinoPro(bot, chatID)
			}
			return

		case "📬 Reklama yuborish", "Reklama yuborish":
			if b.BotType.Code == "kinopro" {
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
			showStatisticsKinoPro(bot, b, chatID)
			return

		case "📤 Kino sozlash", "Kino sozlash":
			if b.BotType.Code == "kinopro" {
				startKinoyuklashPropr(bot, chatID)
			} else {
				startKinoyuklashPro(bot, chatID)
			}
			return

		case "📢 Kanallarni sozlash", "Kanallarni sozlash":
			if b.BotType.Code == "kinopro" {
				showAdminsPanelProprKino(bot, chatID)
			} else {
				showAdminsPanelPro(bot, chatID)
			}
			return

		case "🗄 Asosiy sozlamalar", "Asosiy sozlamalar":
			if b.BotType.Code == "kinopro" {
				showAsosiyPanelProprKino(bot, chatID)
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
		handleKinoStartPro(bot, b, msg)
		return
	}

	switch msg.Text {
	case "/start":
		handleKinoStartPro(bot, b, msg)
		return

	case "/help":
		sendUserBot(bot, chatID, "🎬 Kino kodini yozing...")
		return
	case "reyting", "/reyting", "Reyting", "REYTING", "рейтинг", "Рейтинг":
		showTopKinoPro(bot, b, chatID)
		return
	default:
		if strings.HasPrefix(msg.Text, "/") {
			sendUserBot(bot, chatID, "/admin")
			return
		}
		handleKinoByCodePro(bot, b, msg, msg.Text)
		return
	}
}
func showAsosiyPanelProprKino(bot *tgbotapi.BotAPI, chatID int64) {
	keyboardJSON := `{
		"keyboard": [
			[
				{"text": "Matn sozlash", "icon_custom_emoji_id": "5276020994153161924"},
				{"text": "vip sozlash", "icon_custom_emoji_id": "5276393256148570924"}
			],
			[
				{"text": "Reklama sozlash", "icon_custom_emoji_id": "5273763683896434957"}
			],
			[
				{"text": "Orqaga", "icon_custom_emoji_id": "5274128056036928934"}
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

func showAdminsPanelProprKino(bot *tgbotapi.BotAPI, chatID int64) {
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

func StartQuickKinoUploadPro(bot *tgbotapi.BotAPI, _ *models.CreatedBot, msg *tgbotapi.Message) {
	userID := msg.From.ID
	chatID := msg.Chat.ID

	mu.Lock()
	adminState[userID] = "waiting_quick_kino_name"
	mu.Unlock()

	sendUserBot(bot, chatID, "Kino haqida malumot yozib yuboring:")
}

func showKinoNoteMenuPro(bot *tgbotapi.BotAPI, b *models.CreatedBot, chatID int64, userID int64) {
	// Har safar bazadan eng so'nggi qiymatni olamiz — chunki delete_kino_note/edit_kino_note
	// callback'lari xotiradagi b obyektiga to'g'ridan-to'g'ri kira olmaydi.
	o := orm.NewOrm()
	var fresh models.CreatedBot
	if err := o.QueryTable(new(models.CreatedBot)).Filter("Id", b.Id).One(&fresh); err == nil {
		b.Note = fresh.Note
	} else {
		log.Printf("🔴 showKinoNoteMenuPro: bazadan yangilashda xatolik: %v", err)
	}

	if b.Note == "" {
		mu.Lock()
		adminState[userID] = "waiting_kino_note"
		mu.Unlock()

		msg := tgbotapi.NewMessage(chatID, "✍️ *Kino osti matni yo'q.*\n\nBarcha kinolar/qismlar ostida doimiy chiqib turadigan reklama yoki izoh matnini kiriting:")
		msg.ParseMode = "Markdown"

		if _, sendErr := bot.Send(msg); sendErr != nil {
			log.Printf("🔴 showKinoNoteMenuPro: xabar yuborishda xatolik: %v", sendErr)
		}
		return
	}

	text := fmt.Sprintf("🎬 *Joriy kino osti matni:*\n\n%s", b.Note)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✏️ Tahrirlash", fmt.Sprintf("edit_kino_note:%d", b.Id)),
			tgbotapi.NewInlineKeyboardButtonData("🗑 O'chirish", fmt.Sprintf("delete_kino_note:%d", b.Id)),
		),
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard

	if _, sendErr := bot.Send(msg); sendErr != nil {
		log.Printf("🔴 showKinoNoteMenuPro: matnli xabar yuborishda xatolik: %v", sendErr)
	}
}

func showAdminPanelKinoPro(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📊 Statistika"),
			tgbotapi.NewKeyboardButton("👥 Foydalanuvchilar"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📤 Kino sozlash"),
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
		log.Printf("🔴 showAdminPanelKinoPro: xabar yuborishda xatolik: %v", err)
	}
}

func startKinoyuklashPro(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("➕ Kino joylash"),
			tgbotapi.NewKeyboardButton("🗑 Kinoni o'chirish"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("✏️ Kinoni tahrirlash"),
			tgbotapi.NewKeyboardButton("🎬 qismli kino joylash"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⬅️ Orqaga"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "📋 Quyidagilardan birini tanlang")
	msg.ReplyMarkup = keyboard

	if _, err := bot.Send(msg); err != nil {
		log.Printf("🔴 startKinoyuklash: xabar yuborishda xatolik: %v", err)
	}
}

func showAdminPanelKinoPropr(bot *tgbotapi.BotAPI, chatID int64) {
	keyboardJSON := `{
		"keyboard": [
			[
				{"text": "Statistika", "icon_custom_emoji_id": "5300849371162646845"},
				{"text": "Foydalanuvchilar", "icon_custom_emoji_id": "5276483102569438072"}
			],
			[
				{"text": "Kino sozlash", "icon_custom_emoji_id": "5292207604905320293"},
				{"text": "Kanallarni sozlash", "icon_custom_emoji_id": "5273763683896434957"}
			],
			[
				{"text": "Asosiy sozlamalar", "icon_custom_emoji_id": "5291998508717487652"}
			],
			[
				{"text": "Reklama yuborish", "icon_custom_emoji_id": "5274078277365963912"}
			]
		],
		"resize_keyboard": true
	}`

	params := tgbotapi.Params{}
	params.AddNonEmpty("chat_id", strconv.FormatInt(chatID, 10))
	params.AddNonEmpty("text", "admin")
	params.AddNonEmpty("reply_markup", keyboardJSON)

	if _, err := bot.MakeRequest("sendMessage", params); err != nil {
		log.Printf("🔴 showAdminPanelKinoPropr: xabar yuborishda xatolik: %v", err)
	}
}

func startKinoyuklashPropr(bot *tgbotapi.BotAPI, chatID int64) {
	keyboardJSON := `{
		"keyboard": [
			[
				{"text": "Kino joylash", "icon_custom_emoji_id": "5292138644910415028"},
				{"text": "Kinoni o'chirish", "icon_custom_emoji_id": "5292178777084828091"}
			],
			[
				{"text": "Kinoni tahrirlash", "icon_custom_emoji_id": "5294373221905242428"},
				{"text": "qismli kino joylash", "icon_custom_emoji_id": "5294322773219384907"}
			],
			[
				{"text": "Orqaga", "icon_custom_emoji_id": "5274128056036928934"}
			]
		],
		"resize_keyboard": true
	}`

	params := tgbotapi.Params{}
	params.AddNonEmpty("chat_id", strconv.FormatInt(chatID, 10))
	params.AddNonEmpty("text", " Quyidagilardan birini tanlang")
	params.AddNonEmpty("reply_markup", keyboardJSON)

	if _, err := bot.MakeRequest("sendMessage", params); err != nil {
		log.Printf("🔴 startKinoyuklashPropr: xabar yuborishda xatolik: %v", err)
	}
}

func handleKinoStartPro(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
	args := strings.TrimSpace(strings.TrimPrefix(msg.Text, "/start"))

	if args != "" {
		handleKinoByCodePro(bot, b, msg, args)
		return
	}

	text := `<tg-emoji emoji-id="5960714428394507968">🏷</tg-emoji> /reyting - Top Kino
<tg-emoji emoji-id="5899757765743615694">📥</tg-emoji> /admin - Admin uchun 
<tg-emoji emoji-id="5899757765743615694"></tg-emoji>
<tg-emoji emoji-id="5987802868734760945">🆔</tg-emoji> Kino nomi yoki kodini kiriting:`

	sendUserBot(bot, msg.Chat.ID, text)
}

func handleKinoByCodePro(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message, code string) {
	fmt.Println("========== KINO SEARCH ==========")
	fmt.Println("BotID:", b.Id)
	fmt.Println("Code:", code)

	userID := msg.From.ID
	_ = GetOrCreateBotUser(b, userID, msg.From)

	// Protect qoida:
	//    - Admin → protect = false (uzata oladi)
	//    - VIP va oddiy → protect = true (uzata olmaydi)
	protectContent := !isAdmin(b, userID)

	o := orm.NewOrm()
	var kino models.Kino

	// 1. Avval kinoni bazadan qidirib topamiz
	err := o.QueryTable(new(models.Kino)).
		Filter("Bot__Id", b.Id).
		Filter("Code", strings.ToLower(strings.TrimSpace(code))).
		One(&kino)

	if err != nil {
		fmt.Println("KINO NOT FOUND BY CODE, FALLBACK TO SEARCH:", err)
		handleKinoSearchPro(bot, b, msg, code)
		return
	}

	fmt.Println("FOUND KINO:")
	fmt.Println("ID:", kino.Id)
	fmt.Println("Name:", kino.Name)
	fmt.Println("Code:", kino.Code)
	fmt.Println("Parts:", kino.PartsCount)
	fmt.Println("PhotoID:", kino.PhotoID)

	// ========== VIEWS COUNT OSHIRISH ==========
	kino.ViewsCount++
	if _, err := o.Update(&kino, "ViewsCount"); err != nil {
		log.Printf("ViewsCount yangilashda xatolik: %v", err)
	}

	// 2. Bir qismli kino bo'lsa
	if kino.PartsCount == 1 {
		sendSingleKinoPartPro(bot, o, msg.Chat.ID, &kino, b.Note, protectContent)
		return
	}

	// 3. Ko'p qismli kino uchun matn tayyorlaymiz
	yearLine := ""
	if kino.Year > 0 {
		yearLine = fmt.Sprintf(`<tg-emoji emoji-id="5987802868734760945">📅</tg-emoji>Yili: %d`+"\n", kino.Year)
	}

	caption := fmt.Sprintf(
		"%s\n\n"+
			yearLine+
			`<tg-emoji emoji-id="5273729023510354469">🏷</tg-emoji>Jami qismlar: %d`+"\n"+
			`<tg-emoji emoji-id="5276277025743607642">📥</tg-emoji>Yuklab olishlar: %d`+"\n"+
			`<tg-emoji emoji-id="5298761235372745123">🆔</tg-emoji>Kino kodi: %s`,
		kino.Name,
		kino.PartsCount,
		kino.ViewsCount,
		kino.Code,
	)
	if b.Note != "" {
		caption += fmt.Sprintf("\n\n%s", b.Note)
	}

	var keyboard tgbotapi.InlineKeyboardMarkup
	hasKeyboard := false

	if kino.PartsCount > 0 {
		keyboard = buildKinoPartsKeyboardPro(kino.Id, kino.PartsCount, 1)
		hasKeyboard = true
	} else {
		caption += "\n⚠️ Tez orada qismlar joylanadi!"
	}

	var sendErr error

	if kino.PhotoID != "" {
		var kb *tgbotapi.InlineKeyboardMarkup
		if hasKeyboard {
			kb = &keyboard
		}
		sendErr = sendProtectedPhotoPro(bot, msg.Chat.ID, kino.PhotoID, caption, kb, protectContent)

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
		fmt.Println("SEND KINO ERROR:", sendErr)
		fallback := tgbotapi.NewMessage(msg.Chat.ID, "❌ Kechirasiz, ma'lumotni yuborishda texnik xatolik yuz berdi. Iltimos, keyinroq qayta urinib ko'ring yoki admin bilan bog'laning.")
		if _, fbErr := bot.Send(fallback); fbErr != nil {
			fmt.Println("SEND FALLBACK MESSAGE ERROR:", fbErr)
		}
	}
}

func handleKinoSearchPro(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message, query string) {
	fmt.Println("========== KINO NAME SEARCH ==========")
	fmt.Println("BotID:", b.Id)
	fmt.Println("Query:", query)

	query = strings.TrimSpace(query)
	if query == "" {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "🔍 Qidiruv uchun biror so‘z yoki kod yozing.")
		bot.Send(reply)
		return
	}

	o := orm.NewOrm()
	var kinos []models.Kino

	_, err := o.QueryTable(new(models.Kino)).
		Filter("Bot__Id", b.Id).
		Filter("Name__icontains", query).
		OrderBy("Name").
		Limit(20).
		All(&kinos)

	if err != nil {
		fmt.Println("SEARCH ERROR:", err)
		reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Qidiruvda xatolik yuz berdi. Keyinroq qayta urinib ko‘ring.")
		bot.Send(reply)
		return
	}

	if len(kinos) == 0 {
		text := fmt.Sprintf("🔍 «%s» bo‘yicha hech qanday kino topilmadi.\n\nBoshqa so‘z yoki kod bilan qidirib ko‘ring.", query)
		reply := tgbotapi.NewMessage(msg.Chat.ID, text)
		bot.Send(reply)
		return
	}

	// Kodi faqat harflardan iborat bo'lgan kinolar qidiruvda ko'rsatilmaydi
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, k := range kinos {
		if isCodeOnlyLetters(k.Code) {
			continue
		}
		btnText := k.Name
		if k.Code != "" {
			btnText = fmt.Sprintf("%s  [%s]", k.Name, k.Code)
		}
		btn := tgbotapi.NewInlineKeyboardButtonData(btnText, fmt.Sprintf("kino_select_%d", k.Id))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
	}

	if len(rows) == 0 {
		text := fmt.Sprintf("🔍 «%s» bo‘yicha hech qanday kino topilmadi.\n\nBoshqa so‘z yoki kod bilan qidirib ko‘ring.", query)
		reply := tgbotapi.NewMessage(msg.Chat.ID, text)
		bot.Send(reply)
		return
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	text := fmt.Sprintf("«%s» bo‘yicha topilgan kinolar (%d ta):\n\n", query, len(rows))
	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ReplyMarkup = keyboard

	if _, sendErr := bot.Send(reply); sendErr != nil {
		fmt.Println("SEND SEARCH RESULTS ERROR:", sendErr)
	}
}

func isCodeOnlyLetters(code string) bool {
	if code == "" {
		return false
	}
	for _, r := range code {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return false
		}
	}
	return true
}

func buildKinoPartsKeyboardPro(kinoID int64, totalParts int, page int) tgbotapi.InlineKeyboardMarkup {
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
			fmt.Sprintf("kino_part:%d:%d", kinoID, i),
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
				fmt.Sprintf("kino_page:%d:%d", kinoID, page-1),
			),
		)
	}

	if end < totalParts {
		nav = append(nav,
			tgbotapi.NewInlineKeyboardButtonData(
				">",
				fmt.Sprintf("kino_page:%d:%d", kinoID, page+1),
			),
		)
	}

	if len(nav) > 0 {
		rows = append(rows, nav)
	}

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func sendSingleKinoPartPro(bot *tgbotapi.BotAPI, o orm.Ormer, chatID int64, kino *models.Kino, botNote string, protectContent bool) {
	var part models.KinoPart
	err := o.QueryTable(new(models.KinoPart)).
		Filter("Kino__Id", kino.Id).
		Filter("PartOrder", 1).
		One(&part)

	if err != nil {
		fmt.Println("SINGLE PART NOT FOUND:", err)
		fallback := tgbotapi.NewMessage(chatID, "❌ Fayl topilmadi. Iltimos, admin bilan bog'laning.")
		bot.Send(fallback)
		return
	}

	caption := kino.Name
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

func handleKinoPagePro(bot *tgbotapi.BotAPI, cb *tgbotapi.CallbackQuery) {

	parts := strings.Split(cb.Data, ":")

	kinoID, _ := strconv.ParseInt(parts[1], 10, 64)
	page, _ := strconv.Atoi(parts[2])

	o := orm.NewOrm()

	var kino models.Kino
	kino.Id = kinoID

	if err := o.Read(&kino); err != nil {
		return
	}

	kb := buildKinoPartsKeyboardPro(
		kinoID,
		kino.PartsCount,
		page,
	)

	edit := tgbotapi.NewEditMessageReplyMarkup(
		cb.Message.Chat.ID,
		cb.Message.MessageID,
		kb,
	)

	bot.Send(edit)
}

func RouteKinoEditStatePro(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message, state string) bool {
	userID := msg.From.ID
	chatID := msg.Chat.ID
	o := orm.NewOrm()

	if state == "waiting_kino_edit_code" {
		code := strings.ToLower(strings.TrimSpace(msg.Text))
		var kino models.Kino
		err := o.QueryTable(new(models.Kino)).
			Filter("Bot__Id", b.Id).
			Filter("Code", code).
			One(&kino)

		if err != nil {
			sendUserBot(bot, chatID, "❌ Ushbu botga tegishli bunday kodli kino topilmadi.\n\nQaytadan to'g'ri kod kiriting:")
			return true
		}

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔑 Kodni o'zgartirish", fmt.Sprintf("kino_edit_code:%d", kino.Id)),
				tgbotapi.NewInlineKeyboardButtonData("📝 Nomni o'zgartirish", fmt.Sprintf("kino_edit_name:%d", kino.Id)),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("➕ Qism qo'shish", fmt.Sprintf("kino_edit_addpart:%d", kino.Id)),
				tgbotapi.NewInlineKeyboardButtonData("➖ Qism o'chirish", fmt.Sprintf("kino_edit_delpart:%d", kino.Id)),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🖼 Rasmini o'zgartirish", fmt.Sprintf("kino_edit_photo:%d", kino.Id)),
				tgbotapi.NewInlineKeyboardButtonData("📅 Yilini o'zgartirish", fmt.Sprintf("kino_edit_year:%d", kino.Id)),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🗑 Butunlay o'chirish", fmt.Sprintf("delete_kino:%d", kino.Id)),
			),
		)

		caption := fmt.Sprintf("✅ Kino topildi:\n\n📌 *%s*\n🔑 Kod: `%s`\n🎬 Qismlar: %d\n\nNima qilmoqchisiz?",
			kino.Name, kino.Code, kino.PartsCount)

		if kino.PhotoID != "" {
			photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileID(kino.PhotoID))
			photo.Caption = caption
			photo.ParseMode = "Markdown"
			photo.ReplyMarkup = keyboard

			if _, sendErr := bot.Send(photo); sendErr != nil {
				fmt.Println("SEND EDIT KINO PHOTO ERROR (fallback to text):", sendErr)
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

	if state == "waiting_kino_delete_code" {
		code := strings.ToLower(strings.TrimSpace(msg.Text))
		var kino models.Kino

		// Ma'lumotlar bazasidan qidirish va xatolikni tekshirish
		err := o.QueryTable(new(models.Kino)).
			Filter("Bot__Id", b.Id).
			Filter("Code", code).
			One(&kino)

		if err != nil {
			// Kino topilmadi
			text := fmt.Sprintf("🔍 «%s» kodi bo‘yicha hech qanday kino/anime topilmadi.\n\nBoshqa kod kiritib ko‘ring.", code)
			reply := tgbotapi.NewMessage(msg.Chat.ID, text)
			bot.Send(reply)
			return true
		}

		kinoName := kino.Name

		// Kino qismlarini (KinoPart) o'chirish
		_, err = o.QueryTable(new(models.KinoPart)).Filter("Kino__Id", kino.Id).Delete()
		if err != nil {
			log.Printf("KinoPart o'chirishda xatolik: %v", err)
		}

		// Kinoning o'zini o'chirish
		_, err = o.Delete(&kino)
		if err != nil {
			sendUserBot(bot, chatID, "❌ Kinoni o'chirishda xatolik yuz berdi.")
			return true
		}

		log.Printf("🗑️ Kino o'chirildi! ID: %d, Nomi: %s", kino.Id, kinoName)

		// Admin holatini (state) tozalash
		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()

		sendUserBot(bot, chatID, fmt.Sprintf("✅ \"%s\" kino butunlay o'chirildi!", kinoName))
		return true
	}
	if strings.HasPrefix(state, "waiting_new_kino_code:") {
		kinoID, _ := strconv.ParseInt(strings.TrimPrefix(state, "waiting_new_kino_code:"), 10, 64)
		newCode := strings.ToLower(strings.TrimSpace(msg.Text))

		o.QueryTable(new(models.Kino)).
			Filter("Id", kinoID).
			Filter("Bot__Id", b.Id).
			Update(orm.Params{"Code": newCode})

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()

		sendUserBot(bot, chatID, "✅ Kino kodi muvaffaqiyatli o'zgartirildi!")
		return true
	}

	if strings.HasPrefix(state, "waiting_new_kino_name:") {
		kinoID, _ := strconv.ParseInt(strings.TrimPrefix(state, "waiting_new_kino_name:"), 10, 64)

		o.QueryTable(new(models.Kino)).
			Filter("Id", kinoID).
			Filter("Bot__Id", b.Id).
			Update(orm.Params{"Name": msg.Text})

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()

		sendUserBot(bot, chatID, "✅ Kino nomi muvaffaqiyatli o'zgartirildi!")
		return true
	}

	if strings.HasPrefix(state, "waiting_new_kino_year:") {
		kinoID, _ := strconv.ParseInt(strings.TrimPrefix(state, "waiting_new_kino_year:"), 10, 64)
		year, err := strconv.Atoi(strings.TrimSpace(msg.Text))
		if err != nil || year <= 0 {
			sendUserBot(bot, chatID, "❌ Noto'g'ri yil. Faqat raqam kiriting (masalan 2024):")
			return true
		}

		o.QueryTable(new(models.Kino)).
			Filter("Id", kinoID).
			Filter("Bot__Id", b.Id).
			Update(orm.Params{"Year": year})

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()

		sendUserBot(bot, chatID, "✅ Kino yili muvaffaqiyatli o'zgartirildi!")
		return true
	}

	if strings.HasPrefix(state, "waiting_new_kino_part_file:") {
		kinoID, _ := strconv.ParseInt(strings.TrimPrefix(state, "waiting_new_kino_part_file:"), 10, 64)

		var kino models.Kino
		err := o.QueryTable(new(models.Kino)).
			Filter("Id", kinoID).
			Filter("Bot__Id", b.Id).
			One(&kino)

		if err != nil {
			sendUserBot(bot, chatID, "❌ Xatolik: Kino topilmadi.")
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

		newPartOrder := kino.PartsCount + 1

		newPart := models.KinoPart{
			Kino:      &kino,
			PartOrder: newPartOrder,
			FileID:    fileID,
			Kind:      kind,
		}

		if _, err := o.Insert(&newPart); err == nil {
			kino.PartsCount = newPartOrder
			o.Update(&kino, "PartsCount")

			mu.Lock()
			delete(adminState, userID)
			mu.Unlock()

			sendUserBot(bot, chatID, fmt.Sprintf("✅ Yangi %d-qism muvaffaqiyatli qo'shildi!", newPartOrder))
		}
		return true
	}

	if strings.HasPrefix(state, "waiting_del_kino_part_num:") {
		kinoID, _ := strconv.ParseInt(strings.TrimPrefix(state, "waiting_del_kino_part_num:"), 10, 64)
		partNum, err := strconv.Atoi(strings.TrimSpace(msg.Text))

		if err != nil || partNum <= 0 {
			sendUserBot(bot, chatID, "❌ Noto'g'ri qism raqami. Faqat musbat raqam kiriting:")
			return true
		}

		var kino models.Kino
		if o.QueryTable(new(models.Kino)).
			Filter("Id", kinoID).
			Filter("Bot__Id", b.Id).
			One(&kino) != nil {
			sendUserBot(bot, chatID, "❌ Kino topilmadi.")
			return true
		}

		// 1. Kerakli qismni o'chiramiz
		num, err := o.QueryTable(new(models.KinoPart)).
			Filter("Kino__Id", kinoID).
			Filter("PartOrder", partNum).
			Delete()

		if err != nil || num == 0 {
			sendUserBot(bot, chatID, "❌ Bunday raqamli qism topilmadi. Qaytadan urinib ko'ring:")
			return true
		}

		// 2. O'chirilgan raqamdan keyingi barcha qismlarni 1 ta pastga siljitamiz
		var partsToShift []models.KinoPart
		_, _ = o.QueryTable(new(models.KinoPart)).
			Filter("Kino__Id", kinoID).
			Filter("PartOrder__gt", partNum).
			OrderBy("PartOrder").
			All(&partsToShift)

		for _, p := range partsToShift {
			p.PartOrder = p.PartOrder - 1
			o.Update(&p, "PartOrder")
		}

		// 3. PartsCount ni yangilaymiz
		if kino.PartsCount > 0 {
			kino.PartsCount -= 1
			o.Update(&kino, "PartsCount")
		}

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()

		sendUserBot(bot, chatID, fmt.Sprintf("✅ %d-qism muvaffaqiyatli o'chirildi!\nQolgan qismlar qayta raqamlandi.", partNum))
		return true
	}
	if strings.HasPrefix(state, "waiting_new_kino_photo:") {
		if msg.Photo == nil || len(msg.Photo) == 0 {
			sendUserBot(bot, chatID, "❌ Iltimos, faqat rasm yuboring:")
			return true
		}
		kinoID, _ := strconv.ParseInt(strings.TrimPrefix(state, "waiting_new_kino_photo:"), 10, 64)
		newPhotoID := msg.Photo[len(msg.Photo)-1].FileID

		o.QueryTable(new(models.Kino)).
			Filter("Id", kinoID).
			Filter("Bot__Id", b.Id).
			Update(orm.Params{"PhotoID": newPhotoID})

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()

		sendUserBot(bot, chatID, "✅ Kino bosh sahifa surati muvaffaqiyatli o'zgartirildi!")
		return true
	}
	// --- KINO OSTI MATNI (BOT DARAJASIDAGI NOTE) ---
	if state == "waiting_kino_note" {
		// Bekor qilish
		if msg.Text == "/cancel" || msg.Text == "Orqaga" {
			mu.Lock()
			delete(adminState, userID)
			mu.Unlock()
			sendUserBot(bot, chatID, "❌ Bekor qilindi")
			return true
		}

		newNote := strings.TrimSpace(msg.Text)

		o := orm.NewOrm()
		_, err := o.QueryTable(new(models.CreatedBot)).
			Filter("Id", b.Id).
			Update(orm.Params{"Note": newNote})

		if err != nil {
			sendUserBot(bot, chatID, "❌ Saqlashda xatolik: "+err.Error())
			return true
		}

		// Xotiradagi b obyektini ham yangilab qo‘yamiz
		b.Note = newNote

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()

		sendUserBot(bot, chatID, "✅ Kino osti matni muvaffaqiyatli saqlandi!")
		// Xohlasangiz qayta menyuni chiqarishingiz mumkin:
		// showKinoNoteMenuPro(bot, b, chatID, userID)
		return true
	}
	return false
}

func showStatisticsKinoPro(bot *tgbotapi.BotAPI, b *models.CreatedBot, chatID int64) {
	o := orm.NewOrm()
	now := time.Now()

	totalKino, _ := o.QueryTable(new(models.Kino)).
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
			"• Jami kino: `%d`\n"+
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
		totalKino,
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

func HandleKinoCallbackPro(bot *tgbotapi.BotAPI, cb *tgbotapi.CallbackQuery) {
	if cb.Message == nil {
		return
	}
	data := cb.Data
	chatID := cb.Message.Chat.ID
	userID := cb.From.ID

	bot.Request(tgbotapi.NewCallback(cb.ID, ""))

	log.Printf("Kino callback keldi: %s (Chat ID: %d)", data, chatID)

	switch {
	case strings.HasPrefix(data, "vip_prices_"):
		botID, err := strconv.ParseInt(strings.TrimPrefix(data, "vip_prices_"), 10, 64)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Bot ID aniqlanmadi"))
			return
		}
		HandleVipPricesCallback(bot, cb, botID)
		return

	case strings.HasPrefix(data, "kino_edit_year:"):
		kinoID, _ := strconv.ParseInt(strings.TrimPrefix(data, "kino_edit_year:"), 10, 64)
		mu.Lock()
		adminState[userID] = fmt.Sprintf("waiting_new_kino_year:%d", kinoID)
		mu.Unlock()
		bot.Send(tgbotapi.NewMessage(chatID, "📅 Yangi yilni kiriting (masalan 2024):"))
		return

	case strings.HasPrefix(data, "top_kino:"):
		parts := strings.Split(strings.TrimPrefix(data, "top_kino:"), ":")
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
		handleKinoByCodePro(bot, &createdBot, fakeMsg, code)
		return

	case strings.HasPrefix(data, "kino_select_"):
		idStr := strings.TrimPrefix(data, "kino_select_")
		kinoID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Kino ID aniqlanmadi."))
			return
		}

		o := orm.NewOrm()
		var kino models.Kino
		err = o.QueryTable(new(models.Kino)).
			Filter("Id", kinoID).
			RelatedSel("Bot").
			One(&kino)
		if err != nil || kino.Bot == nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Kino topilmadi."))
			return
		}

		botUser := GetOrCreateBotUser(kino.Bot, userID, cb.From)
		protectContent := !(botUser.IsVip || isAdmin(kino.Bot, userID))

		if kino.PartsCount == 1 {
			sendSingleKinoPartPro(bot, o, chatID, &kino, kino.Bot.Note, protectContent)
		} else {
			caption := fmt.Sprintf("%s\n\nJami qismlar - %d", kino.Name, kino.PartsCount)
			if kino.Bot.Note != "" {
				caption += fmt.Sprintf("\n\n%s", kino.Bot.Note)
			}
			keyboard := buildKinoPartsKeyboardPro(kino.Id, kino.PartsCount, 1)

			if kino.PhotoID != "" {
				_ = sendProtectedPhotoPro(bot, chatID, kino.PhotoID, caption, &keyboard, protectContent)
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

	case strings.HasPrefix(data, "kino_page:"):
		handleKinoPagePro(bot, cb)

	case strings.HasPrefix(data, "kino_part:"):
		handleKinoPartPro(bot, cb)

	case strings.HasPrefix(data, "kino_edit_code:"):
		kinoID, _ := strconv.ParseInt(strings.TrimPrefix(data, "kino_edit_code:"), 10, 64)
		mu.Lock()
		adminState[userID] = fmt.Sprintf("waiting_new_kino_code:%d", kinoID)
		mu.Unlock()
		bot.Send(tgbotapi.NewMessage(chatID, "🔑 Yangi kodni kiriting:"))
		return

	case strings.HasPrefix(data, "kino_edit_name:"):
		kinoID, _ := strconv.ParseInt(strings.TrimPrefix(data, "kino_edit_name:"), 10, 64)
		mu.Lock()
		adminState[userID] = fmt.Sprintf("waiting_new_kino_name:%d", kinoID)
		mu.Unlock()
		bot.Send(tgbotapi.NewMessage(chatID, "📝 Yangi nomni kiriting:"))
		return

	case strings.HasPrefix(data, "kino_edit_addpart:"):
		kinoID, _ := strconv.ParseInt(strings.TrimPrefix(data, "kino_edit_addpart:"), 10, 64)
		mu.Lock()
		adminState[userID] = fmt.Sprintf("waiting_new_kino_part_file:%d", kinoID)
		mu.Unlock()
		bot.Send(tgbotapi.NewMessage(chatID, "➕ Yangi qism uchun video, rasm yoki hujjat yuboring:"))
		return

	case strings.HasPrefix(data, "kino_edit_delpart:"):
		kinoID, _ := strconv.ParseInt(strings.TrimPrefix(data, "kino_edit_delpart:"), 10, 64)
		mu.Lock()
		adminState[userID] = fmt.Sprintf("waiting_del_kino_part_num:%d", kinoID)
		mu.Unlock()
		bot.Send(tgbotapi.NewMessage(chatID, "➖ O'chirmoqchi bo'lgan qism raqamini kiriting:"))
		return

	case strings.HasPrefix(data, "kino_edit_photo:"):
		kinoID, _ := strconv.ParseInt(strings.TrimPrefix(data, "kino_edit_photo:"), 10, 64)
		mu.Lock()
		adminState[userID] = fmt.Sprintf("waiting_new_kino_photo:%d", kinoID)
		mu.Unlock()
		bot.Send(tgbotapi.NewMessage(chatID, "🖼 Yangi rasmni yuboring:"))
		return

	case strings.HasPrefix(data, "delete_kino:"):
		kinoID, err := strconv.ParseInt(strings.TrimPrefix(data, "delete_kino:"), 10, 64)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ ID aniqlanmadi"))
			return
		}

		o := orm.NewOrm()

		var kino models.Kino
		if err := o.QueryTable(new(models.Kino)).Filter("Id", kinoID).One(&kino); err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Kino topilmadi yoki allaqachon o'chirilgan."))
			return
		}

		kinoName := kino.Name

		_, err = o.QueryTable(new(models.KinoPart)).Filter("Kino__Id", kinoID).Delete()
		if err != nil {
			log.Printf("KinoPart o'chirishda xatolik: %v", err)
		}

		_, err = o.Delete(&kino)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Kinoni o'chirishda xatolik yuz berdi."))
			return
		}

		log.Printf("🗑️ Kino o'chirildi! ID: %d, Nomi: %s", kinoID, kinoName)

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ \"%s\" kino butunlay o'chirildi!", kinoName))
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

	case strings.HasPrefix(data, "edit_kino_note:"):
		botID, err := strconv.ParseInt(strings.TrimPrefix(data, "edit_kino_note:"), 10, 64)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Noto‘g‘ri ID"))
			return
		}

		// Botni bazadan topamiz
		o := orm.NewOrm()
		var currentBot models.CreatedBot
		if err := o.QueryTable(new(models.CreatedBot)).Filter("Id", botID).One(&currentBot); err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Bot topilmadi"))
			return
		}

		// Faqat admin ishlata olishi uchun tekshirish (ixtiyoriy, lekin tavsiya etiladi)
		if !isAdmin(&currentBot, userID) {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Ruxsat yo‘q"))
			return
		}

		mu.Lock()
		adminState[userID] = "waiting_kino_note"
		mu.Unlock()

		bot.Send(tgbotapi.NewMessage(chatID, "✍️ Yangi kino osti matnini kiriting:\n\n(Bekor qilish uchun /cancel yuboring)"))
		return

	case strings.HasPrefix(data, "delete_kino_note:"):
		botID, err := strconv.ParseInt(strings.TrimPrefix(data, "delete_kino_note:"), 10, 64)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Noto‘g‘ri ID"))
			return
		}

		o := orm.NewOrm()
		var currentBot models.CreatedBot
		if err := o.QueryTable(new(models.CreatedBot)).Filter("Id", botID).One(&currentBot); err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Bot topilmadi"))
			return
		}

		if !isAdmin(&currentBot, userID) {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Ruxsat yo‘q"))
			return
		}

		_, err = o.QueryTable(new(models.CreatedBot)).
			Filter("Id", botID).
			Update(orm.Params{"Note": ""})

		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ O‘chirishda xatolik"))
			return
		}

		bot.Send(tgbotapi.NewMessage(chatID, "🗑 Kino osti matni o‘chirildi."))

		// Menyuni qayta chiqaramiz (endi bo‘sh holatda)
		showKinoNoteMenuPro(bot, &currentBot, chatID, userID)
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
		log.Printf("Noma'lum kino callback data: %s", data)
		msg := tgbotapi.NewMessage(chatID, "Noma'lum kino buyruq: "+data)
		bot.Send(msg)

	}
}

func handleKinoPartPro(bot *tgbotapi.BotAPI, cb *tgbotapi.CallbackQuery) {
	fmt.Println("===== handleKinoPartPro ISHGA TUSHDI =====")

	// Callback spinnerini o'chirish
	bot.Request(tgbotapi.NewCallback(cb.ID, ""))

	parts := strings.Split(cb.Data, ":")
	if len(parts) < 3 {
		fmt.Println("bad callback data:", cb.Data)
		return
	}

	kinoID, _ := strconv.ParseInt(parts[1], 10, 64)
	partOrder, _ := strconv.Atoi(parts[2])
	chatID := cb.Message.Chat.ID
	userID := cb.From.ID

	o := orm.NewOrm()

	var kinoPart models.KinoPart
	err := o.QueryTable(new(models.KinoPart)).
		Filter("Kino__Id", kinoID).
		Filter("PartOrder", partOrder).
		RelatedSel("Kino").
		One(&kinoPart)

	if err != nil {
		fmt.Println("PART ERROR:", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Qism topilmadi."))
		return
	}

	name := "Kino"
	if kinoPart.Kino != nil {
		name = kinoPart.Kino.Name
	}

	// 1. Defolt holatda saqlash va uzatish TAQIQLANADI
	protect := true
	var botNote string

	// 2. Kino va Bot ma'lumotlarini olish
	var kino models.Kino
	kerr := o.QueryTable(new(models.Kino)).
		Filter("Id", kinoID).
		RelatedSel("Bot").
		One(&kino)

	if kerr == nil && kino.Bot != nil {
		// FAQAT Admin uchun cheklov O'CHIRILADI
		if isAdmin(kino.Bot, userID) {
			protect = false
		}
		botNote = kino.Bot.Note
	} else {
		log.Printf("🔴 BOT VA KINO BOG'LANISHIDA XATOLIK: %v", kerr)
	}

	// Logging (Terminalda tekshirib olishingiz uchun)
	log.Printf("DEBUG: UserID: %d | IsProtect: %t | Kind: %s", userID, protect, kinoPart.Kind)

	// Caption tayyorlaymiz
	caption := fmt.Sprintf("%s - [%d-qism]", name, partOrder)
	if botNote != "" {
		caption += fmt.Sprintf("\n\n%s", botNote)
	}

	var sendErr error
	switch kinoPart.Kind {
	case "video":
		sendErr = sendProtectedVideoPro(bot, chatID, kinoPart.FileID, caption, protect)
	case "document":
		sendErr = sendProtectedDocumentPro(bot, chatID, kinoPart.FileID, caption, protect)
	case "photo":
		sendErr = sendProtectedPhotoPro(bot, chatID, kinoPart.FileID, caption, nil, protect)
	default:
		fmt.Println("UNKNOWN KIND:", kinoPart.Kind)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Noma'lum fayl turi: "+kinoPart.Kind))
		return
	}

	if sendErr != nil {
		fmt.Println(">>> SEND PART ERROR:", sendErr)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Faylni yuborishda xatolik yuz berdi."))
	}
}

func showTopKinoPro(bot *tgbotapi.BotAPI, b *models.CreatedBot, chatID int64) {
	o := orm.NewOrm()
	var kinos []models.Kino

	_, err := o.QueryTable(new(models.Kino)).
		Filter("Bot__Id", b.Id).
		OrderBy("-CreatedAt").
		Limit(10).
		All(&kinos)

	if err != nil || len(kinos) == 0 {
		sendUserBot(bot, chatID, "❌ Hozircha hech qanday kino topilmadi.")
		return
	}

	text := "<b>Eng so‘nggi 10 ta kino</b>\n\nKodni bosib tomosha qiling:"

	var rows [][]tgbotapi.InlineKeyboardButton

	for _, k := range kinos {
		name := k.Name
		if len([]rune(name)) > 22 {
			name = string([]rune(name)[:22]) + "..."
		}
		btnText := name
		if k.Year > 0 {
			btnText = fmt.Sprintf("[%d]  %s", k.Year, name)
		}

		// Endi callback: top_kino:BOTID:CODE
		btn := tgbotapi.NewInlineKeyboardButtonData(btnText, fmt.Sprintf("top_kino:%d:%s", b.Id, k.Code))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
	}
	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}
