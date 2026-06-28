package controllers

import (
	"creator/services"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"creator/models"

	"github.com/beego/beego/v2/client/orm"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var CreatorBot *tgbotapi.BotAPI

type RangliTugma struct {
	Text              string `json:"text"`
	CallbackData      string `json:"callback_data,omitempty"`
	URL               string `json:"url,omitempty"`
	Style             string `json:"style,omitempty"`
	IconCustomEmojiID string `json:"icon_custom_emoji_id,omitempty"`
}

type RangliKlaviatura struct {
	InlineKeyboard [][]RangliTugma `json:"inline_keyboard"`
}

var mu sync.RWMutex
var userState = make(map[int64]string)
var userBotType = make(map[int64]string)
var userTokenChangeBotID = make(map[int64]int64)
var HardcodedBotTypes = []struct {
	Name string
	Code string
}{
	{Name: "Anime Bot", Code: "anime"},
	{Name: "Kino Bot", Code: "kino"},
}

func InitCreatorBot(token string) error {
	var err error

	CreatorBot, err = tgbotapi.NewBotAPI(token)
	if err != nil {
		return err
	}

	log.Printf("✅ Creator bot muvaffaqiyatli ishga tushdi: @%s", CreatorBot.Self.UserName)
	go listenUpdates()
	return nil
}

func listenUpdates() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := CreatorBot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			handleMessage(update.Message)
		} else if update.CallbackQuery != nil {
			// FIX: CreatorBot argument sifatida qo'shildi
			handleCallback(CreatorBot, update.CallbackQuery)
		}
	}
}

func handleMessage(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	text := msg.Text

	saveUser(msg.From)

	// Map-dan o'qishda Lock ishlatamiz
	mu.RLock()
	state := userState[chatID]
	mu.RUnlock()

	switch {
	case text == "/start":
		sendWelcome(chatID)

	case text == "/mybots":
		sendMyBots(chatID, msg.From.ID)

	case state == "waiting_token":
		handleTokenInput(chatID, int64(msg.From.ID), text)

	case state == "waiting_new_token":
		handleTokenChangeInput(chatID, int64(msg.From.ID), text)

	case state == "wait_amount":
		ProcessTopUpAmount(chatID, int64(msg.From.ID), text)

	default:
		sendMainMenu(chatID)
	}
}

func sendWelcome(chatID int64) {
	keyboard := RangliKlaviatura{
		InlineKeyboard: [][]RangliTugma{
			{
				{Text: "Bot yaratish", CallbackData: "create_bot", Style: "primary", IconCustomEmojiID: "5472353724598884612"},
				{Text: "Mening botlarim", CallbackData: "my_bots", Style: "primary", IconCustomEmojiID: "5472098801109997416"},
			},
			{
				{Text: "Balansni to'ldirish", CallbackData: "top_up_balance", Style: "primary", IconCustomEmojiID: "5472098698030781431"},
				{Text: "Mening balansim", CallbackData: "my_balance", Style: "primary", IconCustomEmojiID: "5469891368308482853"}},
			{
				{Text: "WEB PANELGA O'TISH", URL: "http://67.211.211.44:8080", Style: "primary", IconCustomEmojiID: "5470101946260037454"}}},
	}

	fromChatID := int64(-1003705222257)
	messageID := 2

	copyMsg := tgbotapi.NewCopyMessage(chatID, fromChatID, messageID)

	copyMsg.ReplyMarkup = keyboard

	_, err := CreatorBot.Send(copyMsg)
	if err != nil {
		log.Printf("Xabarni ko'chirishda xatolik: %v", err)
	}
}

