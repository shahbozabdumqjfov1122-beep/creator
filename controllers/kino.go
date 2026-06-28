package controllers

import (
	"creator/models"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/beego/beego/v2/client/orm"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ==================== ASOSIY XABAR HANDLERI ====================

func HandleKinoBotMessage(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
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
			msg.Text == "➕ Kino joylash" ||
			msg.Text == "➕ Kanal qo‘shish" ||
			msg.Text == "➖ Kanal o‘chirish" ||
			msg.Text == "✏️ Kinoni tahrirlash" ||
			msg.Text == "🗑 Kinoni o‘chirish" ||
			msg.Text == "👥 Foydalanuvchilar" ||
			msg.Text == "📊 Statistika" ||
			msg.Text == "📢 Reklama" ||
			msg.Text == "👥 Hammaga" ||
			msg.Text == "⭐ VIP'larga" ||
			msg.Text == "👤 Oddiylarga" ||
			msg.Text == "👤 Adminlar" ||
			msg.Text == "➕ Admin qo'shish" ||
			msg.Text == "➖ Admin o'chirish" ||
			msg.Text == "📋 Adminlar ro'yxati" ||
			msg.Text == "⬅️ Orqaga" ||
			msg.Text == "/delkino" ||
			msg.Text == "/editkino"

	if isNewCommand && msg.Text != "/ok" && isAdmin(b, userID) {
		clearAnimeDraft(userID)
		mu.Lock()
		delete(adminState, userID)
		delete(adminTempChannel, userID)
		mu.Unlock()
	}
	if isAdmin(b, userID) {
		// 🎯 AGAR ADMIN DAVOMLI STATE ICHIDA BO'LSA
		mu.Lock()
		state, exists := adminState[userID]
		mu.Unlock()

		if exists {
			// 1. Kino yuklash holatlari bo'lsa
			if RouteKinoUploadState(bot, b, msg, state) {
				return
			}

			// 2. Kino tahrirlash (Edit) holatlari bo'lsa
			if RouteKinoEditState(bot, b, msg, state) {
				return
			}

			// 🎯 3. Foydalanuvchini boshqarish (VIP/Blok) holatlari bo'lsa
			if RouteUserManagementState(bot, b, msg, state) {
				return
			}

			// 4. Agar admin kanal qo'shish holatida bo'lsa
			if state == "wait_channel" || state == "wait_link" {
				HandleAdminCommands(bot, b, msg)
				return
			}
			// 5. Reklama yuborish holati bo'lsa
			if strings.HasPrefix(state, "waiting_broadcast_message:") {
				target := strings.TrimPrefix(state, "waiting_broadcast_message:")

				mu.Lock()
				delete(adminState, userID)
				mu.Unlock()

				RunBroadcast(bot, b, msg, target)
				return
			}
		}

		// Adminning asosiy menyu buyruqlari
		switch msg.Text {
		case "/admin":
			showKinoAdminPanel(bot, chatID)
			return

		case "➕ Kino joylash", "/addkino":
			StartKinoUpload(bot, b, msg)
			return

		case "➕ Kanal qo‘shish", "/addchannel":
			HandleAdminCommands(bot, b, msg)
			return

		case "➖ Kanal o‘chirish", "/delchannel":
			ShowChannelsToDelete(bot, b, chatID)
			return

		case "✏️ Kinoni tahrirlash", "/editkino":
			mu.Lock()
			adminState[userID] = "waiting_edit_kino_code"
			mu.Unlock()

			sendUserBot(bot, chatID, "🔍 *Tahrirlash qismi*\n\nO'zgartirmoqchi bo'lgan kino **kodini** yozib yuboring:")
			return

		case "🗑 Kinoni o‘chirish", "/delkino":
			mu.Lock()
			adminState[userID] = "waiting_delete_kino_code"
			mu.Unlock()

			sendUserBot(bot, chatID, "🗑 *Kinoni o'chirish*\n\nO'chirmoqchi bo'lgan kino **kodini** yozib yuboring:")
			return

		case "👥 Foydalanuvchilar":
			showUsersPanel(bot, chatID)
			return

		case "⭐ VIP qo'shish":
			startVipAdd(bot, chatID, userID)
			return

		case "🚫 VIP o'chirish":
			startVipRemove(bot, chatID, userID)
			return

		case "📋 VIP ro'yxati":
			showUserList(bot, chatID, b.Id, true, false)
			return

		case "📋 Blok ro'yxati":
			showUserList(bot, chatID, b.Id, false, true)
			return

		case "⛔ Blok qo'shish":
			startBlockAdd(bot, chatID, userID)
			return

		case "✅ Blok o'chirish":
			startBlockRemove(bot, chatID, userID)
			return

		case "⬅️ Orqaga":
			showKinoAdminPanel(bot, chatID)
			return

		case "📢 Reklama":
			startBroadcast(bot, chatID)
			return

		case "👥 Hammaga":
			mu.Lock()
			adminState[userID] = "waiting_broadcast_message:all"
			mu.Unlock()
			sendUserBot(bot, chatID, "✉️ Endi yubormoqchi bo'lgan xabarni jo'nating (matn, rasm, video yoki forward qilingan xabar):")
			return

		case "⭐ VIP'larga":
			mu.Lock()
			adminState[userID] = "waiting_broadcast_message:vip"
			mu.Unlock()
			sendUserBot(bot, chatID, "✉️ Endi yubormoqchi bo'lgan xabarni jo'nating (matn, rasm, video yoki forward qilingan xabar):")
			return

		case "👤 Oddiylarga":
			mu.Lock()
			adminState[userID] = "waiting_broadcast_message:regular"
			mu.Unlock()
			sendUserBot(bot, chatID, "✉️ Endi yubormoqchi bo'lgan xabarni jo'nating (matn, rasm, video yoki forward qilingan xabar):")
			return

		case "👤 Adminlar":
			showAdminsPanel(bot, chatID)
			return

		case "➕ Admin qo'shish":
			startAdminAdd(bot, chatID, userID)
			return

		case "➖ Admin o'chirish":
			startAdminRemove(bot, chatID, userID)
			return

		case "📋 Adminlar ro'yxati":
			showAdminsList(bot, b, chatID)
			return

		case "/ok":
			sendUserBot(bot, chatID, "💡 Hozirda faol yuklash jarayoni mavjud emas.")
			return

		case "📊 Statistika":
			showKinoStatistics(bot, b, chatID)
			return

		default:
			if strings.HasPrefix(msg.Text, "/") {
				sendUserBot(bot, chatID, "/admin")
				return
			}
			handleKinoByCode(bot, b, msg, msg.Text)
		}
	}

	if !botUser.IsVip && !CheckSubscription(bot, b, userID) {
		ShowMembership(bot, b, chatID)
		return
	}
	if strings.HasPrefix(msg.Text, "/start") {
		handleKinoStart(bot, b, msg)
		return
	}
	switch msg.Text {
	case "/start":
		handleKinoStart(bot, b, msg)
		return

	case "/help":
		sendUserBot(bot, chatID, "🎬 Kino kodini yozing...")
		return

	default:
		if strings.HasPrefix(msg.Text, "/") {
			sendUserBot(bot, chatID, "/admin")
			return
		}
		handleKinoByCode(bot, b, msg, msg.Text)
	}
}

// ==================== ADMIN PANEL (KINO) ====================

func showKinoAdminPanel(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📊 Statistika"),
			tgbotapi.NewKeyboardButton("👥 Foydalanuvchilar"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("➕ Kino joylash"),
			tgbotapi.NewKeyboardButton("🗑 Kinoni o‘chirish"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("➕ Kanal qo‘shish"),
			tgbotapi.NewKeyboardButton("➖ Kanal o‘chirish"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("✏️ Kinoni tahrirlash"),
			tgbotapi.NewKeyboardButton("📢 Reklama"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("👤 Adminlar"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "admin")
	msg.ReplyMarkup = keyboard
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}

// ==================== /start VA KOD ORQALI QIDIRISH ====================

func handleKinoStart(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
	args := strings.TrimSpace(strings.TrimPrefix(msg.Text, "/start"))

	if args != "" {
		handleKinoByCode(bot, b, msg, args)
		return
	}

	text := "🎬 *Kino Botga xush kelibsiz!*\n\nO‘zingizga yoqqan kino **kodini** yozib yuboring."
	sendUserBot(bot, msg.Chat.ID, text)
}

func handleKinoByCode(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message, code string) {
	fmt.Println("========== KINO SEARCH ==========")
	fmt.Println("BotID:", b.Id)
	fmt.Println("Code:", code)

	o := orm.NewOrm()
	var kino models.Kino

	err := o.QueryTable(new(models.Kino)).
		Filter("Bot__Id", b.Id).
		Filter("Code", strings.ToLower(strings.TrimSpace(code))).
		One(&kino)

	if err != nil {
		fmt.Println("KINO NOT FOUND:", err)

		text := "🔍 Afsuski, bunday kodli kino topilmadi...\n\n"

		respMsg := tgbotapi.NewMessage(msg.Chat.ID, text)
		respMsg.ParseMode = "Markdown"
		bot.Send(respMsg)
		return
	}

	fmt.Println("FOUND KINO:")
	fmt.Println("ID:", kino.Id)
	fmt.Println("Name:", kino.Name)
	fmt.Println("Code:", kino.Code)
	fmt.Println("Parts:", kino.PartsCount)

	caption := fmt.Sprintf(
		"%s\n\nJami qismlar - %d",
		kino.Name,
		kino.PartsCount,
	)

	photo := tgbotapi.NewPhoto(
		msg.Chat.ID,
		tgbotapi.FileID(kino.PhotoID),
	)
	photo.Caption = caption

	if kino.PartsCount > 0 {
		photo.ReplyMarkup = buildKinoPartsKeyboard(kino.Id, kino.PartsCount, 1)
	} else {
		photo.Caption += "\n⚠️ Tez orada qismlar joylanadi!"
	}

	_, sendErr := bot.Send(photo)
	if sendErr != nil {
		fmt.Println("SEND PHOTO ERROR:", sendErr)
	}
}

// ==================== INLINE KLAVIATURA (QISMLAR) ====================

func buildKinoPartsKeyboard(kinoID int64, totalParts int, page int) tgbotapi.InlineKeyboardMarkup {
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

// ==================== CALLBACK HANDLER (KINO) ====================

func HandleKinoCallback(bot *tgbotapi.BotAPI, cb *tgbotapi.CallbackQuery) {
	if cb.Message == nil {
		return
	}
	data := cb.Data
	chatID := cb.Message.Chat.ID
	userID := cb.From.ID

	bot.Request(tgbotapi.NewCallback(cb.ID, ""))

	log.Printf("Kino callback keldi: %s (Chat ID: %d)", data, chatID)

	switch {
	case strings.HasPrefix(data, "kino_page:"):
		handleKinoPage(bot, cb)

	case strings.HasPrefix(data, "kino_part:"):
		handleKinoPart(bot, cb)

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

	default:
		log.Printf("Noma'lum kino callback data: %s", data)
		msg := tgbotapi.NewMessage(chatID, "Noma'lum kino buyruq: "+data)
		bot.Send(msg)
	}
}

func handleKinoPage(bot *tgbotapi.BotAPI, cb *tgbotapi.CallbackQuery) {
	parts := strings.Split(cb.Data, ":")

	kinoID, _ := strconv.ParseInt(parts[1], 10, 64)
	page, _ := strconv.Atoi(parts[2])

	o := orm.NewOrm()

	var kino models.Kino
	kino.Id = kinoID

	if err := o.Read(&kino); err != nil {
		return
	}

	kb := buildKinoPartsKeyboard(
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

func handleKinoPart(bot *tgbotapi.BotAPI, cb *tgbotapi.CallbackQuery) {
	parts := strings.Split(cb.Data, ":")

	kinoID, _ := strconv.ParseInt(parts[1], 10, 64)
	partOrder, _ := strconv.Atoi(parts[2])

	o := orm.NewOrm()

	var kinoPart models.KinoPart
	err := o.QueryTable(new(models.KinoPart)).
		Filter("Kino__Id", kinoID).
		Filter("PartOrder", partOrder).
		RelatedSel("Kino").
		One(&kinoPart)

	if err != nil {
		bot.Send(tgbotapi.NewMessage(
			cb.Message.Chat.ID,
			"❌ Qism topilmadi.",
		))
		return
	}

	captionText := fmt.Sprintf(
		"%s\n"+
			"%d-qism",
		kinoPart.Kino.Name,
		kinoPart.PartOrder,
	)

	switch kinoPart.Kind {

	case "video":
		video := tgbotapi.NewVideo(
			cb.Message.Chat.ID,
			tgbotapi.FileID(kinoPart.FileID),
		)
		video.Caption = captionText
		video.ParseMode = "Markdown"
		bot.Send(video)

	case "document":
		doc := tgbotapi.NewDocument(
			cb.Message.Chat.ID,
			tgbotapi.FileID(kinoPart.FileID),
		)
		doc.Caption = captionText
		doc.ParseMode = "Markdown"
		bot.Send(doc)

	case "photo":
		photo := tgbotapi.NewPhoto(
			cb.Message.Chat.ID,
			tgbotapi.FileID(kinoPart.FileID),
		)
		photo.Caption = captionText
		photo.ParseMode = "Markdown"
		bot.Send(photo)
	}
}

// ==================== TAHRIRLASH HOLATLARI ====================

func RouteKinoEditState(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message, state string) bool {
	userID := msg.From.ID
	chatID := msg.Chat.ID
	o := orm.NewOrm()

	// ==================== KINONI TAHRIRLASH (kod orqali topish) ====================
	if state == "waiting_edit_kino_code" {
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
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🗑 Butunlay o'chirish", fmt.Sprintf("delete_kino:%d", kino.Id)),
			),
		)

		reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Kino topildi:\n\n📌 *%s*\n🔑 Kod: `%s`\n🎬 Qismlar: %d\n\nNima qilmoqchisiz?",
			kino.Name, kino.Code, kino.PartsCount))
		reply.ParseMode = "Markdown"
		reply.ReplyMarkup = keyboard
		bot.Send(reply)

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()
		return true
	}

	// ==================== KINONI BUTUNLAY O'CHIRISH (KOD ORQALI) ====================
	if state == "waiting_delete_kino_code" {
		code := strings.ToLower(strings.TrimSpace(msg.Text))
		var kino models.Kino
		err := o.QueryTable(new(models.Kino)).
			Filter("Bot__Id", b.Id).
			Filter("Code", code).
			One(&kino)

		if err != nil {
			sendUserBot(bot, chatID, "❌ Bunday kodli kino topilmadi.\nQaytadan kiriting:")
			return true
		}

		kinoName := kino.Name

		_, err = o.QueryTable(new(models.KinoPart)).Filter("Kino__Id", kino.Id).Delete()
		if err != nil {
			log.Printf("KinoPart o'chirishda xatolik: %v", err)
		}

		_, err = o.Delete(&kino)
		if err != nil {
			sendUserBot(bot, chatID, "❌ Kinoni o'chirishda xatolik yuz berdi.")
			return true
		}

		log.Printf("🗑️ Kino o'chirildi! ID: %d, Nomi: %s", kino.Id, kinoName)

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()

		sendUserBot(bot, chatID, fmt.Sprintf("✅ \"%s\" kino butunlay o'chirildi!", kinoName))
		showKinoAdminPanel(bot, chatID)
		return true
	}

	// --- 1. KODNI O'ZGARTIRISH ---
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
		showKinoAdminPanel(bot, chatID)
		return true
	}

	// --- 2. NOMINI O'ZGARTIRISH ---
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
		showKinoAdminPanel(bot, chatID)
		return true
	}

	// --- 3. QISM QO'SHISH ---
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
			sendUserBot(bot, chatID, "❌ Iltimos, faqat video, rasm yoki hujjat yuboring:")
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
			showKinoAdminPanel(bot, chatID)
		}
		return true
	}

	// --- 4. QISM O'CHIRISH ---
	if strings.HasPrefix(state, "waiting_del_kino_part_num:") {
		kinoID, _ := strconv.ParseInt(strings.TrimPrefix(state, "waiting_del_kino_part_num:"), 10, 64)
		partNum, err := strconv.Atoi(strings.TrimSpace(msg.Text))

		if err != nil || partNum <= 0 {
			sendUserBot(bot, chatID, "❌ Noto'g'ri qism raqami. Faqat musbat raqam kiriting:")
			return true
		}

		var kino models.Kino
		if o.QueryTable(new(models.Kino)).Filter("Id", kinoID).Filter("Bot__Id", b.Id).One(&kino) != nil {
			sendUserBot(bot, chatID, "❌ Kino topilmadi.")
			return true
		}

		num, _ := o.QueryTable(new(models.KinoPart)).
			Filter("Kino__Id", kinoID).
			Filter("PartOrder", partNum).
			Delete()

		if num > 0 {
			if kino.PartsCount > 0 {
				kino.PartsCount -= 1
				o.Update(&kino, "PartsCount")
			}

			mu.Lock()
			delete(adminState, userID)
			mu.Unlock()

			sendUserBot(bot, chatID, fmt.Sprintf("✅ %d-qism muvaffaqiyatli o'chirildi!", partNum))
			showKinoAdminPanel(bot, chatID)
		} else {
			sendUserBot(bot, chatID, "❌ Bunday raqamli qism topilmadi. Qaytadan urinib ko'ring:")
		}
		return true
	}

	// --- 5. SURATNI O'ZGARTIRISH ---
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
		showKinoAdminPanel(bot, chatID)
		return true
	}

	return false
}

