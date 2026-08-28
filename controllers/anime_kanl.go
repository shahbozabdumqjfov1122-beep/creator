package controllers

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"creator/models"

	"github.com/beego/beego/v2/client/orm"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	adminState       = make(map[int64]string)
	adminKinoID      = make(map[int64]int64) // ← yangi
	adminTempChannel = make(map[int64]int64)
	quickAnimeTemp   = make(map[int64]string) // 🎯 shu qatorni qo'shing
	quickKinoTemp    = make(map[int64]string)
	pendingSearch    = make(map[int64]string) // 🎯 obuna kutilayotgan qidiruv kodi

)

func parseChannelID(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) (int64, error) {
	if msg.ForwardFromChat != nil {
		if msg.ForwardFromChat.IsChannel() {
			return msg.ForwardFromChat.ID, nil
		}
	}

	text := strings.TrimSpace(msg.Text)

	if strings.HasPrefix(text, "-100") {
		id, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("id_format_error")
		}
		return id, nil
	}

	if !strings.HasPrefix(text, "@") {
		text = "@" + text
	}

	chat, err := bot.GetChat(
		tgbotapi.ChatInfoConfig{
			ChatConfig: tgbotapi.ChatConfig{
				SuperGroupUsername: text,
			},
		},
	)
	if err != nil {
		return 0, fmt.Errorf("bot_not_admin_or_not_found")
	}

	return chat.ID, nil
}