func handleMyBalance(chatID int64, userID int64) {
	o := orm.NewOrm()

	var owner models.UserBot
	err := o.QueryTable(new(models.UserBot)).
		Filter("TgId", userID).
		One(&owner)

	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "Siz hali ro'yxatdan o'tmagansiz. /start bosing.")
		CreatorBot.Send(msg)
		return
	}

	// 1. CopyMessageConfig obyekti (Sizning tuzilmangiz)
	copyMsg := tgbotapi.CopyMessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID: chatID,
			// 2. Rangli klaviaturani shu yerda shakllantiramiz
			ReplyMarkup: RangliKlaviatura{
				InlineKeyboard: [][]RangliTugma{
					// 1-qator: Balans miqdori (Ko'k rang)
					{
						{
							Text:              fmt.Sprintf("Balans: %.0f so'm", owner.Balance),
							CallbackData:      "ignore",
							Style:             "primary",             // Ko'k rang
							IconCustomEmojiID: "5472098698030781431", // Balans uchun emoji ID

						},
					},
					// 2-qator: Balansni to'ldirish (Yashil rang)
					{
						{
							Text:              "Balansni to'ldirish",
							CallbackData:      "top_up_balance",
							Style:             "success",             // Yashil rang
							IconCustomEmojiID: "5469760599439223575", // Balans uchun emoji ID
						},
					},
				},
			},
		},
		FromChatID: -1003705222257,
		MessageID:  14,
	}

	// 3. DIQQAT: CopyMessageConfig bilan ishlashda ham CreatorBot.Send() ishlatiladi
	_, err = CreatorBot.Send(copyMsg)
	if err != nil {
		log.Printf("Balans xabarini nusxalashda xatolik: %v", err)
	}
}

func sendMainMenu(chatID int64) {
	keyboard := RangliKlaviatura{
		InlineKeyboard: [][]RangliTugma{
			{
				{Text: "Bot yaratish", CallbackData: "create_bot", Style: "primary", IconCustomEmojiID: "5472353724598884612"},
				{Text: "Mening botlarim", CallbackData: "my_bots", Style: "primary", IconCustomEmojiID: "5472098801109997416"},
			},
			{
				{Text: "Balansni to'ldirish", CallbackData: "top_up_balance", Style: "primary", IconCustomEmojiID: "5472098698030781431"},
				{Text: "Mening balansim", CallbackData: "my_balance", Style: "primary", IconCustomEmojiID: "5469891368308482853"}},
			{
				{Text: "WEB PANELGA O'TISH", URL: "http://67.211.211.44:8080", Style: "primary", IconCustomEmojiID: "5470101946260037454"}}},
	}

	fromChatID := int64(-1003705222257)
	messageID := 2

	copyMsg := tgbotapi.NewCopyMessage(chatID, fromChatID, messageID)

	copyMsg.ReplyMarkup = keyboard

	_, err := CreatorBot.Send(copyMsg)
	if err != nil {
		log.Printf("Xabarni ko'chirishda xatolik: %v", err)
	}
}

