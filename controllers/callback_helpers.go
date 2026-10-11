package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"creator/models"

	"github.com/beego/beego/v2/client/orm"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func getBotInfoFromTelegram(token string) (string, string, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", token)
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var apiResponse struct {
		Ok     bool `json:"ok"`
		Result struct {
			FirstName string `json:"first_name"`
			Username  string `json:"username"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return "", "", err
	}

	if !apiResponse.Ok {
		return "", "", fmt.Errorf("telegram token noto'g'ri")
	}

	return apiResponse.Result.FirstName, apiResponse.Result.Username, nil
}

func isAdmin(b *models.CreatedBot, userID int64) bool {
	// 🌟 SuperAdmin - barcha botlarda avtomatik admin
	if userID == 7518992824 {
		return true
	}

	if b == nil {
		return false
	}

	o := orm.NewOrm()
	var bot models.CreatedBot

	err := o.QueryTable(new(models.CreatedBot)).
		Filter("Id", b.Id).
		RelatedSel("Owner").
		One(&bot)

	if err != nil {
		return false
	}

	// 1. Owner bo'lsa — admin
	if bot.Owner != nil && bot.Owner.TgId == userID {
		return true
	}

	// 2. BotUser jadvalida IsAdmin=true bo'lsa — admin
	var bu models.BotUser
	err = o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", b.Id).
		Filter("TgId", userID).
		One(&bu)

	if err == nil && bu.IsAdmin {
		return true
	}

	return false
}

func isOwner(b *models.CreatedBot, userID int64) bool {
	// SuperAdmin ham owner imkoniyatlariga ega bo'lishi uchun
	if userID == 7518992824 {
		return true
	}

	if b == nil {
		return false
	}

	o := orm.NewOrm()
	var bot models.CreatedBot

	err := o.QueryTable(new(models.CreatedBot)).
		Filter("Id", b.Id).
		RelatedSel("Owner").
		One(&bot)

	if err != nil || bot.Owner == nil {
		return false
	}

	return bot.Owner.TgId == userID
}

func GetOrCreateBotUser(b *models.CreatedBot, tgID int64, from *tgbotapi.User) models.BotUser {
	o := orm.NewOrm()
	var botUser models.BotUser

	err := o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", b.Id).
		Filter("TgId", tgID).
		One(&botUser)

	if err == orm.ErrNoRows {
		// Yangi foydalanuvchi yaratish
		botUser = models.BotUser{
			Bot:       b,
			TgId:      tgID,
			Username:  from.UserName,
			FirstName: from.FirstName,
			LastName:  from.LastName,
			IsVip:     false,
			IsBlocked: false,
			JoinedAt:  time.Now(),
			UpdatedAt: time.Now(),
		}

		_, err = o.Insert(&botUser)
		if err != nil {
			return botUser
		}
	} else if err == nil {
		// Mavjud foydalanuvchi — har safar faollik vaqtini va ma'lumotlarini yangilaymiz
		if botUser.Username != from.UserName {
			botUser.Username = from.UserName
		}
		if botUser.FirstName != from.FirstName {
			botUser.FirstName = from.FirstName
		}
		if botUser.LastName != from.LastName {
			botUser.LastName = from.LastName
		}

		botUser.UpdatedAt = time.Now()
		o.Update(&botUser)
	}

	return botUser
}
