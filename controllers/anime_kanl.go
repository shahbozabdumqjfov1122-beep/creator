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
	adminKinoID      = make(map[int64]int64)
	adminTempChannel = make(map[int64]int64)
	quickAnimeTemp   = make(map[int64]string)
	quickKinoTemp    = make(map[int64]string)
	pendingSearch    = make(map[int64]string)
)

func parseVipDuration(tariffText string) time.Time {
	now := time.Now()
	lower := strings.ToLower(tariffText)

	// Tarif matnidan kun/oy/yilni qidiramiz
	if strings.Contains(lower, "7 kun") {
		return now.AddDate(0, 0, 7)
	} else if strings.Contains(lower, "10 kun") {
		return now.AddDate(0, 0, 10)
	} else if strings.Contains(lower, "15 kun") {
		return now.AddDate(0, 0, 15)
	} else if strings.Contains(lower, "1 oy") {
		return now.AddDate(0, 1, 0)
	} else if strings.Contains(lower, "3 oy") {
		return now.AddDate(0, 3, 0)
	} else if strings.Contains(lower, "6 oy") {
		return now.AddDate(0, 6, 0)
	} else if strings.Contains(lower, "1 yil") {
		return now.AddDate(1, 0, 0)
	} else if strings.Contains(lower, "cheksiz") {
		return now.AddDate(100, 0, 0) // 100 yil
	}

	// Agar tarif matnida aniq muddat topilmasa, standart 1 oy (30 kun) beriladi
	return now.AddDate(0, 1, 0)
}

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
	chatID := msg.Chat.ID

	log.Printf("🟣 [HandleAdminCommands] boshlandi. UserID=%d, BotID=%d, Text=%q", userID, b.Id, msg.Text)

	if msg.Text == "/addchannel" || msg.Text == "➕ Kanal qo‘shish" || msg.Text == "Kanall qo‘shish" {
		mu.Lock()
		adminState[userID] = "wait_channel"
		mu.Unlock()
		log.Printf("🟢 [HandleAdminCommands] state='wait_channel' o'rnatildi. UserID=%d", userID)
		sendUserBot(bot, chatID, "📢 Kanal ID yoki @username yuboring...")
		return
	}

	if msg.Text == "/vipnarx" || msg.Text == "💎 vip narx qo'shish" || msg.Text == "vip narx qo'shish" {
		mu.Lock()
		adminState[userID] = "wait_vip_name"
		mu.Unlock()
		log.Printf("🟢 [HandleAdminCommands] state='wait_vip_name' o'rnatildi. UserID=%d", userID)

		// 🎛 Administrator uchun VIP muddatlari tugmalarini yaratamiz
		keyboard := tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("7 kun"),
				tgbotapi.NewKeyboardButton("10 kun"),
				tgbotapi.NewKeyboardButton("15 kun"),
			),
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("1 oy"),
				tgbotapi.NewKeyboardButton("3 oy"),
				tgbotapi.NewKeyboardButton("6 oy"),
			),
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("1 yil"),
				tgbotapi.NewKeyboardButton("Cheksiz"),
			),
		)
		keyboard.ResizeKeyboard = true

		replyMsg := tgbotapi.NewMessage(chatID, "⏳ Iltimos, VIP tarif muddatini pastdagi tugmalardan tanlang yoki o'zingiz yozing:")
		replyMsg.ReplyMarkup = keyboard
		bot.Send(replyMsg)
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

			mu.Lock()
			delete(adminState, userID)
			delete(adminTempChannel, userID)
			mu.Unlock()

			if err.Error() == "id_format_error" {
				sendUserBot(bot, chatID, "❌ Noto‘g‘ri channel ID yoki username formati! Jarayon bekor qilindi.")
			} else {
				sendUserBot(bot, chatID, "❌ Kanal topilmadi yoki bot u yerda admin emas! Jarayon bekor qilindi.")
			}
			log.Printf("🟡 [wait_channel] state tozalandi, jarayon bekor qilindi. UserID=%d", userID)
			return
		}

		log.Printf("🟢 [wait_channel] channelID topildi: %d", channelID)

		mu.Lock()
		adminTempChannel[userID] = channelID
		adminState[userID] = "wait_link"
		mu.Unlock()

		log.Printf("🟢 [wait_channel] state='wait_link' ga o'tkazildi. UserID=%d, channelID=%d", userID, channelID)
		sendUserBot(bot, chatID, "🔗 Endi kanal uchun Invite link yuboring (https://t.me/....)")
		return
	case "waiting_vip_card":
		cardInfo := strings.TrimSpace(msg.Text)
		if cardInfo == "" {
			sendUserBot(bot, chatID, "❌ Karta ma'lumoti bo'sh bo'lishi mumkin emas!")
			return
		}

		o := orm.NewOrm()
		createdBot := models.CreatedBot{Id: b.Id}
		if err := o.Read(&createdBot); err == nil {
			createdBot.Card = cardInfo
			if _, err := o.Update(&createdBot, "Card"); err == nil {
				mu.Lock()
				delete(adminState, userID)
				mu.Unlock()

				sendUserBot(bot, chatID, fmt.Sprintf("✅ Karta ma'lumoti muvaffaqiyatli saqlandi!\n\n💳 Joriy karta:\n%s", cardInfo))
				return
			}
		}
		sendUserBot(bot, chatID, "❌ Saqlashda xatolik yuz berdi. Qaytadan urinib ko'ring.")
		return
	case "wait_link":
		link := strings.TrimSpace(msg.Text)
		log.Printf("🟣 [wait_link] link qabul qilindi: %q", link)

		if link == "" || !strings.HasPrefix(link, "http") {
			log.Printf("🔴 [wait_link] noto'g'ri link format: %q", link)
			sendUserBot(bot, chatID, "❌ Iltimos, to'g'ri havola (link) yuboring!")
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
			sendUserBot(bot, chatID, "❌ Ma'lumotlar bazasiga saqlashda xato yuz berdi.")
			return
		}

		log.Printf("✅ [wait_link] Kanal muvaffaqiyatli qo'shildi! channelID=%d, BotID=%d", channelID, b.Id)
		sendUserBot(bot, chatID, fmt.Sprintf("✅ Kanal muvaffaqiyatli qo‘shildi!\n📢 ID: %d", channelID))

		mu.Lock()
		delete(adminState, userID)
		delete(adminTempChannel, userID)
		mu.Unlock()
		log.Printf("🏁 [wait_link] state va tempChannel tozalandi. UserID=%d", userID)

	case "wait_vip_name":
		name := strings.TrimSpace(msg.Text)
		log.Printf("🟣 [wait_vip_name] tarif nomi/muddati tanlandi: %q", name)

		if name == "" {
			log.Printf("🔴 [wait_vip_name] nom bo'sh")
			sendUserBot(bot, chatID, "❌ Nom bo'sh bo'lishi mumkin emas. Qaytadan yuboring.")
			return
		}

		mu.Lock()
		quickKinoTemp[userID] = name
		adminState[userID] = "wait_vip_price"
		mu.Unlock()

		log.Printf("🟢 [wait_vip_name] state='wait_vip_price' ga o'tdi. name=%q, UserID=%d", name, userID)

		// 🔕 Tugmalarni olib tashlaymiz va narx so'raymiz
		priceMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ **%s** tarif tanlandi.\n\n💰 Endi ushbu tarif narxini kiriting:\n(Masalan: 15 000 so'm)", name))
		priceMsg.ParseMode = "Markdown"
		priceMsg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
		bot.Send(priceMsg)
		return

	case "wait_vip_price":
		price := strings.TrimSpace(msg.Text)
		log.Printf("🟣 [wait_vip_price] narx qabul qilindi: %q", price)

		if price == "" {
			log.Printf("🔴 [wait_vip_price] narx bo'sh")
			sendUserBot(bot, chatID, "❌ Narx bo'sh bo'lishi mumkin emas. Qaytadan yuboring.")
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
			sendUserBot(bot, chatID, "❌ Bot ma'lumotini o'qishda xato.")
			return
		}

		// Formatni toza saqlaymiz: "1 oy - 15 000 so'm"
		newLine := fmt.Sprintf("%s - %s", name, price)

		if strings.TrimSpace(createdBot.VipPrices) == "" {
			createdBot.VipPrices = newLine + "\n"
		} else {
			createdBot.VipPrices += newLine + "\n"
		}

		log.Printf("🔵 [wait_vip_price] yangi tarif qo'shilmoqda: %q", newLine)

		if _, err := o.Update(&createdBot, "VipPrices"); err != nil {
			log.Printf("🔴 [wait_vip_price] saqlashda xatolik: %v", err)
			sendUserBot(bot, chatID, "❌ Saqlashda xato yuz berdi.")
			return
		}

		log.Printf("✅ [wait_vip_price] VIP tarif muvaffaqiyatli qo'shildi. name=%q, price=%q, BotID=%d", name, price, b.Id)

		resMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Yangi VIP tarif qo'shildi:\n💎 %s", newLine))
		resMsg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
		bot.Send(resMsg)
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
				sendUserBot(bot, chatID, "❌ Nom bo'sh bo'lishi mumkin emas.")
				return
			}

			mu.Lock()
			quickKinoTemp[userID] = name
			adminState[userID] = fmt.Sprintf("wait_vip_edit_price:%d", idx)
			mu.Unlock()

			log.Printf("🟢 [wait_vip_edit_name] state='wait_vip_edit_price:%d' ga o'tdi. name=%q", idx, name)

			priceMsg := tgbotapi.NewMessage(chatID, "💰 Endi yangi narxni yuboring:")
			priceMsg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
			bot.Send(priceMsg)
			return
		}

		if strings.HasPrefix(state, "wait_vip_edit_price:") {
			idxStr := strings.TrimPrefix(state, "wait_vip_edit_price:")
			idx, _ := strconv.Atoi(idxStr)
			log.Printf("🟣 [wait_vip_edit_price] idx=%d", idx)

			price := strings.TrimSpace(msg.Text)
			if price == "" {
				log.Printf("🔴 [wait_vip_edit_price] narx bo'sh")
				sendUserBot(bot, chatID, "❌ Narx bo'sh bo'lishi mumkin emas.")
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
				sendUserBot(bot, chatID, "❌ Ma'lumotni o'qishda xato.")
				return
			}

			lines := parseVipLines(createdBot.VipPrices)
			log.Printf("🔵 [wait_vip_edit_price] jami tariflar soni: %d, tahrirlanayotgan idx=%d", len(lines), idx)

			if idx < 0 || idx >= len(lines) {
				log.Printf("🔴 [wait_vip_edit_price] idx=%d chegaradan tashqarida (jami=%d)", idx, len(lines))
				sendUserBot(bot, chatID, "❌ Bu tarif topilmadi (o'chirilgan bo'lishi mumkin).")
				return
			}

			lines[idx] = fmt.Sprintf("%s - %s", name, price)
			createdBot.VipPrices = strings.Join(lines, "\n") + "\n"

			if _, err := o.Update(&createdBot, "VipPrices"); err != nil {
				log.Printf("🔴 [wait_vip_edit_price] saqlashda xatolik: %v", err)
				sendUserBot(bot, chatID, "❌ Saqlashda xato yuz berdi.")
				return
			}

			log.Printf("✅ [wait_vip_edit_price] tarif yangilandi: idx=%d, name=%q, price=%q", idx, name, price)

			resMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Tarif yangilandi:\n💎 %s - %s", name, price))
			resMsg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
			bot.Send(resMsg)
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

func RouteVipPaymentState(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message, state string) bool {
	userID := msg.From.ID
	chatID := msg.Chat.ID

	// --- Admin karta kiritmoqda ---
	if state == "waiting_vip_card" {
		if !isAdmin(b, userID) {
			return false
		}

		card := strings.TrimSpace(msg.Text)
		if card == "" {
			sendUserBot(bot, chatID, "❌ Iltimos, karta ma'lumotini matn ko'rinishida yuboring:")
			return true
		}

		o := orm.NewOrm()
		if _, err := o.QueryTable(new(models.CreatedBot)).
			Filter("Id", b.Id).
			Update(orm.Params{"Card": card}); err != nil {
			sendUserBot(bot, chatID, "❌ Saqlashda xatolik: "+err.Error())
			return true
		}
		b.Card = card

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()

		sendUserBot(bot, chatID, "✅ Karta saqlandi!")
		return true
	}

	// --- Foydalanuvchi chek yubormoqda ---
	if strings.HasPrefix(state, "waiting_vip_check:") {
		if len(msg.Photo) == 0 {
			sendUserBot(bot, chatID, "❌ Iltimos, chekni rasm ko'rinishida yuboring:")
			return true
		}

		idx, _ := strconv.Atoi(strings.TrimPrefix(state, "waiting_vip_check:"))

		o := orm.NewOrm()
		fresh := models.CreatedBot{Id: b.Id}
		if err := o.Read(&fresh); err != nil {
			sendUserBot(bot, chatID, "❌ Xatolik yuz berdi.")
			return true
		}
		o.LoadRelated(&fresh, "Owner")

		tariff := "—"
		if lines := parseVipLines(fresh.VipPrices); idx >= 0 && idx < len(lines) {
			tariff = strings.TrimSpace(lines[idx])
		}

		if fresh.Owner == nil || fresh.Owner.TgId == 0 {
			sendUserBot(bot, chatID, "❌ Admin topilmadi, keyinroq urinib ko'ring.")
			return true
		}

		caption := fmt.Sprintf("💳 Yangi to'lov cheki\n\n👤 %s %s\n🆔 ID: %d\n💎 Tarif: %s\n\nTo'lov qilinganmi?",
			msg.From.FirstName, msg.From.LastName, userID, tariff)

		photo := tgbotapi.NewPhoto(fresh.Owner.TgId, tgbotapi.FileID(msg.Photo[len(msg.Photo)-1].FileID))
		photo.Caption = caption
		photo.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ Ha", fmt.Sprintf("vip_pay_ok:%d:%d", userID, idx)),
				tgbotapi.NewInlineKeyboardButtonData("❌ Yo'q", fmt.Sprintf("vip_pay_no:%d:%d", userID, idx)),
			),
		)

		if _, err := bot.Send(photo); err != nil {
			log.Printf("Chekni adminga yuborishda xatolik: %v", err)
			sendUserBot(bot, chatID, "❌ Chekni yuborib bo'lmadi, keyinroq urinib ko'ring.")
			return true
		}

		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()

		sendUserBot(bot, chatID, "✅ Chek adminga yuborildi. Tasdiqlanishini kuting ⏳")
		return true
	}

	return false
}