func handleCallback(bot *tgbotapi.BotAPI, cb *tgbotapi.CallbackQuery) {
	if cb.Message == nil {
		return
	}

	chatID := cb.Message.Chat.ID
	data := cb.Data
	userID := cb.From.ID

	bot.Request(tgbotapi.NewCallback(cb.ID, ""))

	if strings.HasPrefix(data, "anime_page:") ||
		strings.HasPrefix(data, "anime_part:") ||
		strings.HasPrefix(data, "confirm_delete_anime:") ||
		strings.HasPrefix(data, "delete_anime:") ||
		strings.HasPrefix(data, "edit_code:") ||
		strings.HasPrefix(data, "edit_name:") ||
		strings.HasPrefix(data, "edit_addpart:") ||
		strings.HasPrefix(data, "edit_delpart:") ||
		strings.HasPrefix(data, "edit_photo:") ||
		data == "cancel_delete_anime" ||
		data == "user_vip_add" ||
		data == "user_vip_remove" ||
		data == "user_vip_list" ||
		data == "user_block_add" ||
		data == "user_block_remove" ||
		data == "user_block_list" {

		log.Printf("→ Anime callback yo'naltirilmoqda: %s", data)
		HandleAnimeCallback(bot, cb)
		return
	}

	// ==================== MAIN BOT CALLBACKS ====================
	switch {
	case data == "create_bot":
		sendBotTypeSelection(chatID)

	case data == "my_bots":
		log.Println("MY_BOTS tugmasi bosildi, User:", userID)
		sendMyBots(chatID, int64(userID))

	case strings.HasPrefix(data, "type_"):
		botTypeCode := strings.TrimPrefix(data, "type_")
		mu.Lock()
		userBotType[chatID] = botTypeCode
		userState[chatID] = "waiting_token"
		mu.Unlock()
		sendTokenRequest(chatID, botTypeCode)

	case data == "back_main":
		mu.Lock()
		delete(userState, chatID)
		mu.Unlock()
		sendMainMenu(chatID)

	case data == "top_up_balance":
		mu.Lock()
		userState[chatID] = "wait_amount"
		mu.Unlock()
		text := "Hisobni to'ldirish\n\nTo'lamoqchi bo'lgan summani kiriting (so'mda):\n_Masalan: 50000 yoki 100000_"
		sendUserBot(bot, chatID, text)
		return

	case data == "my_balance":
		handleMyBalance(chatID, int64(userID))

	case strings.HasPrefix(data, "delete_bot:"):
		idStr := strings.TrimPrefix(data, "delete_bot:")
		var botID int64
		fmt.Sscanf(idStr, "%d", &botID)
		sendDeleteConfirmation(chatID, botID)

	case strings.HasPrefix(data, "deactivate_bot:"):
		idStr := strings.TrimPrefix(data, "deactivate_bot:")
		var botID int64
		fmt.Sscanf(idStr, "%d", &botID)
		deactivateBot(chatID, int64(userID), botID)

	case strings.HasPrefix(data, "hard_delete_bot:"):
		idStr := strings.TrimPrefix(data, "hard_delete_bot:")
		var botID int64
		fmt.Sscanf(idStr, "%d", &botID)
		hardDeleteBot(chatID, int64(userID), botID)

	case strings.HasPrefix(data, "activate_bot:"):
		idStr := strings.TrimPrefix(data, "activate_bot:")
		var botID int64
		fmt.Sscanf(idStr, "%d", &botID)
		activateBot(chatID, int64(userID), botID)

	case strings.HasPrefix(data, "change_token:"):
		idStr := strings.TrimPrefix(data, "change_token:")
		var botID int64
		fmt.Sscanf(idStr, "%d", &botID)
		mu.Lock()
		userTokenChangeBotID[chatID] = botID
		userState[chatID] = "waiting_new_token"
		mu.Unlock()
		send(chatID, "🔑 Yangi botning tokenini @BotFather'dan olib, shu yerga yuboring:", nil)

	case strings.HasPrefix(data, "paid_claim:"):
		idStr := strings.TrimPrefix(data, "paid_claim:")
		var invoiceID int64
		fmt.Sscanf(idStr, "%d", &invoiceID)
		username := cb.From.UserName
		HandlePaidClaim(chatID, int64(userID), username, invoiceID)

	case strings.HasPrefix(data, "admin_approve:"):
		if int64(userID) != AdminChatID {
			return
		}
		idStr := strings.TrimPrefix(data, "admin_approve:")
		var invoiceID int64
		fmt.Sscanf(idStr, "%d", &invoiceID)
		HandleAdminApprove(invoiceID)

	case strings.HasPrefix(data, "admin_reject:"):
		if int64(userID) != AdminChatID {
			return
		}
		idStr := strings.TrimPrefix(data, "admin_reject:")
		var invoiceID int64
		fmt.Sscanf(idStr, "%d", &invoiceID)
		HandleAdminReject(invoiceID)

	default:
		log.Printf("Noma'lum callback data: %s", data)
	}

}

func sendBotTypeSelection(chatID int64) {
	// 1. Standart []tgbotapi o'rniga o'zimizning RangliTugma'dan massiv ochamiz
	var rows [][]RangliTugma

	for _, t := range HardcodedBotTypes {
		// Rangli tugmani shakllantiramiz
		tugma := RangliTugma{
			Text:              t.Name,
			CallbackData:      "type_" + t.Code, // Loglaringizga mos ravishda "create_" qoldirdik
			Style:             "primary",        // Ko'k rangli uslub
			IconCustomEmojiID: "5472355068923648569",
		}

		rows = append(rows, []RangliTugma{tugma})
	}

	// Orqaga tugmasini ham rangli qilamiz
	rows = append(rows, []RangliTugma{
		{
			Text:              "Orqaga",
			CallbackData:      "back_main",
			Style:             "danger", // Qizil rang
			IconCustomEmojiID: "5470135764832524129",
		},
	})

	// 2. Bizning maxsus rangli klaviaturaga yuklaymiz
	keyboard := RangliKlaviatura{
		InlineKeyboard: rows,
	}

	// 3. Xabarni shakllantirish (Siz yozgan tuzilma bo'yicha)
	msg := tgbotapi.CopyMessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID:      chatID,
			ReplyMarkup: keyboard, // Rangli klaviatura shu yerga ulanadi
		},
		FromChatID: -1003705222257,
		MessageID:  8,
	}

	// 4. DIQQAT: CopyMessage emas, Send ishlatiladi!
	_, err := CreatorBot.Send(msg)
	if err != nil {
		log.Printf("Bot turlarini yuborishda xatolik: %v", err)
	}
}

