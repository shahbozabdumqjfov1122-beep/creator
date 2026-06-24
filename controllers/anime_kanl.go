package controllers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"creator/models"

	"github.com/beego/beego/v2/client/orm"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var adminState = map[int64]string{}
var adminTempChannel = map[int64]int64{}

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

	// 🎯 FIX: Endi ham /addchannel, ham "➕ Kanal qo‘shish" matni kelganda ishlaydi
	if msg.Text == "/addchannel" || msg.Text == "➕ Kanal qo‘shish" {
		mu.Lock()
		adminState[userID] = "wait_channel"
		mu.Unlock()

		sendUserBot(bot, msg.Chat.ID, "📢 Kanal ID yoki @username yuboring.\nYoki kanaldan birorta xabarni **Forward (pereposlat)** qiling!\n\n⚠️ Bot kanalga admin bo‘lishi shart!")
		return
	}

	mu.Lock()
	state, hasState := adminState[userID]
	mu.Unlock()

	if !hasState {
		return
	}

	switch state {
	case "wait_channel":
		channelID, err := parseChannelID(bot, msg)
		if err != nil {
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

			// Adminga boshqadan urinishi uchun panelni qaytarib ko'rsatamiz
			showAdminPanel(bot, msg.Chat.ID)
			return
		}

		// Agar hammasi to'g'ri bo'lsa, keyingi qadamga o'tadi...
		mu.Lock()
		adminTempChannel[userID] = channelID
		adminState[userID] = "wait_link"
		mu.Unlock()

		sendUserBot(bot, msg.Chat.ID, "🔗 Endi kanal uchun Invite link yuboring (https://t.me/....)")
		return

	case "wait_link":
		link := strings.TrimSpace(msg.Text)
		if link == "" || !strings.HasPrefix(link, "http") {
			sendUserBot(bot, msg.Chat.ID, "❌ Iltimos, to'g'ri havola (link) yuboring!")
			return
		}

		mu.Lock()
		channelID := adminTempChannel[userID]
		mu.Unlock()

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
			sendUserBot(bot, msg.Chat.ID, "❌ Ma'lumotlar bazasiga saqlashda xato yuz berdi.")
			return
		}

		sendUserBot(bot, msg.Chat.ID, fmt.Sprintf("✅ Kanal muvaffaqiyatli qo‘shildi!\n📢 ID: %d", channelID))

		mu.Lock()
		delete(adminState, userID)
		delete(adminTempChannel, userID)
		mu.Unlock()

		// Asosiy admin panelni qayta ko'rsatamiz
		showAdminPanel(bot, msg.Chat.ID)
		return
	}
}

func ShowMembership(bot *tgbotapi.BotAPI, b *models.CreatedBot, chatID int64) {
	o := orm.NewOrm()
	var channels []models.BotChannel

	_, err := o.QueryTable(new(models.BotChannel)).
		Filter("Bot__Id", b.Id).
		Filter("IsActive", true).
		All(&channels)

	if err != nil || len(channels) == 0 {
		return
	}

	text := "🚨 Botdan foydalanish uchun quyidagi kanallarga obuna bo‘ling:\n\n"
	var rows [][]tgbotapi.InlineKeyboardButton

	for _, ch := range channels {
		btn := tgbotapi.NewInlineKeyboardButtonURL("OBUNA BOLISH", ch.InviteLink)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
	}

	checkBtn := tgbotapi.NewInlineKeyboardButtonData("✅ Tekshirish", fmt.Sprintf("check_sub_%d", b.Id))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(checkBtn))

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	bot.Send(msg)
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