func sendVipPricesInfoPro(bot *tgbotapi.BotAPI, chatID int64, botID int64) error {
	o := orm.NewOrm()
	createdBot := models.CreatedBot{Id: botID}

	if err := o.Read(&createdBot); err != nil {
		sendUserBot(bot, chatID, "❌ Ma'lumot topilmadi!")
		return err
	}

	pricesText := "💎 VIP Obuna Tariflari\n\nKerakli tarifni tanlang 👇"
	if createdBot.Note != "" {
		pricesText += fmt.Sprintf("\n\n📌 Eslatma: %s", createdBot.Note)
	}

	var rows [][]tgbotapi.InlineKeyboardButton

	// Har bir tarif alohida tugma
	for i, line := range parseVipLines(createdBot.VipPrices) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💎 "+line, fmt.Sprintf("vip_buy:%d", i)),
		))
	}

	if len(rows) == 0 {
		pricesText = "💎 Hozircha VIP tariflar qo'shilmagan."
	}

	// Faqat admin ko'radi
	if isAdmin(&createdBot, chatID) {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💳 Karta qo'shish", "vip_card_add"),
		))
	}

	msg := tgbotapi.NewMessage(chatID, pricesText)
	if len(rows) > 0 {
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	}

	if _, err := bot.Send(msg); err != nil {
		log.Printf("🔴 sendVipPricesInfoPro xatolik: %v", err)
		return err
	}
	return nil
}

func HandleVipPricesCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, botID int64) {
	if callback.Message == nil {
		bot.Request(tgbotapi.NewCallback(callback.ID, "❌ Xabar topilmadi."))
		return
	}

	_ = sendVipPricesInfoPro(bot, callback.Message.Chat.ID, botID)

	bot.Request(tgbotapi.NewCallback(callback.ID, ""))
}

func HandleVipCommand(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
	_ = sendVipPricesInfoPro(bot, msg.Chat.ID, b.Id)
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

func showChannelsListPro(bot *tgbotapi.BotAPI, b *models.CreatedBot, chatID int64) {
	o := orm.NewOrm()
	var channels []models.BotChannel

	_, err := o.QueryTable(new(models.BotChannel)).
		Filter("Bot__Id", b.Id).
		Filter("IsActive", true).
		OrderBy("Id").
		All(&channels)

	if err != nil || len(channels) == 0 {
		sendUserBot(bot, chatID, "📭 Hozircha hech qanday majburiy obuna kanali qo'shilmagan.")
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 <b>Majburiy obuna kanallari</b> (%d ta):\n\n", len(channels)))

	for i, ch := range channels {
		sb.WriteString(fmt.Sprintf(
			"<b>%d.</b> 🆔 ID: <code>%d</code>\n🔗 Link: %s\n\n",
			i+1, ch.ChannelID, ch.InviteLink,
		))
	}

	msg := tgbotapi.NewMessage(chatID, sb.String())
	msg.ParseMode = "HTML"

	if _, sendErr := bot.Send(msg); sendErr != nil {
		log.Printf("🔴 showChannelsListPro: xabar yuborishda xatolik: %v", sendErr)
	}
}