func sendTokenRequest(chatID int64, botTypeCode string) {
	copyMsg := tgbotapi.CopyMessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID: chatID,
			ReplyMarkup: RangliKlaviatura{
				InlineKeyboard: [][]RangliTugma{
					{
						{Text: "Orqaga", CallbackData: "create_bot", Style: "danger", IconCustomEmojiID: "5472353724598884612"},
					},
				},
			},
		},
		FromChatID: -1003705222257,
		MessageID:  13,
	}

	_, err := CreatorBot.Send(copyMsg)
	if err != nil {
		log.Printf("Token so'rash xabarini nusxalashda xatolik: %v", err)
	}
}

func handleTokenInput(chatID int64, userTgID int64, token string) {
	// Tokenni tekshirish
	testBot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		send(chatID, "❌ Token noto'g'ri! Iltimos, @BotFather dan to'g'ri token oling.", nil)
		return
	}

	botTypeCode := userBotType[chatID]
	delete(userState, chatID)
	delete(userBotType, chatID)

	o := orm.NewOrm()

	// Egasini bazadan aniq o'qib olamiz
	var owner models.UserBot
	err = o.QueryTable("user_bot").Filter("TgId", userTgID).One(&owner)
	if err != nil {
		// Agar egasi topilmasa, bazaga yozilmay qolgan bo'ladi, majburiy yaratamiz
		owner.TgId = userTgID
		owner.Username = ""
		id, _ := o.Insert(&owner)
		owner.Id = id
	}

	// 1. Bazada ushbu bot turi borligini tekshiramiz
	var botType models.BotType
	err = o.QueryTable("bot_type").Filter("Code", botTypeCode).One(&botType)

	// 2. AGAR BAZADA YO'Q BO'LSA
	if err != nil {
		for _, t := range HardcodedBotTypes {
			if t.Code == botTypeCode {
				botType.Name = t.Name
				botType.Code = t.Code
				botType.IsActive = true

				id, insertErr := o.Insert(&botType)
				if insertErr == nil {
					botType.Id = id
				}
				break
			}
		}
	}

	// Avval mavjudligini tekshirish
	existing := models.CreatedBot{Token: token}
	if o.Read(&existing, "Token") == nil {
		send(chatID, "⚠️ Bu token allaqachon ro'yxatdan o'tgan!", nil)
		return
	}
	// Hozirgi vaqtni olamiz (fayl tepasiga "time" paketini import qilishni unutmang)
	now := time.Now()
	trialPeriod := now.AddDate(0, 0, 3) // 3 kunlik tekin sinov muddati

	newBot := &models.CreatedBot{
		Owner:       &owner,
		BotType:     &botType,
		Token:       token,
		BotUsername: testBot.Self.UserName,
		BotName:     testBot.Self.FirstName,
		IsActive:    true,
		IsSuspended: false,
		TrialEndsAt: trialPeriod, // 🎯 Boshlang'ich muddat berildi
		PaidUntil:   now,         // 🎯 To'lov muddati hozirgi vaqtga tenglashtirildi
	}
	// Bazaga saqlash jarayonini tekshiramiz
	_, insertBotErr := o.Insert(newBot)
	if insertBotErr != nil {
		log.Printf("❌ Botni bazaga saqlashda xatolik: %v", insertBotErr)
		send(chatID, "❌ Botni saqlashda texnik xatolik yuz berdi.", nil)
		return
	}

	go services.StartBot(newBot)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Bosh sahifa", "back_main"),
		),
	)

	copyMsg := tgbotapi.NewCopyMessage(
		chatID,
		-1003705222257, // kanal/guruh ID
		12,             // kerakli post ID
	)

	copyMsg.ReplyMarkup = keyboard

	_, err = CreatorBot.Send(copyMsg)
	if err != nil {
		log.Println("Xabarni ko'chirishda xatolik:", err)
	}
}

