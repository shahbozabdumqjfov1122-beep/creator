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

const DailyPrice = 1500.0
const TestDuration = 24 * time.Hour

func SetSharedCreatorBot(bot *tgbotapi.BotAPI) {
	CreatorBot = bot
}

func StartBot(b *models.CreatedBot) {
	mu.Lock()
	defer mu.Unlock()

	o := orm.NewOrm()

	// 🎯 MUHIM TUZATISH: BotType har doim to'g'ri yuklanganini kafolatlaymiz.
	// CreatedBot struct'ida alohida BotTypeId maydoni yo'q (faqat
	// `BotType *BotType` rel(fk) sifatida), shuning uchun ID'ni
	// to'g'ridan-to'g'ri Go orqali o'qib bo'lmaydi — raw SQL kerak.
	if b.BotType == nil || b.BotType.Code == "" {
		var bt models.BotType
		err := o.Raw(`
			SELECT bt.id, bt.name, bt.code, bt.description, bt.is_active
			FROM bot_type bt
			INNER JOIN created_bot cb ON cb.bot_type_id = bt.id
			WHERE cb.id = ?
		`, b.Id).QueryRow(&bt)

		if err == nil {
			b.BotType = &bt
			log.Printf("ℹ️ BotType qayta yuklandi: @%s -> %s", b.BotUsername, bt.Code)
		} else {
			log.Printf("⚠️ BotType topilmadi (bot Id=%d, @%s uchun): %v", b.Id, b.BotUsername, err)
		}
	}

	// Eski botni to'xtatish
	if cancel, exists := RunningBots[b.Id]; exists {
		cancel()
		if oldBot, ok := RunningBotAPIs[b.Id]; ok {
			oldBot.StopReceivingUpdates()
		}
		delete(RunningBots, b.Id)
		delete(RunningBotAPIs, b.Id)
	}

	bot, err := tgbotapi.NewBotAPI(b.Token)
	if err != nil {
		log.Printf("❌ Bot ishga tushmadi @%s: %v", b.BotUsername, err)
		return
	}

	_, _ = bot.Request(tgbotapi.DeleteWebhookConfig{DropPendingUpdates: true})

	ctx, cancel := context.WithCancel(context.Background())
	RunningBots[b.Id] = cancel
	RunningBotAPIs[b.Id] = bot

	// 1 KUNLIK REJIM
	b.PaidUntil = time.Now().Add(24 * time.Hour)

	o.Update(b, "PaidUntil", "IsSuspended")

	go runBotLoop(ctx, bot, b)
	log.Printf("✅ Bot ishga tushdi: @%s (1 kun)", b.BotUsername)
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

func StartFastBillingChecker() {
	go func() {
		ticker := time.NewTicker(30 * time.Second) // Har 30 sekundda tekshiradi
		defer ticker.Stop()

		for range ticker.C {
			fastBillingCheck()
		}
	}()

	log.Println("⚡ Tez billing tekshiruvi (har 30 sek) ishga tushdi")
}

func fastBillingCheck() {
	o := orm.NewOrm()
	now := time.Now()

	var activeBots []models.CreatedBot
	o.QueryTable("created_bot").
		Filter("IsActive", true).
		Filter("IsSuspended", false).
		All(&activeBots)

	for _, bot := range activeBots {
		if bot.PaidUntil.Before(now) {
			// Pul yechish vaqti keldi
			var user models.UserBot
			err := o.QueryTable("user_bot").Filter("Id", bot.Owner.Id).One(&user)
			if err != nil {
				StopBot(bot.Id)
				continue
			}

			if user.Balance >= DailyPrice {
				user.Balance -= DailyPrice
				o.Update(&user, "Balance")

				bot.PaidUntil = time.Now().Add(TestDuration)
				o.Update(&bot, "PaidUntil")

				log.Printf("💰 Bot @%s uchun 1500 so'm yechildi. Yangi muddat: +24 Hour", bot.BotUsername)
			} else {
				// Pul yetmadi → to'xtatish
				StopBot(bot.Id)
				bot.IsSuspended = true
				o.Update(&bot, "IsSuspended")
				NotifyOwner(user.TgId, &bot, false)
				log.Printf("⚠️ Bot @%s to'xtatildi (balans yetmadi)", bot.BotUsername)
			}
		}
	}
}

func StartDailyBillingScheduler() {
	// Test paytida kerak bo'lmasa, izohga oling yoki o'chirib qo'ying
	log.Println("💰 Kunlik billing scheduler hozircha o'chirilgan (test rejimi)")
}

func ResumeBotsAfterTopUp(ownerTgId int64) {
	o := orm.NewOrm()

	var owner models.UserBot
	if err := o.QueryTable("user_bot").Filter("TgId", ownerTgId).One(&owner); err != nil {
		log.Printf("❌ ResumeBots: User topilmadi %d", ownerTgId)
		return
	}

	var suspendedBots []models.CreatedBot
	_, err := o.QueryTable("created_bot").
		Filter("Owner__Id", owner.Id).
		Filter("IsActive", true).
		Filter("IsSuspended", true).
		All(&suspendedBots)

	if err != nil || len(suspendedBots) == 0 {
		log.Printf("ℹ️ ResumeBots: To'xtatilgan bot yo'q (User %d)", ownerTgId)
		return
	}

	log.Printf("🔄 %d ta to'xtatilgan bot topildi. Balans: %.0f so'm", len(suspendedBots), owner.Balance)

	resumed := 0
	for i := range suspendedBots {
		b := &suspendedBots[i]

		if owner.Balance >= DailyPrice {
			owner.Balance -= DailyPrice
			b.IsSuspended = false
			b.PaidUntil = time.Now().Add(24 * time.Hour) // 1 kun

			o.Update(&owner, "Balance")
			o.Update(b, "IsSuspended", "PaidUntil")

			go StartBot(b)
			NotifyOwner(ownerTgId, b, true)

			resumed++
			log.Printf("✅ Bot #%d (@%s) avtomatik ishga tushdi", b.Id, b.BotUsername)
		} else {
			log.Printf("⛔ Balans yetarli emas, qolgan botlar ochilmaydi")
			break
		}
	}

	if resumed == 0 {
		log.Printf("⚠️ Hech qanday bot ochilmadi (User %d)", ownerTgId)
	}
}

func NotifyOwner(ownerTgId int64, b *models.CreatedBot, resumed bool) {
	if CreatorBot == nil {
		log.Printf("⚠️ NotifyOwner: CreatorBot nil!")
		return
	}

	var text string
	if resumed {
		text = "✅ *@" + b.BotUsername + "* botingiz qayta ishga tushdi!\n💰 Hisobingizdan *1,500 so'm* yechildi."
	} else {
		text = "⚠️ *@" + b.BotUsername + "* botingiz *to'xtatildi!*\n\n💳 Hisobingizda mablag' yetarli emas.\n➕ Balansni to'ldiring."
	}

	msg := tgbotapi.NewMessage(ownerTgId, text)
	msg.ParseMode = "Markdown"
	CreatorBot.Send(msg)
}

func RestoreActiveBots() {
	o := orm.NewOrm()
	now := time.Now()

	var activeBots []models.CreatedBot
	_, err := o.QueryTable("created_bot").
		Filter("IsActive", true).
		Filter("IsSuspended", false).
		All(&activeBots) // ⬅️ bu yerda RelatedSel("BotType") yo'q!

	if err != nil {
		log.Printf("❌ RestoreActiveBots: so'rovda xatolik: %v", err)
		return
	}

	if len(activeBots) == 0 {
		log.Println("ℹ️ RestoreActiveBots: qayta yoqiladigan bot topilmadi")
		return
	}

	log.Printf("🔄 RestoreActiveBots: %d ta faol bot topildi, qayta ishga tushirilmoqda...", len(activeBots))

	restored := 0
	for i := range activeBots {
		b := &activeBots[i]

		// Muddati tugagan bo'lsa, qayta ishga tushirmasdan billing checker'ga qoldiramiz
		// (fastBillingCheck o'zi balansni tekshirib, kerak bo'lsa to'xtatadi yoki yangilaydi)
		if b.PaidUntil.Before(now) {
			log.Printf("⏰ Bot @%s muddati tugagan, billing checker hal qiladi", b.BotUsername)
		}

		go StartBot(b)
		restored++
	}

	log.Printf("✅ RestoreActiveBots: %d ta bot qayta ishga tushirildi", restored)
}