// ==================== KINO YUKLASH (YANGI KINO QO'SHISH) ====================
// Eslatma: StartKinoUpload va RouteKinoUploadState funksiyalari sizning
// StartAnimeUpload / RouteAnimeUploadState funksiyalaringizga mos ravishda
// yozilishi kerak. O'sha ikki funksiyaning to'liq kodini yuborsangiz,
// men ularni ham aynan shu faylga Kino versiyasida qo'shib beraman.

// ==================== STATISTIKA (KINO) ====================

func showKinoStatistics(bot *tgbotapi.BotAPI, b *models.CreatedBot, chatID int64) {
	o := orm.NewOrm()

	totalKino, _ := o.QueryTable(new(models.Kino)).
		Filter("Bot__Id", b.Id).
		Count()

	// Qolgan statistik ko'rsatkichlar (foydalanuvchilar, VIP, blok, faollik)
	// umumiy showStatistics funksiyasi bilan bir xil, shuning uchun shu yerda
	// faqat "Jami kino" sonini almashtirib, qolganini umumiy funksiyaga
	// topshirish mumkin. Hozircha mustaqil versiya sifatida qoldiramiz:

	text := fmt.Sprintf(
		"📊 *Bot Statistikasi*\n\n🎬 Jami kino: `%d`\n\n(Qolgan statistika uchun umumiy showStatistics chaqirilmoqda)\n",
		totalKino,
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	bot.Send(msg)

	// To'liq statistika (foydalanuvchilar, VIP, blok va h.k.) uchun
	// umumiy showStatistics funksiyasini ham chaqiramiz:
	showStatistics(bot, b, chatID)
}