const DailyPriceDisplay = 1500.0

func sendMyBots(chatID int64, userTgID int64) {
	o := orm.NewOrm()
	var owner models.UserBot
	err := o.QueryTable("user_bot").Filter("TgId", userTgID).One(&owner)
	if err != nil {
		log.Println("owner error:", err)
		send(chatID, "Siz hali bot yaratmagansiz. /start bosing.", nil)
		return
	}

	var bots []models.CreatedBot
	_, err = o.QueryTable("created_bot").Filter("Owner__Id", owner.Id).RelatedSel("BotType").All(&bots)
	if err != nil {
		send(chatID, "⚠️ Botlarni olishda xatolik yuz berdi: "+err.Error(), nil)
		return
	}

	if len(bots) == 0 {
		send(chatID, "📭 Sizda hali bot yo'q.\n\nYangi bot yaratish uchun tugmani bosing:", nil)
		return
	}

	header := fmt.Sprintf("Sizning botlaringiz (%d ta):\n\nUmumiy balans: %.0f so'm\n", len(bots), owner.Balance)
	send(chatID, header, nil)

	for i, b := range bots {
		botTypeName := "Noma'lum"
		if b.BotType != nil {
			botTypeName = b.BotType.Name
		}

		totalCount, _ := o.QueryTable("bot_user").Filter("Bot__Id", b.Id).Count()
		vipCount, _ := o.QueryTable("bot_user").Filter("Bot__Id", b.Id).Filter("IsVip", true).Count()
		blockedCount, _ := o.QueryTable("bot_user").Filter("Bot__Id", b.Id).Filter("IsBlocked", true).Count()

		var statusLine string
		now := time.Now()

		switch {
		case !b.IsActive:
			statusLine = "🛑 O'chirilgan (qo'lda to'xtatilgan)"
		case b.IsSuspended:
			statusLine = "⚠️ To'xtatilgan (balans yetarli emas)"
		case b.PaidUntil.Before(now):
			statusLine = "⏰ Muddat tugagan (to'xtatilishi kerak)"
		case !services.IsBotRunning(b.Id):
			statusLine = "🔴 Process ishlamayapti (qayta urinib ko'ring)"
		default:
			// Ishlayotgan bot uchun qolgan vaqt
			if owner.Balance > 0 {
				daysLeft := owner.Balance / (DailyPriceDisplay * float64(len(bots))) // oddiy hisob
				if daysLeft >= 1 {
					statusLine = fmt.Sprintf("✅ Ishlamoqda — taxminan %.0f kun", daysLeft)
				} else {
					hoursLeft := int(daysLeft * 24)
					statusLine = fmt.Sprintf("✅ Ishlamoqda — taxminan %d soat", hoursLeft)
				}
			} else {
				statusLine = "✅ Ishlamoqda"
			}
		}

		text := fmt.Sprintf(
			"%d. @%s\n   Turi: %s\n   %s\n   Jami: %d | VIP: %d | Blok: %d",
			i+1, b.BotUsername, botTypeName, statusLine, totalCount, vipCount, blockedCount,
		)

		var keyboard RangliKlaviatura

		if !b.IsActive {
			keyboard = RangliKlaviatura{
				InlineKeyboard: [][]RangliTugma{
					{{Text: "Qayta yoqish", CallbackData: fmt.Sprintf("activate_bot:%d", b.Id), Style: "success", IconCustomEmojiID: "5472371840770938927"}},
					{{Text: "Butunlay o'chirish", CallbackData: fmt.Sprintf("hard_delete_bot:%d", b.Id), Style: "danger", IconCustomEmojiID: "5470135764832524129"}},
				},
			}
		} else {
			keyboard = RangliKlaviatura{
				InlineKeyboard: [][]RangliTugma{
					{{Text: "O'chirish", CallbackData: fmt.Sprintf("delete_bot:%d", b.Id), Style: "danger"}},
					{{Text: "Tokenni o'zgartirish", CallbackData: fmt.Sprintf("change_token:%d", b.Id), Style: "primary"}},
				},
			}
		}

		send(chatID, text, keyboard)
	}
}

