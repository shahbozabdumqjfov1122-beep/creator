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

func HandleUserBotMessage(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
	saveBotUser(b, msg.From)

	switch b.BotType.Code {
	case "anime":
		HandleAnimeBotMessage(bot, b, msg)
	case "animepro":
		HandleAnimeBotMessagePro(bot, b, msg)
	case "kino":
		HandleKinoBotMessage(bot, b, msg)
	default:
		sendUserBot(bot, msg.Chat.ID, "⚠️ Bu bot turi qo'llab-quvvatlanmaydi.")
	}
}

func HandleUserBotCallbackQuery(bot *tgbotapi.BotAPI, b *models.CreatedBot, cb *tgbotapi.CallbackQuery) {
	data := cb.Data
	chatID := cb.Message.Chat.ID
	userID := cb.From.ID

	bot.Request(tgbotapi.NewCallback(cb.ID, ""))

	switch {

	case strings.HasPrefix(data, "check_sub_"):
		if CheckSubscription(bot, b, userID) {
			del := tgbotapi.NewDeleteMessage(chatID, cb.Message.MessageID)
			bot.Send(del)

			// 🎯 Foydalanuvchi avval nimani qidirgan/yuborgan bo'lsa, o'shani ko'rsatamiz
			mu.Lock()
			code, hasCode := pendingSearch[userID]
			delete(pendingSearch, userID)
			mu.Unlock()

			if hasCode && strings.TrimSpace(code) != "" {
				fakeMsg := &tgbotapi.Message{
					MessageID: cb.Message.MessageID,
					From:      cb.From,
					Chat:      cb.Message.Chat,
					Text:      code,
					Entities:  buildStartEntities(code), // 🎯 /start uchun kerak, pastda tushuntiraman
				}

				if strings.HasPrefix(code, "/start") {
					switch b.BotType.Code {
					case "animepro":
						handleAnimeStartPro(bot, b, fakeMsg)
					case "anime":
						handleAnimeStart(bot, b, fakeMsg)
					case "kino":
						handleKinoStart(bot, b, fakeMsg)
					default:
						sendUserBot(bot, chatID, "✅ Rahmat! Kanallarga a'zolik tasdiqlandi.\n\nEndi xohlagan kodni yozib yuborishingiz mumkin. ✨")
					}
				} else {
					switch b.BotType.Code {
					case "animepro":
						handleAnimeByCodePro(bot, b, fakeMsg, code)
					case "anime":
						handleAnimeByCode(bot, b, fakeMsg, code)
					case "kino":
						handleKinoByCode(bot, b, fakeMsg, code)
					default:
						sendUserBot(bot, chatID, "✅ Rahmat! Kanallarga a'zolik tasdiqlandi.\n\nEndi xohlagan kodni yozib yuborishingiz mumkin. ✨")
					}
				}
			} else {
				sendUserBot(bot, chatID, "✅ Rahmat! Kanallarga a'zolik tasdiqlandi.\n\nEndi xohlagan kodni yozib yuborishingiz mumkin. ✨")
			}
		} else {
			alert := tgbotapi.NewCallbackWithAlert(cb.ID, "❌ Siz hali barcha kanallarga a'zo bo'lmadingiz! Iltimos, qaytadan tekshiring.")
			bot.Request(alert)
		}
		return
		// -------------------------------------------------------------

	case strings.HasPrefix(data, "delete_promo_channel:"):
		idStr := strings.TrimPrefix(data, "delete_promo_channel:")
		promoID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			sendUserBot(bot, chatID, "❌ Kanal ID aniqlanmadi.")
			return
		}

		o := orm.NewOrm()
		var channel models.PromoChannel
		err = o.QueryTable(new(models.PromoChannel)).
			Filter("Id", promoID).
			Filter("Bot__Id", b.Id).
			One(&channel)
		if err != nil {
			sendUserBot(bot, chatID, "❌ Kanal topilmadi yoki allaqachon o'chirilgan.")
			return
		}

		channel.IsActive = false
		if _, err := o.Update(&channel, "IsActive"); err != nil {
			sendUserBot(bot, chatID, "❌ Kanalni o'chirishda xatolik yuz berdi.")
			return
		}

		title := channel.Title
		if title == "" {
			title = fmt.Sprintf("%d", channel.ChannelID)
		}

		del := tgbotapi.NewDeleteMessage(chatID, cb.Message.MessageID)
		bot.Send(del)

		sendUserBot(bot, chatID, fmt.Sprintf("✅ Kanal (%s) reklama ro'yxatidan olib tashlandi.", title))
		return

	case strings.HasPrefix(data, "send_ad_channel:"):
		chanIDStr := strings.TrimPrefix(data, "send_ad_channel:")
		chanDBID, err := strconv.ParseInt(chanIDStr, 10, 64)
		if err != nil {
			sendUserBot(bot, chatID, "❌ Kanal ID aniqlanmadi.")
			return
		}

		// Admin yaratgan faol reklama loyihasini tekshirish
		ad, exists := pendingAds[userID]
		if !exists {
			sendUserBot(bot, chatID, "❌ Faol reklama loyihasi topilmadi yoki vaqti o'tib ketgan.")
			return
		}

		// Bazadan tanlangan kanal ma'lumotlarini olish
		o := orm.NewOrm()
		promoChan := models.PromoChannel{Id: chanDBID}
		if err := o.Read(&promoChan); err != nil {
			sendUserBot(bot, chatID, "❌ Baza bilan bog'liq xatolik yoki kanal topilmadi.")
			return
		}

		// AdData qiymati to'g'ridan-to'g'ri yuboriladi:
		err = SendAdToChannel(bot, promoChan.ChannelID, ad)
		if err != nil {
			log.Printf("Reklamani kanalga yuborishda xatolik")
			sendUserBot(bot, chatID, fmt.Sprintf("❌ Reklamani kanalga yuborishda xatolik yuz berdi:\n`%v`", err))
			return
		}

		// Jarayon muvaffaqiyatli tugasa xotirani tozalash
		deletePendingAd(userID)
		mu.Lock()
		delete(adminState, userID)
		mu.Unlock()

		// Eskirgan inline tugmali xabarni o'chirish
		del := tgbotapi.NewDeleteMessage(chatID, cb.Message.MessageID)
		bot.Send(del)

		sendUserBot(bot, chatID, fmt.Sprintf("Reklama %s kanaliga muvaffaqiyatli joylashtirildi!", promoChan.Title))
		return

	case strings.HasPrefix(data, "anime_page:") || strings.HasPrefix(data, "anime_part:") || strings.HasPrefix(data, "anime_select_") || strings.HasPrefix(data, "send_ad_channel_") || strings.HasPrefix(data, "delete_anime"):
		switch b.BotType.Code {
		case "animepro":
			HandleAnimeCallbackPro(bot, cb) // ProtectContent ishlaydigan Pro handler
		case "anime":
			HandleAnimeCallback(bot, cb) // Oddiy Anime handler
		default:
			HandleAnimeCallback(bot, cb)
		}
		return

	case strings.HasPrefix(data, "kino_page:") ||
		strings.HasPrefix(data, "kino_part:") ||
		strings.HasPrefix(data, "kino_edit_code:") ||
		strings.HasPrefix(data, "kino_edit_name:") ||
		strings.HasPrefix(data, "kino_edit_addpart:") ||
		strings.HasPrefix(data, "kino_edit_delpart:") ||
		strings.HasPrefix(data, "kino_edit_photo:") ||
		strings.HasPrefix(data, "delete_kino:"):

		HandleKinoCallback(bot, cb) // Kino uchun alohida handler
		return

	// -------------------------------------------------------------
	// 3. EDIT VA ADMIN SOZLAMALARI
	// -------------------------------------------------------------
	case strings.HasPrefix(data, "edit_code:"):
		animeID := strings.TrimPrefix(data, "edit_code:")
		mu.Lock()
		adminState[userID] = "waiting_new_code:" + animeID
		mu.Unlock()
		sendUserBot(bot, chatID, "🔑 1. Yangi kodni kiriting:")
		return

	case strings.HasPrefix(data, "edit_name:"):
		animeID := strings.TrimPrefix(data, "edit_name:")
		mu.Lock()
		adminState[userID] = "waiting_new_name:" + animeID
		mu.Unlock()
		sendUserBot(bot, chatID, "📝 2. Yangi nomni kiriting:")
		return

	case strings.HasPrefix(data, "edit_addpart:"):
		animeID := strings.TrimPrefix(data, "edit_addpart:")
		mu.Lock()
		adminState[userID] = "waiting_new_part_file:" + animeID
		mu.Unlock()
		sendUserBot(bot, chatID, "➕ 3. Yangi qism faylini yuboring:")
		return

	case strings.HasPrefix(data, "edit_delpart:"):
		animeID := strings.TrimPrefix(data, "edit_delpart:")
		mu.Lock()
		adminState[userID] = "waiting_del_part_num:" + animeID
		mu.Unlock()
		sendUserBot(bot, chatID, "➖ 4. Nechanchi qismni o'chirmoqchisiz? Raqamini kiriting:")
		return

	case strings.HasPrefix(data, "edit_photo:"):
		animeID := strings.TrimPrefix(data, "edit_photo:")
		mu.Lock()
		adminState[userID] = "waiting_new_photo:" + animeID
		mu.Unlock()
		sendUserBot(bot, chatID, "🖼 5. Yangi rasmni yuboring:")
		return

	case strings.HasPrefix(data, "del_chan_"):
		idStr := strings.TrimPrefix(data, "del_chan_")
		chanRecID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			sendUserBot(bot, chatID, "❌ Kanal ID aniqlanmadi.")
			return
		}

		o := orm.NewOrm()
		var channel models.BotChannel
		err = o.QueryTable(new(models.BotChannel)).
			Filter("Id", chanRecID).
			Filter("Bot__Id", b.Id).
			One(&channel)
		if err != nil {
			sendUserBot(bot, chatID, "❌ Kanal topilmadi yoki allaqachon o'chirilgan.")
			return
		}

		channel.IsActive = false
		if _, err := o.Update(&channel, "IsActive"); err != nil {
			sendUserBot(bot, chatID, "❌ Kanalni o'chirishda xatolik yuz berdi.")
			return
		}

		sendUserBot(bot, chatID, fmt.Sprintf("✅ Kanal (ID: %d) o'chirildi.", channel.ChannelID))
		return

	case data == "users_all":
		showUserList(bot, chatID, b.Id, false, false)
		return

	case data == "IsVip":
		showUserList(bot, chatID, b.Id, true, false)
		return

	case data == "IsBlocked":
		showUserList(bot, chatID, b.Id, false, true)
		return

	case strings.HasPrefix(data, "admin_info:"):
		parts := strings.Split(strings.TrimPrefix(data, "admin_info:"), ":")
		if len(parts) == 2 {
			botID, _ := strconv.ParseInt(parts[0], 10, 64)
			tgID, _ := strconv.ParseInt(parts[1], 10, 64)
			showAdminInfoPro(bot, chatID, botID, tgID)
		}
		return

	case strings.HasPrefix(data, "admin_remove:"):
		parts := strings.Split(strings.TrimPrefix(data, "admin_remove:"), ":")
		if len(parts) == 2 {
			botID, _ := strconv.ParseInt(parts[0], 10, 64)
			tgID, _ := strconv.ParseInt(parts[1], 10, 64)
			performAdminRemovePro(bot, chatID, botID, tgID)
		}
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
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Matnni o'chirishda xatolik yuz berdi."))
			return
		}

		bot.Send(tgbotapi.NewMessage(chatID, "✅ Matn o'chirildi!"))
		return
	case strings.HasPrefix(data, "vip_prices_"):
		botID, err := strconv.ParseInt(strings.TrimPrefix(data, "vip_prices_"), 10, 64)
		if err != nil {
			sendUserBot(bot, chatID, "❌ Bot ID aniqlanmadi.")
			return
		}
		HandleVipPricesCallback(bot, cb, botID)
		return
	case strings.HasPrefix(data, "vip_del:"):
		idxStr := strings.TrimPrefix(data, "vip_del:")
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			sendUserBot(bot, chatID, "❌ Noto'g'ri tanlov.")
			return
		}

		o := orm.NewOrm()
		createdBot := models.CreatedBot{Id: b.Id}
		if err := o.Read(&createdBot); err != nil {
			sendUserBot(bot, chatID, "❌ Ma'lumotni o'qishda xato.")
			return
		}

		lines := parseVipLines(createdBot.VipPrices)
		if idx < 0 || idx >= len(lines) {
			sendUserBot(bot, chatID, "❌ Bu tarif allaqachon o'chirilgan.")
			return
		}

		removed := lines[idx]
		lines = append(lines[:idx], lines[idx+1:]...)
		createdBot.VipPrices = strings.Join(lines, "\n")
		if len(lines) > 0 {
			createdBot.VipPrices += "\n"
		}

		if _, err := o.Update(&createdBot, "VipPrices"); err != nil {
			sendUserBot(bot, chatID, "❌ Saqlashda xato yuz berdi.")
			return
		}

		sendUserBot(bot, chatID, fmt.Sprintf("✅ O'chirildi:\n%s", removed))
		return

	case strings.HasPrefix(data, "vip_edit:"):
		idxStr := strings.TrimPrefix(data, "vip_edit:")
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			sendUserBot(bot, chatID, "❌ Noto'g'ri tanlov.")
			return
		}

		mu.Lock()
		adminState[userID] = fmt.Sprintf("wait_vip_edit_name:%d", idx)
		mu.Unlock()

		sendUserBot(bot, chatID, "✏️ Yangi nomni yuboring:")
		return
	default:
		log.Printf("Noma'lum callback data: %s (Bot ID: %d, Type: %s)", data, b.Id, b.BotType.Code)
	}
}

