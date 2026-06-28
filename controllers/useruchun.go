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

	// Telegram'ga yuklanish belgisi yo'qolishi uchun javob qaytaramiz
	bot.Request(tgbotapi.NewCallback(cb.ID, ""))

	switch {

	case strings.HasPrefix(data, "check_sub_"):
		// Obunani tekshiramiz
		if CheckSubscription(bot, b, userID) {
			del := tgbotapi.NewDeleteMessage(chatID, cb.Message.MessageID)
			bot.Send(del)
			sendUserBot(bot, chatID, "✅ *Rahmat! Kanallarga a'zolik tasdiqlandi.*\n\nEndi xohlagan anime **kodini** yozib yuborishingiz mumkin. ✨")
		} else {
			alert := tgbotapi.NewCallbackWithAlert(cb.ID, "❌ Siz hali barcha kanallarga a'zo bo'lmadingiz! Iltimos, qaytadan tekshiring.")
			bot.Request(alert)
		}
		return

	case strings.HasPrefix(data, "anime_page:") || strings.HasPrefix(data, "anime_part:"):
		HandleAnimeCallback(bot, cb)
		return

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
		sendUserBot(bot, chatID, "➕ 3. Yangi qism faylini (Video, Dokument yoki Rasm) yuboring:")
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

	case data == "anime_settings":
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
			log.Printf("Kanalni o'chirishda xatolik: %v", err)
			sendUserBot(bot, chatID, "❌ Kanalni o'chirishda xatolik yuz berdi.")
			return
		}

		log.Printf("🗑️ Kanal o'chirildi (IsActive=false): RecID: %d, ChannelID: %d, Bot ID: %d", channel.Id, channel.ChannelID, b.Id)
		sendUserBot(bot, chatID, fmt.Sprintf("✅ Kanal (ID: %d) majburiy obuna ro'yxatidan olib tashlandi.", channel.ChannelID))
		return

	case strings.HasPrefix(data, "check_sub_"):
		// Obunani tekshiramiz
		if CheckSubscription(bot, b, userID) {
			del := tgbotapi.NewDeleteMessage(chatID, cb.Message.MessageID)
			bot.Send(del)
			sendUserBot(bot, chatID, "✅ *Rahmat! Kanallarga a'zolik tasdiqlandi.*\n\nEndi xohlagan anime **kodini** yozib yuborishingiz mumkin. ✨")
		} else {
			alert := tgbotapi.NewCallbackWithAlert(cb.ID, "❌ Siz hali barcha kanallarga a'zo bo'lmadingiz! Iltimos, qaytadan tekshiring.")
			bot.Request(alert)
		}
		return

	case strings.HasPrefix(data, "anime_page:") || strings.HasPrefix(data, "anime_part:"):
		HandleAnimeCallback(bot, cb)
		return

	case strings.HasPrefix(data, "kino_page:") ||
		strings.HasPrefix(data, "kino_part:") ||
		strings.HasPrefix(data, "kino_edit_code:") ||
		strings.HasPrefix(data, "kino_edit_name:") ||
		strings.HasPrefix(data, "kino_edit_addpart:") ||
		strings.HasPrefix(data, "kino_edit_delpart:") ||
		strings.HasPrefix(data, "kino_edit_photo:") ||
		strings.HasPrefix(data, "delete_kino:"):
		HandleKinoCallback(bot, cb)
		return

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
		sendUserBot(bot, chatID, "➕ 3. Yangi qism faylini (Video, Dokument yoki Rasm) yuboring:")
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
			log.Printf("Kanalni o'chirishda xatolik: %v", err)
			sendUserBot(bot, chatID, "❌ Kanalni o'chirishda xatolik yuz berdi.")
			return
		}

		log.Printf("🗑️ Kanal o'chirildi (IsActive=false): RecID: %d, ChannelID: %d, Bot ID: %d", channel.Id, channel.ChannelID, b.Id)
		sendUserBot(bot, chatID, fmt.Sprintf("✅ Kanal (ID: %d) majburiy obuna ro'yxatidan olib tashlandi.", channel.ChannelID))
		return

	case data == "users_all":
		showUserList(bot, chatID, b.Id, false, false)
		return

	case data == "IsVip":
		// VIP ro'yxatni chiqarish
		showUserList(bot, chatID, b.Id, true, false)
		return

	case data == "IsBlocked":
		// Bloklanganlar ro'yxatini chiqarish
		showUserList(bot, chatID, b.Id, false, true)
		return

	default:
		log.Printf("Noma'lum anime callback data: %s (Bot ID: %d)", data, b.Id)
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
