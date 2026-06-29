package controllers

import (
	"creator/models"
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"log"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func HandleAnimeBotMessage(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
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
			msg.Text == "➕ Anime joylash" ||
			msg.Text == "➕ Kanal qo‘shish" ||
			msg.Text == "➖ Kanal o‘chirish" ||
			msg.Text == "✏️ Animeni tahrirlash" ||
			msg.Text == "🗑 Animeni o‘chirish" ||
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
			msg.Text == "📋 Blok ro'yxati" ||
			msg.Text == "📋 VIP ro'yxati" ||
			msg.Text == "⬅️ Orqaga" ||
			msg.Text == "/delanime" ||
			msg.Text == "/editanime"

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
			// 1. Anime yuklash holatlari bo'lsa
			if RouteAnimeUploadState(bot, b, msg, state) {
				return
			}

			// 2. Anime tahrirlash (Edit) holatlari bo'lsa
			if RouteAnimeEditState(bot, b, msg, state) {
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
		// --- ADMIN QO'SHISH ---
		// --- ADMIN QO'SHISH ---

		// Adminning asosiy menyu buyruqlari
		switch msg.Text {
		case "/admin":
			showAdminPanel(bot, chatID)
			return

		case "➕ Anime joylash", "/addanime":
			StartAnimeUpload(bot, b, msg)
			return

		case "➕ Kanal qo‘shish", "/addchannel":
			HandleAdminCommands(bot, b, msg)
			return

		case "➖ Kanal o‘chirish", "/delchannel":
			ShowChannelsToDelete(bot, b, chatID)
			return

		case "✏️ Animeni tahrirlash", "/editanime":
			mu.Lock()
			adminState[userID] = "waiting_edit_code"
			mu.Unlock()

			sendUserBot(bot, chatID, "🔍 *Tahrirlash qismi*\n\nO'zgartirmoqchi bo'lgan anime **kodini** yozib yuboring:")
			return

		case "🗑 Animeni o‘chirish", "/delanime":
			mu.Lock()
			adminState[userID] = "waiting_delete_code"
			mu.Unlock()

			sendUserBot(bot, chatID, "🗑 *Animeni o'chirish*\n\nO'chirmoqchi bo'lgan anime **kodini** yozib yuboring:")
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
			showAdminPanel(bot, chatID)
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
			showStatistics(bot, b, chatID)
			return

		default:
			if strings.HasPrefix(msg.Text, "/") {
				sendUserBot(bot, chatID, "/admin")
				return
			}
			handleAnimeByCode(bot, b, msg, msg.Text)
		}
	}

	if !botUser.IsVip && !CheckSubscription(bot, b, userID) {
		ShowMembership(bot, b, chatID)
		return
	}
	if strings.HasPrefix(msg.Text, "/start") {
		handleAnimeStart(bot, b, msg)
		return
	}
	switch msg.Text {
	case "/start":
		handleAnimeStart(bot, b, msg)
		return

	case "/help":
		sendUserBot(bot, chatID, "🎌 Anime kodini yozing...")
		return

	default:
		if strings.HasPrefix(msg.Text, "/") {
			sendUserBot(bot, chatID, "/admin")
			return
		}
		// Anime kodi orqali qidirish
		handleAnimeByCode(bot, b, msg, msg.Text)
	}
}

func showAdminPanel(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📊 Statistika"),
			tgbotapi.NewKeyboardButton("👥 Foydalanuvchilar"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("➕ Anime joylash"),
			tgbotapi.NewKeyboardButton("🗑 Animeni o‘chirish"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("➕ Kanal qo‘shish"),
			tgbotapi.NewKeyboardButton("➖ Kanal o‘chirish"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("✏️ Animeni tahrirlash"),
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

func showUsersPanel(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⭐ VIP qo'shish"),
			tgbotapi.NewKeyboardButton("🚫 VIP o'chirish"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📋 VIP ro'yxati"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⛔ Blok qo'shish"),
			tgbotapi.NewKeyboardButton("✅ Blok o'chirish"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📋 Blok ro'yxati"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⬅️ Orqaga"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "👥 *Foydalanuvchilarni boshqarish*\n\nKerakli amalni tanlang:")
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

func startBroadcast(bot *tgbotapi.BotAPI, chatID int64) {
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

	msg := tgbotapi.NewMessage(chatID, "📢 *Reklama yuborish*\n\nKimlarga yuborilsin?")
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

func showAdminsPanel(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(
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

	msg := tgbotapi.NewMessage(chatID, "👤 *Adminlarni boshqarish*\n\nKerakli amalni tanlang:")
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

func handleAnimeStart(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
	// msg.Text ichidan "/start " qismini olib tashlaymiz va parametr borligini tekshiramiz
	args := strings.TrimSpace(strings.TrimPrefix(msg.Text, "/start"))

	if args != "" {
		// Agar /start buyrug'idan keyin kod kelgan bo'lsa (masalan: /start 123)
		// Uni to'g'ridan-to'g'ri anime qidirish funksiyasiga yuboramiz
		handleAnimeByCode(bot, b, msg, args)
		return
	}

	// Agar shunchaki toza /start bosilgan bo'lsa, oddiy xush kelibsiz matni chiqadi
	text := "🎌 *Anime Botga xush kelibsiz!*\n\nO‘zingizga yoqqan anime **kodini** yozib yuboring."
	sendUserBot(bot, msg.Chat.ID, text)
}

func handleAnimeByCode(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message, code string) {

	fmt.Println("========== ANIME SEARCH ==========")
	fmt.Println("BotID:", b.Id)
	fmt.Println("Code:", code)

	o := orm.NewOrm()
	var anime models.Anime

	err := o.QueryTable(new(models.Anime)).
		Filter("Bot__Id", b.Id).
		Filter("Code", strings.ToLower(strings.TrimSpace(code))).
		One(&anime)

	if err != nil {
		fmt.Println("ANIME NOT FOUND:", err)

		text := "🔍 Afsuski, bunday kodli anime topilmadi...\n\n"

		msg := tgbotapi.NewMessage(msg.Chat.ID, text)
		msg.ParseMode = "Markdown"
		bot.Send(msg)
		return
	}

	fmt.Println("FOUND ANIME:")
	fmt.Println("ID:", anime.Id)
	fmt.Println("Name:", anime.Name)
	fmt.Println("Code:", anime.Code)
	fmt.Println("Parts:", anime.PartsCount)

	caption := fmt.Sprintf(
		"%s\n\nJami qismlar - %d",
		anime.Name,
		anime.PartsCount,
	)

	photo := tgbotapi.NewPhoto(
		msg.Chat.ID,
		tgbotapi.FileID(anime.PhotoID),
	)
	photo.Caption = caption

	// FIX: Faqat qismlar soni 0 dan katta bo'lsagina klaviatura qo'shamiz
	if anime.PartsCount > 0 {
		photo.ReplyMarkup = buildAnimePartsKeyboard(anime.Id, anime.PartsCount, 1)
	} else {
		// Qismlar yo'qligi haqida ogohlantirish matni qo'shish ham mumkin
		photo.Caption += "\n⚠️ Tez orada qismlar joylanadi!"
	}

	_, sendErr := bot.Send(photo)
	if sendErr != nil {
		fmt.Println("SEND PHOTO ERROR:", sendErr)
	}
}

func buildAnimePartsKeyboard(animeID int64, totalParts int, page int) tgbotapi.InlineKeyboardMarkup {
	// Agar qismlar bo'lmasa, bo'sh klaviatura obyektini qaytaramiz (lekin yuqoridagi tekshiruv uni tgbotapi'ga ketishini to'sadi)
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

func HandleAnimeCallback(bot *tgbotapi.BotAPI, cb *tgbotapi.CallbackQuery) {
	if cb.Message == nil {
		return
	}
	data := cb.Data
	chatID := cb.Message.Chat.ID
	userID := cb.From.ID

	bot.Request(tgbotapi.NewCallback(cb.ID, ""))

	log.Printf("Anime callback keldi: %s (Chat ID: %d)", data, chatID)

	switch {
	case strings.HasPrefix(data, "anime_page:"):
		handleAnimePage(bot, cb)

	case strings.HasPrefix(data, "anime_part:"):
		handleAnimePart(bot, cb)

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

	default:
		log.Printf("Noma'lum anime callback data: %s", data)
		msg := tgbotapi.NewMessage(chatID, "Noma'lum anime buyruq: "+data)
		bot.Send(msg)

	}
}

func handleAnimePage(bot *tgbotapi.BotAPI, cb *tgbotapi.CallbackQuery) {

	parts := strings.Split(cb.Data, ":")

	animeID, _ := strconv.ParseInt(parts[1], 10, 64)
	page, _ := strconv.Atoi(parts[2])

	o := orm.NewOrm()

	var anime models.Anime
	anime.Id = animeID

	if err := o.Read(&anime); err != nil {
		return
	}

	kb := buildAnimePartsKeyboard(
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

func handleAnimePart(bot *tgbotapi.BotAPI, cb *tgbotapi.CallbackQuery) {
	parts := strings.Split(cb.Data, ":")

	animeID, _ := strconv.ParseInt(parts[1], 10, 64)
	partOrder, _ := strconv.Atoi(parts[2])

	o := orm.NewOrm()

	// 1. 🎯 FIX: AnimePart bilan birga uning Anime ma'lumotlarini ham bazadan o'qiymiz (RelatedSel orqali)
	var animePart models.AnimePart
	err := o.QueryTable(new(models.AnimePart)).
		Filter("Anime__Id", animeID).
		Filter("PartOrder", partOrder).
		RelatedSel("Anime"). // Anime nomini olish uchun buni qo'shish shart!
		One(&animePart)

	if err != nil {
		bot.Send(tgbotapi.NewMessage(
			cb.Message.Chat.ID,
			"❌ Qism topilmadi.",
		))
		return
	}

	// 2. 🎯 Chiroyli matn tayyorlaymiz
	captionText := fmt.Sprintf(
		"%s\n"+
			"%d-qism",
		animePart.Anime.Name, // Anime nomi
		animePart.PartOrder,  // Nechanchi qismligi
	)

	// 3. Fayl turiga qarab Caption qo'shib jo'natamiz
	switch animePart.Kind {

	case "video":
		video := tgbotapi.NewVideo(
			cb.Message.Chat.ID,
			tgbotapi.FileID(animePart.FileID),
		)
		video.Caption = captionText  // 🎯 Izoh qo'shildi
		video.ParseMode = "Markdown" // Matn qalin (bold) chiqishi uchun
		bot.Send(video)

	case "document":
		doc := tgbotapi.NewDocument(
			cb.Message.Chat.ID,
			tgbotapi.FileID(animePart.FileID),
		)
		doc.Caption = captionText // 🎯 Izoh qo'shildi
		doc.ParseMode = "Markdown"
		bot.Send(doc)

	case "photo":
		photo := tgbotapi.NewPhoto(
			cb.Message.Chat.ID,
			tgbotapi.FileID(animePart.FileID),
		)
		photo.Caption = captionText // 🎯 Izoh qo'shildi
		photo.ParseMode = "Markdown"
		bot.Send(photo)
	}
}

func RouteAnimeEditState(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message, state string) bool {
	userID := msg.From.ID
	chatID := msg.Chat.ID
	o := orm.NewOrm()

	// ==================== ANIMENI TAHRIRLASH (kod orqali topish) ====================
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
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🗑 Butunlay o'chirish", fmt.Sprintf("delete_anime:%d", anime.Id)),
			),
		)

		reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Anime topildi:\n\n📌 *%s*\n🔑 Kod: `%s`\n🎬 Qismlar: %d\n\nNima qilmoqchisiz?",
			anime.Name, anime.Code, anime.PartsCount))
		reply.ParseMode = "Markdown"
		reply.ReplyMarkup = keyboard
		bot.Send(reply)

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()
		return true
	}

	// ==================== ANIMENI BUTUNLAY O'CHIRISH (KOD ORQALI) ====================
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

		// Avval bog'liq qismlarni o'chiramiz
		_, err = o.QueryTable(new(models.AnimePart)).Filter("Anime__Id", anime.Id).Delete()
		if err != nil {
			log.Printf("AnimePart o'chirishda xatolik: %v", err)
		}

		// Keyin animeni o'chiramiz
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
		showAdminPanel(bot, chatID)
		return true
	}
	// --- 1. KODNI O'ZGARTIRISH ---
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
		showAdminPanel(bot, chatID)
		return true
	}

	// --- 2. NOMINI O'ZGARTIRISH ---
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
		showAdminPanel(bot, chatID)
		return true
	}

	// --- 3. QISM QO'SHISH ---
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
			sendUserBot(bot, chatID, "❌ Iltimos, faqat video, rasm yoki hujjat yuboring:")
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
			showAdminPanel(bot, chatID)
		}
		return true
	}

	// --- 4. QISM O'CHIRISH ---
	if strings.HasPrefix(state, "waiting_del_part_num:") {
		animeID, _ := strconv.ParseInt(strings.TrimPrefix(state, "waiting_del_part_num:"), 10, 64)
		partNum, err := strconv.Atoi(strings.TrimSpace(msg.Text))

		if err != nil || partNum <= 0 {
			sendUserBot(bot, chatID, "❌ Noto'g'ri qism raqami. Faqat musbat raqam kiriting:")
			return true
		}

		var anime models.Anime
		if o.QueryTable(new(models.Anime)).Filter("Id", animeID).Filter("Bot__Id", b.Id).One(&anime) != nil {
			sendUserBot(bot, chatID, "❌ Anime topilmadi.")
			return true
		}

		num, _ := o.QueryTable(new(models.AnimePart)).
			Filter("Anime__Id", animeID).
			Filter("PartOrder", partNum).
			Delete()

		if num > 0 {
			if anime.PartsCount > 0 {
				anime.PartsCount -= 1
				o.Update(&anime, "PartsCount")
			}

			mu.Lock()
			delete(adminState, userID)
			mu.Unlock()

			sendUserBot(bot, chatID, fmt.Sprintf("✅ %d-qism muvaffaqiyatli o'chirildi!", partNum))
			showAdminPanel(bot, chatID)
		} else {
			sendUserBot(bot, chatID, "❌ Bunday raqamli qism topilmadi. Qaytadan urinib ko'ring:")
		}
		return true
	}

	// --- 5. SURATNI O'ZGARTIRISH ---
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
		showAdminPanel(bot, chatID)
		return true
	}

	return false
}

func showUserList(bot *tgbotapi.BotAPI, chatID int64, botID int64, vipOnly bool, blockedOnly bool) {
	o := orm.NewOrm()

	// BotUser modeliga to'g'ri so'rov yuborish
	qs := o.QueryTable(new(models.BotUser)).Filter("Bot__Id", botID)

	if vipOnly {
		qs = qs.Filter("IsVip", true)
	}
	if blockedOnly {
		qs = qs.Filter("IsBlocked", true)
	}

	var users []models.BotUser
	_, err := qs.All(&users)

	// Agar xatolik bo'lsa yoki baza chindan ham bo'sh bo'lsa
	if err != nil {
		log.Printf("showUserList xatolik: %v", err)
		sendUserBot(bot, chatID, "❌ Ro'yxatni yuklashda texnik xatolik yuz berdi.")
		return
	}

	if len(users) == 0 {
		sendUserBot(bot, chatID, "📭 Ro'yxat hozircha bo'sh.")
		return
	}

	// Sarlavha qismini chiroyli qilamiz
	title := "📋 *Foydalanuvchilar ro'yxati:*"
	if vipOnly {
		title = "⭐ *VIP foydalanuvchilar ro'yxati:*"
	} else if blockedOnly {
		title = "🚫 *Bloklanganlar ro'yxati:*"
	}

	text := title + "\n\n"
	for i, u := range users {
		uname := u.Username
		if uname == "" {
			uname = "noma'lum"
		} else {
			// Markdown buzilib ketmasligi uchun username ichidagi '_' belgisini qochiramiz (escape)
			uname = strings.ReplaceAll(uname, "_", "\\_")
		}

		text += fmt.Sprintf("%d. ID: `%d` — @%s\n", i+1, u.TgId, uname)
	}

	// Xabarni yuborish
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"

	_, sendErr := bot.Send(msg)
	if sendErr != nil {
		log.Printf("Ro'yxatni yuborishda Telegram xatoligi (ehtimol Markdown parse error): %v", sendErr)
		// Agar Markdown sababli o'xshamaslik ehtimoli bo'lsa, oddiy matn sifatida qayta urinamiz
		msg.ParseMode = ""
		bot.Send(msg)
	}
}

func startVipAdd(bot *tgbotapi.BotAPI, chatID int64, userID int64) {
	mu.Lock()
	adminState[userID] = "waiting_vip_add"
	mu.Unlock()
	sendUserBot(bot, chatID, "⭐ VIP qilmoqchi bo'lgan foydalanuvchining Telegram ID raqamini yuboring:")
}

func startVipRemove(bot *tgbotapi.BotAPI, chatID int64, userID int64) {
	mu.Lock()
	adminState[userID] = "waiting_vip_remove"
	mu.Unlock()
	sendUserBot(bot, chatID, "🚫 VIP'dan chiqarmoqchi bo'lgan foydalanuvchining Telegram ID raqamini yuboring:")
}

func startBlockAdd(bot *tgbotapi.BotAPI, chatID int64, userID int64) {
	mu.Lock()
	adminState[userID] = "waiting_block_add"
	mu.Unlock()
	sendUserBot(bot, chatID, "⛔ Bloklamoqchi bo'lgan foydalanuvchining Telegram ID raqamini yuboring:")
}

func startBlockRemove(bot *tgbotapi.BotAPI, chatID int64, userID int64) {
	mu.Lock()
	adminState[userID] = "waiting_block_remove"
	mu.Unlock()
	sendUserBot(bot, chatID, "✅ Blokdan chiqarmoqchi bo'lgan foydalanuvchining Telegram ID raqamini yuboring:")
}

func RouteUserManagementState(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message, state string) bool {
	userID := msg.From.ID
	chatID := msg.Chat.ID
	o := orm.NewOrm()

	// --- ADMIN QO'SHISH ---
	if state == "waiting_admin_add" {
		tgID, err := strconv.ParseInt(strings.TrimSpace(msg.Text), 10, 64)
		if err != nil {
			sendUserBot(bot, chatID, "❌ Noto'g'ri ID. Faqat raqam kiriting:")
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
		showAdminsPanel(bot, chatID)
		return true

	}

	// --- ADMIN O'CHIRISH ---
	if state == "waiting_admin_remove" {
		tgID, err := strconv.ParseInt(strings.TrimSpace(msg.Text), 10, 64)
		if err != nil {
			sendUserBot(bot, chatID, "❌ Noto'g'ri ID. Faqat raqam kiriting:")
			return true
		}

		// 🎯 Owner'ni o'chirishga ruxsat yo'q
		if isOwner(b, tgID) {
			sendUserBot(bot, chatID, "❌ Botning asl egasini adminlikdan chiqarib bo'lmaydi.")
			mu.Lock()
			delete(adminState, userID)
			mu.Unlock()
			showAdminsPanel(bot, chatID)
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
		showAdminsPanel(bot, chatID)
		return true
	}

	// --- VIP QO'SHISH ---
	if state == "waiting_vip_add" {
		tgID, err := strconv.ParseInt(strings.TrimSpace(msg.Text), 10, 64)
		if err != nil {
			sendUserBot(bot, chatID, "❌ Noto'g'ri ID. Faqat raqam kiriting:")
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
			bu.IsVip = true
			o.Update(&bu, "IsVip")
			sendUserBot(bot, chatID, fmt.Sprintf("✅ ID %d VIP qilindi!", tgID))
		}

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()
		showUsersPanel(bot, chatID)
		return true
	}

	// --- VIP O'CHIRISH ---
	if state == "waiting_vip_remove" {
		tgID, err := strconv.ParseInt(strings.TrimSpace(msg.Text), 10, 64)
		if err != nil {
			sendUserBot(bot, chatID, "❌ Noto'g'ri ID. Faqat raqam kiriting:")
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
		showUsersPanel(bot, chatID)
		return true
	}

	// --- BLOK QO'SHISH ---
	if state == "waiting_block_add" {
		tgID, err := strconv.ParseInt(strings.TrimSpace(msg.Text), 10, 64)
		if err != nil {
			sendUserBot(bot, chatID, "❌ Noto'g'ri ID. Faqat raqam kiriting:")
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
		showUsersPanel(bot, chatID)
		return true
	}

	// --- BLOK O'CHIRISH ---
	if state == "waiting_block_remove" {
		tgID, err := strconv.ParseInt(strings.TrimSpace(msg.Text), 10, 64)
		if err != nil {
			sendUserBot(bot, chatID, "❌ Noto'g'ri ID. Faqat raqam kiriting:")
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
		showUsersPanel(bot, chatID)
		return true
	}

	return false
}

func showStatistics(bot *tgbotapi.BotAPI, b *models.CreatedBot, chatID int64) {
	o := orm.NewOrm()
	now := time.Now()

	// ---- Animelar ----
	totalAnime, _ := o.QueryTable(new(models.Anime)).
		Filter("Bot__Id", b.Id).
		Count()

	// ---- Foydalanuvchilar (jami) ----
	totalUsers, _ := o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", b.Id).
		Count()

	// ---- VIP / Blok ----
	vipCount, _ := o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", b.Id).
		Filter("IsVip", true).
		Count()

	blockedCount, _ := o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", b.Id).
		Filter("IsBlocked", true).
		Count()

	// ---- Yangi qo'shilganlar (JoinedAt bo'yicha) ----
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

	// ---- Aktiv foydalanuvchilar (UpdatedAt bo'yicha) ----
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
		"📊 *Bot Statistikasi*\n\n"+
			"🎬 Jami anime/kino: `%d`\n"+
			"👥 Jami user: `%d`\n"+
			"⭐ VIP user: `%d`\n"+
			"⛔ Ban user: `%d`\n\n"+
			"🆕 *Qo'shilgan foydalanuvchilar*\n"+
			"├ 📅 1 kun: `%d`\n"+
			"├ 📆 7 kun: `%d`\n"+
			"├ 🗓 30 kun: `%d`\n"+
			"├ 📈 60 kun: `%d`\n"+
			"└ 🚀 90 kun: `%d`\n\n"+
			"🟢 *Faol foydalanuvchilar*\n"+
			"├ ⚡ 1 kun: `%d`\n"+
			"├ 🔥 7 kun: `%d`\n"+
			"├ 💎 30 kun: `%d`\n"+
			"├ 📊 60 kun: `%d`\n"+
			"└ 🏆 90 kun: `%d`\n",
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

func RunBroadcast(bot *tgbotapi.BotAPI, b *models.CreatedBot, srcMsg *tgbotapi.Message, target string) {
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

		// Telegram limitiga urilmaslik uchun kichik kechikish
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

	showAdminPanel(bot, chatID)
}

func startAdminAdd(bot *tgbotapi.BotAPI, chatID int64, userID int64) {
	mu.Lock()
	adminState[userID] = "waiting_admin_add"
	mu.Unlock()
	sendUserBot(bot, chatID, "➕ Admin qilmoqchi bo'lgan foydalanuvchining Telegram ID raqamini yuboring:")
}

func startAdminRemove(bot *tgbotapi.BotAPI, chatID int64, userID int64) {
	mu.Lock()
	adminState[userID] = "waiting_admin_remove"
	mu.Unlock()
	sendUserBot(bot, chatID, "➖ Adminlikdan chiqarmoqchi bo'lgan foydalanuvchining Telegram ID raqamini yuboring:")
}

func showAdminsList(bot *tgbotapi.BotAPI, b *models.CreatedBot, chatID int64) {
	o := orm.NewOrm()

	var owner models.CreatedBot
	err := o.QueryTable(new(models.CreatedBot)).
		Filter("Id", b.Id).
		RelatedSel("Owner").
		One(&owner)

	text := "👤 Adminlar ro'yxati:\n\n"

	if err == nil && owner.Owner != nil {
		text += fmt.Sprintf("👑 Owner — ID: %d\n", owner.Owner.TgId)
	}

	var admins []models.BotUser
	_, err = o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", b.Id).
		Filter("IsAdmin", true).
		All(&admins)

	if err != nil || len(admins) == 0 {
		text += "\n📭 Qo'shimcha adminlar yo'q."
	} else {
		for i, a := range admins {
			uname := a.Username
			if uname == "" {
				uname = "noma'lum"
			}
			text += fmt.Sprintf("%d. ID: %d — @%s\n", i+1, a.TgId, uname)
		}
	}

	msg := tgbotapi.NewMessage(chatID, text)
	// ParseMode olib tashlandi — username'lardagi "_" kabi belgilar Markdown'ni buzib, xabarni butunlay yuborilmay qoldirardi
	_, sendErr := bot.Send(msg)
	if sendErr != nil {
		log.Printf("🔴 ADMIN LIST: xabar yuborishda xatolik: %v", sendErr)
	}
}