func HandleAdminCommands(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
	userID := msg.From.ID

	log.Printf("🟣 [HandleAdminCommands] boshlandi. UserID=%d, BotID=%d, Text=%q", userID, b.Id, msg.Text)

	if msg.Text == "/addchannel" || msg.Text == "➕ Kanal qo‘shish" || msg.Text == "Kanall qo‘shish" {
		mu.Lock()
		adminState[userID] = "wait_channel"
		mu.Unlock()
		log.Printf("🟢 [HandleAdminCommands] state='wait_channel' o'rnatildi. UserID=%d", userID)
		sendUserBot(bot, msg.Chat.ID, "📢 Kanal ID yoki @username yuboring...")
		return
	}

	if msg.Text == "/vipnarx" || msg.Text == "💎 vip narx qo'shish" {
		mu.Lock()
		adminState[userID] = "wait_vip_name"
		mu.Unlock()
		log.Printf("🟢 [HandleAdminCommands] state='wait_vip_name' o'rnatildi. UserID=%d", userID)
		sendUserBot(bot, msg.Chat.ID, "tarif nomini yuboring.\nMasalan: 1 oylik obuna")
		return
	}

	mu.Lock()
	state, hasState := adminState[userID]
	mu.Unlock()

	if !hasState {
		log.Printf("🟡 [HandleAdminCommands] UserID=%d uchun state topilmadi. Chiqib ketilmoqda.", userID)
		return
	}

	log.Printf("🔵 [HandleAdminCommands] joriy state=%q, UserID=%d", state, userID)

	switch state {
	case "wait_channel":
		log.Printf("🟣 [wait_channel] parseChannelID chaqirilmoqda. Text=%q", msg.Text)
		channelID, err := parseChannelID(bot, msg)
		if err != nil {
			log.Printf("🔴 [wait_channel] parseChannelID xatosi: %v", err)

			// 🎯 FIX: Admin xato matn yuborganida statelarni tozalaymiz, bot tiqilib qolmaydi!
			mu.Lock()
			delete(adminState, userID)
			delete(adminTempChannel, userID)
			mu.Unlock()

			if err.Error() == "id_format_error" {
				sendUserBot(bot, msg.Chat.ID, "❌ Noto‘g‘ri channel ID yoki username formati! Jarayon bekor qilindi.")
			} else {
				sendUserBot(bot, msg.Chat.ID, "❌ Kanal topilmadi yoki bot u yerda admin emas! Jarayon bekor qilindi.")
			}
			log.Printf("🟡 [wait_channel] state tozalandi, jarayon bekor qilindi. UserID=%d", userID)
			return
		}

		log.Printf("🟢 [wait_channel] channelID topildi: %d", channelID)

		// Agar hammasi to'g'ri bo'lsa, keyingi qadamga o'tadi...
		mu.Lock()
		adminTempChannel[userID] = channelID
		adminState[userID] = "wait_link"
		mu.Unlock()

		log.Printf("🟢 [wait_channel] state='wait_link' ga o'tkazildi. UserID=%d, channelID=%d", userID, channelID)
		sendUserBot(bot, msg.Chat.ID, "🔗 Endi kanal uchun Invite link yuboring (https://t.me/....)")
		return

	case "wait_link":
		link := strings.TrimSpace(msg.Text)
		log.Printf("🟣 [wait_link] link qabul qilindi: %q", link)

		if link == "" || !strings.HasPrefix(link, "http") {
			log.Printf("🔴 [wait_link] noto'g'ri link format: %q", link)
			sendUserBot(bot, msg.Chat.ID, "❌ Iltimos, to'g'ri havola (link) yuboring!")
			return
		}

		mu.Lock()
		channelID := adminTempChannel[userID]
		mu.Unlock()

		log.Printf("🔵 [wait_link] bazaga yozilmoqda. channelID=%d, link=%q, BotID=%d", channelID, link, b.Id)

		o := orm.NewOrm()

		bc := models.BotChannel{
			Bot:        &models.CreatedBot{Id: b.Id},
			ChannelID:  channelID,
			InviteLink: link,
			IsActive:   true,
			CreatedAt:  time.Now(),
		}

		_, err := o.Insert(&bc)
		if err != nil {
			log.Printf("🔴 [wait_link] bazaga saqlashda xatolik: %v", err)
			sendUserBot(bot, msg.Chat.ID, "❌ Ma'lumotlar bazasiga saqlashda xato yuz berdi.")
			return
		}

		log.Printf("✅ [wait_link] Kanal muvaffaqiyatli qo'shildi! channelID=%d, BotID=%d", channelID, b.Id)
		sendUserBot(bot, msg.Chat.ID, fmt.Sprintf("✅ Kanal muvaffaqiyatli qo‘shildi!\n📢 ID: %d", channelID))

		mu.Lock()
		delete(adminState, userID)
		delete(adminTempChannel, userID)
		mu.Unlock()
		log.Printf("🏁 [wait_link] state va tempChannel tozalandi. UserID=%d", userID)

	case "wait_vip_name":
		name := strings.TrimSpace(msg.Text)
		log.Printf("🟣 [wait_vip_name] nom qabul qilindi: %q", name)

		if name == "" {
			log.Printf("🔴 [wait_vip_name] nom bo'sh")
			sendUserBot(bot, msg.Chat.ID, "❌ Nom bo'sh bo'lishi mumkin emas. Qaytadan yuboring.")
			return
		}

		mu.Lock()
		quickKinoTemp[userID] = name // 🎯 mavjud mapni qayta ishlatamiz
		adminState[userID] = "wait_vip_price"
		mu.Unlock()

		log.Printf("🟢 [wait_vip_name] state='wait_vip_price' ga o'tdi. name=%q, UserID=%d", name, userID)
		sendUserBot(bot, msg.Chat.ID, "Endi shu tarif uchun narxni yuboring.\nMasalan: 15 000 so'm")
		return

	case "wait_vip_price":
		price := strings.TrimSpace(msg.Text)
		log.Printf("🟣 [wait_vip_price] narx qabul qilindi: %q", price)

		if price == "" {
			log.Printf("🔴 [wait_vip_price] narx bo'sh")
			sendUserBot(bot, msg.Chat.ID, "❌ Narx bo'sh bo'lishi mumkin emas. Qaytadan yuboring.")
			return
		}

		mu.Lock()
		name := quickKinoTemp[userID]
		delete(quickKinoTemp, userID)
		delete(adminState, userID)
		mu.Unlock()

		log.Printf("🔵 [wait_vip_price] bazadan bot o'qilmoqda. BotID=%d", b.Id)

		o := orm.NewOrm()
		createdBot := models.CreatedBot{Id: b.Id}
		if err := o.Read(&createdBot); err != nil {
			log.Printf("🔴 [wait_vip_price] bot o'qishda xatolik: %v", err)
			sendUserBot(bot, msg.Chat.ID, "❌ Bot ma'lumotini o'qishda xato.")
			return
		}

		newLine := fmt.Sprintf("💎 VIP Obuna Tariflari\n\n %s - %s\n", name, price)
		createdBot.VipPrices += newLine

		log.Printf("🔵 [wait_vip_price] yangi qator qo'shilmoqda: %q", newLine)

		if _, err := o.Update(&createdBot, "VipPrices"); err != nil {
			log.Printf("🔴 [wait_vip_price] saqlashda xatolik: %v", err)
			sendUserBot(bot, msg.Chat.ID, "❌ Saqlashda xato yuz berdi.")
			return
		}

		log.Printf("✅ [wait_vip_price] VIP tarif muvaffaqiyatli qo'shildi. name=%q, price=%q, BotID=%d", name, price, b.Id)
		sendUserBot(bot, msg.Chat.ID, fmt.Sprintf("✅ Qo'shildi:\n%s", newLine))
		return

	default:
		log.Printf("🟡 [HandleAdminCommands] default blokka tushdi. state=%q", state)

		if strings.HasPrefix(state, "wait_vip_edit_name:") {
			idxStr := strings.TrimPrefix(state, "wait_vip_edit_name:")
			idx, _ := strconv.Atoi(idxStr)
			log.Printf("🟣 [wait_vip_edit_name] idx=%d", idx)

			name := strings.TrimSpace(msg.Text)
			if name == "" {
				log.Printf("🔴 [wait_vip_edit_name] nom bo'sh")
				sendUserBot(bot, msg.Chat.ID, "❌ Nom bo'sh bo'lishi mumkin emas.")
				return
			}

			mu.Lock()
			quickKinoTemp[userID] = name
			adminState[userID] = fmt.Sprintf("wait_vip_edit_price:%d", idx)
			mu.Unlock()

			log.Printf("🟢 [wait_vip_edit_name] state='wait_vip_edit_price:%d' ga o'tdi. name=%q", idx, name)
			sendUserBot(bot, msg.Chat.ID, "💰 Endi yangi narxni yuboring:")
			return
		}

		if strings.HasPrefix(state, "wait_vip_edit_price:") {
			idxStr := strings.TrimPrefix(state, "wait_vip_edit_price:")
			idx, _ := strconv.Atoi(idxStr)
			log.Printf("🟣 [wait_vip_edit_price] idx=%d", idx)

			price := strings.TrimSpace(msg.Text)
			if price == "" {
				log.Printf("🔴 [wait_vip_edit_price] narx bo'sh")
				sendUserBot(bot, msg.Chat.ID, "❌ Narx bo'sh bo'lishi mumkin emas.")
				return
			}

			mu.Lock()
			name := quickKinoTemp[userID]
			delete(quickKinoTemp, userID)
			delete(adminState, userID)
			mu.Unlock()

			log.Printf("🔵 [wait_vip_edit_price] bazadan bot o'qilmoqda. BotID=%d", b.Id)

			o := orm.NewOrm()
			createdBot := models.CreatedBot{Id: b.Id}
			if err := o.Read(&createdBot); err != nil {
				log.Printf("🔴 [wait_vip_edit_price] bot o'qishda xatolik: %v", err)
				sendUserBot(bot, msg.Chat.ID, "❌ Ma'lumotni o'qishda xato.")
				return
			}

			lines := parseVipLines(createdBot.VipPrices)
			log.Printf("🔵 [wait_vip_edit_price] jami tariflar soni: %d, tahrirlanayotgan idx=%d", len(lines), idx)

			if idx < 0 || idx >= len(lines) {
				log.Printf("🔴 [wait_vip_edit_price] idx=%d chegaradan tashqarida (jami=%d)", idx, len(lines))
				sendUserBot(bot, msg.Chat.ID, "❌ Bu tarif topilmadi (o'chirilgan bo'lishi mumkin).")
				return
			}

			lines[idx] = fmt.Sprintf("%s: %s", name, price)
			createdBot.VipPrices = strings.Join(lines, "\n") + "\n"

			if _, err := o.Update(&createdBot, "VipPrices"); err != nil {
				log.Printf("🔴 [wait_vip_edit_price] saqlashda xatolik: %v", err)
				sendUserBot(bot, msg.Chat.ID, "❌ Saqlashda xato yuz berdi.")
				return
			}

			log.Printf("✅ [wait_vip_edit_price] tarif yangilandi: idx=%d, name=%q, price=%q", idx, name, price)
			sendUserBot(bot, msg.Chat.ID, fmt.Sprintf("✅ Yangilandi:\n%s: %s", name, price))
			return
		}

		log.Printf("🔴 [HandleAdminCommands] noma'lum state, hech qanday amal bajarilmadi: %q", state)
	}

	log.Printf("🏁 [HandleAdminCommands] tugadi. UserID=%d", userID)
}

