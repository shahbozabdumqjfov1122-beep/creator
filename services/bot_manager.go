package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"creator/models"

	"github.com/beego/beego/v2/client/orm"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type MessageHandler func(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message)
type CallbackHandler func(bot *tgbotapi.BotAPI, b *models.CreatedBot, cb *tgbotapi.CallbackQuery)
type JoinRequestHandler func(b *models.CreatedBot, tgID int64, channelID int64)

var (
	OnMessage     MessageHandler
	OnCallback    CallbackHandler
	OnJoinRequest JoinRequestHandler
)

var (
	RunningBots    = make(map[int64]context.CancelFunc)
	RunningBotAPIs = make(map[int64]*tgbotapi.BotAPI)
	mu             sync.Mutex
	CreatorBot     *tgbotapi.BotAPI
)

func StartBot(b *models.CreatedBot) {
	mu.Lock()
	defer mu.Unlock()

	if cancel, exists := RunningBots[b.Id]; exists {
		cancel()

		if oldBot, ok := RunningBotAPIs[b.Id]; ok {
			oldBot.StopReceivingUpdates() // 🔑 shu qator yo'q edi
		}

		delete(RunningBots, b.Id)
		delete(RunningBotAPIs, b.Id)
	}

	bot, err := tgbotapi.NewBotAPI(b.Token)
	if err != nil {
		log.Printf("❌ Bot ishga tushmadi @%s: %v", b.BotUsername, err)
		return
	}

	_, werr := bot.Request(tgbotapi.DeleteWebhookConfig{DropPendingUpdates: true})
	if werr != nil {
		log.Printf("⚠️ Webhookni o'chirishda xatolik @%s: %v", b.BotUsername, werr)
	}

	ctx, cancel := context.WithCancel(context.Background())
	RunningBots[b.Id] = cancel
	RunningBotAPIs[b.Id] = bot

	go runBotLoop(ctx, bot, b)
	log.Printf("✅ Bot ishga tushdi: @%s", b.BotUsername)
}

func StopBot(botId int64) {
	mu.Lock()
	defer mu.Unlock()

	if cancel, exists := RunningBots[botId]; exists {
		cancel()
		delete(RunningBots, botId)
	}
	if api, exists := RunningBotAPIs[botId]; exists {
		api.StopReceivingUpdates()
		delete(RunningBotAPIs, botId)
	}
	log.Printf("🔴 Bot #%d to'xtatildi", botId)

}

func IsBotRunning(botId int64) bool {
	mu.Lock()
	defer mu.Unlock()
	_, exists := RunningBots[botId]
	return exists
}

func StartAllBots() {
	o := orm.NewOrm()
	var bots []models.CreatedBot
	o.QueryTable("created_bot").
		Filter("IsActive", true).
		Filter("IsSuspended", false).
		RelatedSel("BotType").
		RelatedSel("Owner").
		All(&bots)

	log.Printf("🚀 %d ta bot ishga tushirilmoqda...", len(bots))
	for i := range bots {
		go StartBot(&bots[i])
	}
}

func runBotLoop(ctx context.Context, bot *tgbotapi.BotAPI, b *models.CreatedBot) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("🛑 Bot @%s to'xtatildi\n", b.BotUsername)
			return
		case update, ok := <-updates:
			if !ok {
				return
			}
			if b.IsSuspended {
				continue
			}
			if update.Message != nil && OnMessage != nil {
				OnMessage(bot, b, update.Message)
			}
			if update.CallbackQuery != nil && OnCallback != nil {
				OnCallback(bot, b, update.CallbackQuery)
			}
			if update.ChatJoinRequest != nil && OnJoinRequest != nil {
				OnJoinRequest(b, update.ChatJoinRequest.From.ID, update.ChatJoinRequest.Chat.ID)
			}
		}
	}
}

const DailyPrice = 1500.0

func StartDailyBillingScheduler() {
	go func() {
		for {
			now := time.Now()
			nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			time.Sleep(time.Until(nextMidnight))
			processDailyBilling()
		}
	}()
	log.Println("💰 Kunlik billing scheduler ishga tushdi")
}

func processDailyBilling() {
	o := orm.NewOrm()
	now := time.Now()
	var bots []models.CreatedBot
	o.QueryTable("created_bot").
		Filter("IsActive", true).
		Filter("TrialEndsAt__lt", now).
		RelatedSel("Owner").
		All(&bots)
	for i := range bots {
		chargeBot(o, &bots[i])
	}
}

func chargeBot(o orm.Ormer, b *models.CreatedBot) {
	var owner models.UserBot
	if err := o.QueryTable("user_bot").Filter("Id", b.Owner.Id).One(&owner); err != nil {
		return
	}
	if owner.Balance >= DailyPrice {
		owner.Balance -= DailyPrice
		o.Update(&owner, "Balance")
		b.IsSuspended = false
		b.PaidUntil = time.Now().Add(24 * time.Hour)
		o.Update(b, "IsSuspended", "PaidUntil")
		if !IsBotRunning(b.Id) {
			go StartBot(b)
			NotifyOwner(owner.TgId, b, true)
		}
	} else {
		StopBot(b.Id)
		b.IsSuspended = true
		o.Update(b, "IsSuspended")
		NotifyOwner(owner.TgId, b, false)
	}
}

func ResumeBotsAfterTopUp(ownerTgId int64) {
	o := orm.NewOrm()
	var owner models.UserBot
	if err := o.QueryTable("user_bot").Filter("TgId", ownerTgId).One(&owner); err != nil {
		return
	}
	var bots []models.CreatedBot
	o.QueryTable("created_bot").
		Filter("Owner__Id", owner.Id).
		Filter("IsSuspended", true).
		Filter("IsActive", true).
		All(&bots)
	for i := range bots {
		b := &bots[i]
		if owner.Balance >= DailyPrice {
			owner.Balance -= DailyPrice
			o.Update(&owner, "Balance")
			b.IsSuspended = false
			b.PaidUntil = time.Now().Add(24 * time.Hour)
			o.Update(b, "IsSuspended", "PaidUntil")
			go StartBot(b)
			NotifyOwner(ownerTgId, b, true)
		}
	}
}

func NotifyOwner(ownerTgId int64, b *models.CreatedBot, resumed bool) {
	if CreatorBot == nil {
		return
	}
	var text string
	if resumed {
		text = "✅ *@" + b.BotUsername + "* botingiz qayta ishga tushdi!\n💰 Hisobingizdan *1,500 so'm* yechildi."
	} else {
		text = "⚠️ *@" + b.BotUsername + "* botingiz *to'xtatildi!*\n\n" +
			"💳 Hisobingizda mablag' yetarli emas.\n" +
			"➕ Balansni to'ldiring — bot avtomatik qayta ishga tushadi."
	}
	msg := tgbotapi.NewMessage(ownerTgId, text)
	msg.ParseMode = "Markdown"
	CreatorBot.Send(msg)
}