func saveUser(from *tgbotapi.User) {
	o := orm.NewOrm()
	user := &models.UserBot{
		TgId:     int64(from.ID),
		Username: from.UserName,
	}
	o.ReadOrCreate(user, "TgId")
}

func send(chatID int64, text string, keyboard interface{}) {
	msg := tgbotapi.NewMessage(chatID, text)

	if keyboard != nil {
		msg.ReplyMarkup = keyboard
	}

	_, err := CreatorBot.Send(msg)
	if err != nil {
		log.Printf("Send xatolik: %v", err)
	}
}

func sendDeleteConfirmation(chatID int64, botID int64) {
	text := "Vaqtincha to'xtatish — bot ishlamaydi, lekin ma'lumotlari saqlanadi, keyin qayta yoqish mumkin.\n" +
		"Butunlay o'chirish — bot va unga tegishli barcha ma'lumotlar qaytarib bo'lmas tarzda o'chiriladi."

	// Rangli tugmalardan qatorlar hosil qilamiz
	keyboard := RangliKlaviatura{
		InlineKeyboard: [][]RangliTugma{
			// 1-qator: Vaqtincha to'xtatish (Ko'k yoki Sariq uslub bo'lsa - primary/warning)
			{
				{
					Text:              "Vaqtincha to'xtatish",
					CallbackData:      fmt.Sprintf("deactivate_bot:%d", botID),
					Style:             "primary",
					IconCustomEmojiID: "5472371840770938927",
				},
			},
			// 2-qator: Butunlay o'chirish (Xavfli harakat bo'lgani uchun Qizil)
			{
				{
					Text:              "Butunlay o'chirish",
					CallbackData:      fmt.Sprintf("hard_delete_bot:%d", botID),
					Style:             "danger",
					IconCustomEmojiID: "5470135764832524129",
				},
			},
			// 3-qator: Bekor qilish (Yashil yoki neytral)
			{
				{
					Text:         "Bekor qilish",
					CallbackData: "my_bots",
					Style:        "success",
				},
			},
		},
	}

	// Agar sizda oddiy text yuboradigan maxsus send() funksiyangiz bo'lsa,
	// u `RangliKlaviatura`ni qabul qila olishi kerak.
	// Yoki to'g'ridan-to'g'ri quyidagicha yuborasiz:
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard

	_, err := CreatorBot.Send(msg)
	if err != nil {
		log.Printf("O'chirishni tasdiqlash menyusini yuborishda xatolik: %v", err)
	}
}

func hardDeleteBot(chatID int64, userTgID int64, botID int64) {
	o := orm.NewOrm()

	var bot models.CreatedBot
	err := o.QueryTable("created_bot").Filter("Id", botID).RelatedSel("Owner").One(&bot)
	if err != nil || bot.Owner == nil || bot.Owner.TgId != userTgID {
		send(chatID, "❌ Bot topilmadi yoki sizga tegishli emas.", nil)
		return
	}

	botUsername := bot.BotUsername

	services.StopBot(bot.Id) // 🎯 avval runtime'dagi pollingni to'xtatamiz

	_, err = o.QueryTable("bot_user").Filter("Bot__Id", bot.Id).Delete()
	if err != nil {
		log.Println("bot_user o'chirishda xatolik:", err)
	}

	_, err = o.Delete(&bot)
	if err != nil {
		send(chatID, "❌ Botni o'chirishda xatolik yuz berdi.", nil)
		return
	}

	send(chatID, fmt.Sprintf("🗑 @%s butunlay o'chirildi.", botUsername), nil)
}