func ShowMembership(bot *tgbotapi.BotAPI, b *models.CreatedBot, chatID int64, userID int64) {
	o := orm.NewOrm()
	var channels []models.BotChannel

	_, err := o.QueryTable(new(models.BotChannel)).
		Filter("Bot__Id", b.Id).
		Filter("IsActive", true).
		All(&channels)

	if err != nil || len(channels) == 0 {
		return
	}

	var notSubscribed []models.BotChannel
	for _, ch := range channels {
		member, err := bot.GetChatMember(tgbotapi.GetChatMemberConfig{
			ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
				ChatID: ch.ChannelID,
				UserID: userID,
			},
		})

		if err == nil && (member.Status == "member" || member.Status == "administrator" || member.Status == "creator") {
			continue
		}

		hasRequest := o.QueryTable(new(models.BotJoinRequest)).
			Filter("Bot__Id", b.Id).
			Filter("TgId", userID).
			Filter("ChannelID", ch.ChannelID).
			Exist()

		if hasRequest {
			continue // Zayavka tashlagan bo'lsa ham qo'shmaymiz
		}

		notSubscribed = append(notSubscribed, ch)
	}

	// Agar hammasiga obuna bo'lgan bo'lsa — ko'rsatishga hojat yo'q
	if len(notSubscribed) == 0 {
		return
	}

	text := "⚠️ Botdan foydalanish uchun obuna bo‘ling:\n\n"

	var rows [][]RangliTugma

	for _, ch := range notSubscribed {
		obunaTugma := RangliTugma{
			Text:              "OBUNA BO'LISH",
			URL:               ch.InviteLink,
			IconCustomEmojiID: "5775887550262546277",
		}
		rows = append(rows, []RangliTugma{obunaTugma})
	}

	checkBtn := RangliTugma{
		Text:              "Tekshirish",
		CallbackData:      fmt.Sprintf("check_sub_%d", b.Id),
		Style:             "success",
		IconCustomEmojiID: "5460960662421257616",
	}

	rows = append(rows, []RangliTugma{checkBtn})
	vipBtn := RangliTugma{
		Text:              " 💎 vip",
		CallbackData:      fmt.Sprintf("vip_prices_%d", b.Id),
		Style:             "primary",
		IconCustomEmojiID: "5310134444541819884",
	}
	rows = append(rows, []RangliTugma{vipBtn})
	keyboard := RangliKlaviatura{
		InlineKeyboard: rows,
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard

	_, err = bot.Send(msg)
	if err != nil {
		log.Printf("Obuna xabarini yuborishda xatolik: %v", err)
	}
}