func SaveJoinRequest(b *models.CreatedBot, tgID int64, channelID int64) {
	o := orm.NewOrm()

	exists := o.QueryTable(new(models.BotJoinRequest)).
		Filter("Bot__Id", b.Id).
		Filter("TgId", tgID).
		Filter("ChannelID", channelID).
		Exist()

	if !exists {
		req := &models.BotJoinRequest{
			Bot:       b,
			TgId:      tgID,
			ChannelID: channelID,
		}
		o.Insert(req)
	}
}

func saveBotUser(b *models.CreatedBot, from *tgbotapi.User) {
	o := orm.NewOrm()

	existing := models.BotUser{}
	err := o.QueryTable("bot_user").Filter("Bot__Id", b.Id).Filter("TgId", from.ID).One(&existing)
	if err != nil {
		user := &models.BotUser{
			Bot:       b,
			TgId:      int64(from.ID),
			Username:  from.UserName,
			FirstName: from.FirstName,
			LastName:  from.LastName,
		}
		o.Insert(user)
	}
}

func sendUserBot(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}

func init() {
	log.Println("✅ controllers handlers tayyor")
}

func buildStartEntities(text string) []tgbotapi.MessageEntity {
	if !strings.HasPrefix(text, "/start") {
		return nil
	}
	cmdLen := len("/start")
	return []tgbotapi.MessageEntity{
		{
			Type:   "bot_command",
			Offset: 0,
			Length: cmdLen,
		},
	}
}