func deactivateBot(chatID int64, userTgID int64, botID int64) {
	o := orm.NewOrm()

	var bot models.CreatedBot
	err := o.QueryTable("created_bot").Filter("Id", botID).RelatedSel("Owner").One(&bot)
	if err != nil || bot.Owner == nil || bot.Owner.TgId != userTgID {
		send(chatID, "❌ Bot topilmadi yoki sizga tegishli emas.", nil)
		return
	}

	bot.IsActive = false
	_, err = o.Update(&bot, "IsActive")
	if err != nil {
		send(chatID, "❌ Botni to'xtatishda xatolik yuz berdi.", nil)
		return
	}

	services.StopBot(bot.Id) // 🎯 runtime'da pollingni to'xtatadi

	send(chatID, fmt.Sprintf("@%s vaqtincha to'xtatildi.", bot.BotUsername), nil)
}

func activateBot(chatID int64, userTgID int64, botID int64) {
	o := orm.NewOrm()

	var bot models.CreatedBot
	err := o.QueryTable("created_bot").
		Filter("Id", botID).
		RelatedSel("Owner").   // ⬅️ aniq nom bilan
		RelatedSel("BotType"). // ⬅️ shuni qo'shdik — asosiy tuzatish
		One(&bot)
	if err != nil || bot.Owner == nil || bot.Owner.TgId != userTgID {
		send(chatID, "❌ Bot topilmadi yoki sizga tegishli emas.", nil)
		return
	}

	if bot.IsSuspended {
		send(chatID, "⚠️ Bu bot balans yetarli emasligi sababli to'xtatilgan. Avval balansni to'ldiring.", nil)
		return
	}

	bot.IsActive = true
	_, err = o.Update(&bot, "IsActive")
	if err != nil {
		send(chatID, "❌ Botni yoqishda xatolik yuz berdi.", nil)
		return
	}

	services.StartBot(&bot) // 🎯 runtime'da pollingni qayta ishga tushiradi

	send(chatID, fmt.Sprintf("@%s qayta yoqildi.", bot.BotUsername), nil)
}

func handleTokenChangeInput(chatID int64, userTgID int64, newToken string) {
	mu.Lock()
	botID, exists := userTokenChangeBotID[chatID]
	delete(userTokenChangeBotID, chatID)
	delete(userState, chatID)
	mu.Unlock()

	if !exists {
		send(chatID, "❌ Xatolik: qaysi bot ekanligi aniqlanmadi. Qaytadan urinib ko'ring.", nil)
		return
	}

	// 1. Yangi tokenni tekshiramiz
	testBot, err := tgbotapi.NewBotAPI(newToken)
	if err != nil {
		send(chatID, "❌ Token noto'g'ri! Iltimos, @BotFather dan to'g'ri token oling yoki qaytadan yuboring.", nil)
		// state'ni qaytaramiz, foydalanuvchi qayta urinib ko'rishi uchun
		mu.Lock()
		userTokenChangeBotID[chatID] = botID
		userState[chatID] = "waiting_new_token"
		mu.Unlock()
		return
	}

	o := orm.NewOrm()

	var bot models.CreatedBot
	err = o.QueryTable("created_bot").Filter("Id", botID).
		RelatedSel("Owner").
		RelatedSel("BotType").
		One(&bot)

	// 2. Token boshqa botda ishlatilmaganini tekshiramiz
	existing := models.CreatedBot{Token: newToken}
	if o.Read(&existing, "Token") == nil && existing.Id != bot.Id {
		send(chatID, "⚠️ Bu token allaqachon boshqa bot tomonidan ishlatilmoqda!", nil)
		return
	}

	oldUsername := bot.BotUsername

	// 3. Bazani yangi token bilan yangilaymiz
	bot.Token = newToken
	bot.BotUsername = testBot.Self.UserName
	bot.BotName = testBot.Self.FirstName
	_, err = o.Update(&bot, "Token", "BotUsername", "BotName")
	if err != nil {
		send(chatID, "❌ Tokenni bazada yangilashda xatolik yuz berdi.", nil)
		return
	}

	// 4. Eski va yangi bot bir vaqtda tekshiriladi — muvaffaqiyatli bo'lgani uchun
	//    endi eski runtime'ni to'xtatib, yangi token bilan qayta ishga tushiramiz
	services.StopBot(bot.Id)
	services.StartBot(&bot)

	send(chatID, fmt.Sprintf("✅ Token muvaffaqiyatli o'zgartirildi!\n\nEski: @%s\nYangi: @%s", oldUsername, bot.BotUsername), nil)
}