func CheckSubscription(bot *tgbotapi.BotAPI, b *models.CreatedBot, userID int64) bool {
	o := orm.NewOrm()
	var channels []models.BotChannel

	// Botga bog'langan barcha faol majburiy obuna kanallarini olamiz
	_, err := o.QueryTable(new(models.BotChannel)).
		Filter("Bot__Id", b.Id).
		Filter("IsActive", true).
		All(&channels)

	// Agar majburiy kanallar sozlanmagan bo'lsa, tekshirmasdan o'tkazaveramiz
	if err != nil || len(channels) == 0 {
		return true
	}

	// Har bir kanalni bittalab tekshiramiz
	for _, ch := range channels {
		// 1. Telegram API orqali rasmiy tekshirish
		member, err := bot.GetChatMember(tgbotapi.GetChatMemberConfig{
			ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
				ChatID: ch.ChannelID,
				UserID: userID,
			},
		})

		// Agar foydalanuvchi kanalda a'zo, admin yoki yaratuvchi bo'lsa - hammasi joyida
		if err == nil && (member.Status == "member" || member.Status == "administrator" || member.Status == "creator") {
			continue // Bu kanal muvaffaqiyatli o'tdi, keyingi kanalga o'tamiz
		}

		// 2. 🔥 TELEGRAMDA TOPILMASA: Bizning bazadan "Zayavka" (Join Request) tashlaganini tekshiramiz
		hasRequest := o.QueryTable(new(models.BotJoinRequest)).
			Filter("Bot__Id", b.Id).
			Filter("TgId", userID).
			Filter("ChannelID", ch.ChannelID).
			Exist()

		if hasRequest {
			// Foydalanuvchi zayavka tashlagan ekan! Unga botni ishlatishga ruxsat beramiz
			continue
		}

		// Agar foydalanuvchi guruhda a'zo ham bo'lmasa va zayavka ham tashlamagan bo'lsa - demak o'tolmadi
		return false
	}

	// Agar hamma kanallardan muvaffaqiyatli o'tsa - true qaytadi
	return true
}

func ShowChannelsToDelete(bot *tgbotapi.BotAPI, b *models.CreatedBot, chatID int64) {
	o := orm.NewOrm()
	var channels []models.BotChannel

	_, err := o.QueryTable(new(models.BotChannel)).
		Filter("Bot__Id", b.Id).
		Filter("IsActive", true).
		All(&channels)

	if err != nil || len(channels) == 0 {
		sendUserBot(bot, chatID, "📭 Hozircha o‘chirish uchun hech qanday kanal sozlanmagan.")
		return
	}

	text := "🗑 O‘chirmoqchi bo‘lgan kanalingiz ustiga bosing:\n\n⚠️ Diqqat! Kanal o‘chirilsa, bot uni majburiy obunadan olib tashlaydi."
	var rows [][]tgbotapi.InlineKeyboardButton

	for _, ch := range channels {
		btnText := fmt.Sprintf("❌ Kanal ID: %d", ch.ChannelID)
		btn := tgbotapi.NewInlineKeyboardButtonData(btnText, fmt.Sprintf("del_chan_%d", ch.Id))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...) // Mana shu yer muammosiz holatga keltirildi
	bot.Send(msg)
}

func HandleVipPricesCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, botID int64) {
	if callback.Message == nil {
		bot.Request(tgbotapi.NewCallback(callback.ID, "❌ Xabar topilmadi."))
		return
	}

	o := orm.NewOrm()
	createdBot := models.CreatedBot{Id: botID}

	err := o.Read(&createdBot)
	if err != nil {
		bot.Request(tgbotapi.NewCallback(callback.ID, "❌ Ma'lumot topilmadi!"))
		return
	}

	o.LoadRelated(&createdBot, "Owner")

	pricesText := createdBot.VipPrices
	if pricesText == "" {
		pricesText = "💎 VIP Obuna Tariflari\n\n" +
			" 1 oylik: 10,000 so'm\n" +
			" 3 oylik: 25,000 so'm\n" +
			" Cheksiz (VIP): 50,000 so'm\n\n" +
			"💳 Sotib olish uchun admin bilan bog'laning!"
	}

	if createdBot.Note != "" {
		pricesText += fmt.Sprintf("\n\n📌 Eslatma: %s", createdBot.Note)
	}

	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, pricesText)
	msg.ParseMode = "Markdown"

	if createdBot.Owner != nil {
		var contactURL string
		if createdBot.Owner.Username != "" {
			contactURL = "https://t.me/" + createdBot.Owner.Username
		} else {
			contactURL = fmt.Sprintf("tg://user?id=%d", createdBot.Owner.TgId)
		}

		contactBtn := tgbotapi.NewInlineKeyboardButtonURL("💎 sotib olish", contactURL)
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(contactBtn),
		)
	}

	// 🎯 FIX: Markdown xato bersa, formatlashsiz qayta yuboramiz
	if _, sendErr := bot.Send(msg); sendErr != nil {
		log.Printf("🟡 Markdown xatosi, formatlashsiz qayta yuborilmoqda: %v", sendErr)
		msg.ParseMode = "" // Markdown o'chiriladi
		if _, sendErr2 := bot.Send(msg); sendErr2 != nil {
			log.Printf("🔴 Ikkinchi urinishda ham xato: %v", sendErr2)
		}
	}

	bot.Request(tgbotapi.NewCallback(callback.ID, ""))
}

func parseVipLines(vipPrices string) []string {
	raw := strings.Split(strings.TrimSpace(vipPrices), "\n")
	var lines []string
	for _, l := range raw {
		l = strings.TrimSpace(l)
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

func showVipListForAction(bot *tgbotapi.BotAPI, b *models.CreatedBot, chatID int64, userID int64, action string) {
	o := orm.NewOrm()
	createdBot := models.CreatedBot{Id: b.Id}
	if err := o.Read(&createdBot); err != nil {
		sendUserBot(bot, chatID, "❌ Ma'lumotni o'qishda xato.")
		return
	}

	lines := parseVipLines(createdBot.VipPrices)
	if len(lines) == 0 {
		sendUserBot(bot, chatID, "📭 Hozircha hech qanday VIP narx qo'shilmagan.")
		return
	}

	var text string
	var rows [][]tgbotapi.InlineKeyboardButton

	if action == "delete" {
		text = "🗑 O'chirmoqchi bo'lgan tarifni tanlang:"
	} else {
		text = "✏️ Tahrirlamoqchi bo'lgan tarifni tanlang:"
	}

	for i, line := range lines {
		btnText := line
		if len(btnText) > 40 {
			btnText = btnText[:40] + "..."
		}
		var cb string
		if action == "delete" {
			cb = fmt.Sprintf("vip_del:%d", i)
		} else {
			cb = fmt.Sprintf("vip_edit:%d", i)
		}
		btn := tgbotapi.NewInlineKeyboardButtonData(btnText, cb)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	bot.Send(msg)
}
