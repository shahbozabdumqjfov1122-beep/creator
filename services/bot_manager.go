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
const DailyPriceAnimePro = 3000.0
const TestDuration = 24 * time.Hour

// dailyPriceFor bot turiga qarab kunlik narxni qaytaradi.
// "animepro" uchun 3000 so'm, qolgan barcha turlar uchun standart 1500 so'm.
func dailyPriceFor(b *models.CreatedBot) float64 {
	if b != nil && b.BotType != nil && b.BotType.Code == "animepro" {
		return DailyPriceAnimePro
	}
	return DailyPrice
}

func SetSharedCreatorBot(bot *tgbotapi.BotAPI) {
	CreatorBot = bot
}

// startMode - bot qanday holatda ishga tushayotganini bildiradi
type startMode int

const (
	ModeNewBot    startMode = iota // 🎁 birinchi marta yaratilganda — 1 kun BEPUL
	ModeResume                     // 💰 to'xtatilgandan keyin qayta yoqilganda — darhol to'lov
	ModeReconnect                  // 🔄 server qayta ishga tushganda — hech narsa o'zgarmaydi
)

// StartNewBot — FAQAT bot birinchi marta yaratilganda chaqiriladi.
// Balansdan HECH NARSA yechilmaydi, 1-kun bepul beriladi.
func StartNewBot(b *models.CreatedBot) {
	mu.Lock()
	defer mu.Unlock()
	startBotInternal(b, ModeNewBot)
}

// StartBot — to'xtatilgan botni qayta yoqishda chaqiriladi (masalan "Qayta yoqish" tugmasi,
// yoki balans to'ldirilgach ResumeBotsAfterTopUp orqali). Darhol bot turiga mos narx yechiladi.
func StartBot(b *models.CreatedBot) {
	mu.Lock()
	defer mu.Unlock()
	startBotInternal(b, ModeResume)
}

// ReconnectBot — server qayta ishga tushganda (RestoreActiveBots) chaqiriladi.
// PaidUntil'ga tegmaydi, balans yechilmaydi, faqat mavjud holatni davom ettiradi.
func ReconnectBot(b *models.CreatedBot) {
	mu.Lock()
	defer mu.Unlock()
	startBotInternal(b, ModeReconnect)
}

func startBotInternal(b *models.CreatedBot, mode startMode) {
	o := orm.NewOrm()

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

	price := dailyPriceFor(b)

	// 💰 Faqat "qayta yoqish" holatida darhol pul yechiladi. Yangi bot uchun — BEPUL.
	if mode == ModeResume {
		var owner models.UserBot
		if err := o.QueryTable("user_bot").Filter("Id", b.Owner.Id).One(&owner); err != nil {
			log.Printf("❌ StartBot: egasi topilmadi (Bot Id=%d): %v", b.Id, err)
			return
		}

		if owner.Balance < price {
			log.Printf("⛔ Bot @%s ishga tushmadi: balans yetarli emas (%.0f so'm, kerak: %.0f so'm)", b.BotUsername, owner.Balance, price)
			b.IsSuspended = true
			o.Update(b, "IsSuspended")
			NotifyOwner(owner.TgId, b, false)
			return
		}

		owner.Balance -= price
		o.Update(&owner, "Balance")
		log.Printf("💰 Bot @%s uchun %.0f so'm darhol yechildi. Qolgan balans: %.0f", b.BotUsername, price, owner.Balance)
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

	switch mode {
	case ModeNewBot:
		b.PaidUntil = time.Now().Add(24 * time.Hour)
		b.IsSuspended = false
		o.Update(b, "PaidUntil", "IsSuspended")
		log.Printf("🎁 Bot ishga tushdi: @%s (1-kun BEPUL, 2-kundan to'lov boshlanadi)", b.BotUsername)

	case ModeResume:
		b.PaidUntil = time.Now().Add(24 * time.Hour)
		b.IsSuspended = false
		o.Update(b, "PaidUntil", "IsSuspended")
		log.Printf("✅ Bot ishga tushdi: @%s (1 kun to'landi, %.0f so'm)", b.BotUsername, price)

	case ModeReconnect:
		log.Printf("✅ Bot qayta ulandi: @%s (PaidUntil o'zgarishsiz: %v)", b.BotUsername, b.PaidUntil)
	}

	go runBotLoop(ctx, bot, b)
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
		ticker := time.NewTicker(30 * time.Second)
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
		RelatedSel("BotType").
		All(&activeBots)

	for i := range activeBots {
		bot := &activeBots[i]

		if bot.PaidUntil.Before(now) {
			var user models.UserBot
			err := o.QueryTable("user_bot").Filter("Id", bot.Owner.Id).One(&user)
			if err != nil {
				StopBot(bot.Id)
				continue
			}

			price := dailyPriceFor(bot)

			if user.Balance >= price {
				user.Balance -= price
				o.Update(&user, "Balance")

				bot.PaidUntil = time.Now().Add(TestDuration)
				o.Update(bot, "PaidUntil")

				log.Printf("💰 Bot @%s uchun %.0f so'm yechildi. Yangi muddat: +24 Hour", bot.BotUsername, price)
			} else {
				StopBot(bot.Id)
				bot.IsSuspended = true
				o.Update(bot, "IsSuspended")
				NotifyOwner(user.TgId, bot, false)
				log.Printf("⚠️ Bot @%s to'xtatildi (balans yetmadi, kerak: %.0f so'm)", bot.BotUsername, price)
			}
		}
	}
}

func StartDailyBillingScheduler() {
	log.Println("💰 Kunlik billing scheduler hozircha o'chirilgan (test rejimi)")
}

// ResumeBotsAfterTopUp — balans to'ldirilgach to'xtatilgan botlarni qayta yoqadi.
// StartBot (ModeResume) o'zi balansni yechadi, shuning uchun bu yerda qo'lda yechish OLIB TASHLANDI.
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
		RelatedSel("BotType").
		All(&suspendedBots)

	if err != nil || len(suspendedBots) == 0 {
		log.Printf("ℹ️ ResumeBots: To'xtatilgan bot yo'q (User %d)", ownerTgId)
		return
	}

	log.Printf("🔄 %d ta to'xtatilgan bot topildi. Balans: %.0f so'm", len(suspendedBots), owner.Balance)

	resumed := 0
	for i := range suspendedBots {
		b := &suspendedBots[i]

		// har safar balansni bazadan yangilab olamiz (StartBot ichida o'zgargan bo'lishi mumkin)
		if err := o.QueryTable("user_bot").Filter("Id", owner.Id).One(&owner); err != nil {
			log.Printf("❌ ResumeBots: balansni yangilab bo'lmadi: %v", err)
			break
		}

		price := dailyPriceFor(b)

		if owner.Balance < price {
			log.Printf("⛔ Balans yetarli emas (kerak: %.0f so'm), qolgan botlar ochilmaydi", price)
			break
		}

		StartBot(b) // ✅ balansni o'zi yechadi va botni ishga tushiradi (ModeResume)
		NotifyOwner(ownerTgId, b, true)

		resumed++
		log.Printf("✅ Bot #%d (@%s) avtomatik ishga tushdi", b.Id, b.BotUsername)
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

	price := dailyPriceFor(b)

	var text string
	if resumed {
		text = fmt.Sprintf("✅ *@%s* botingiz qayta ishga tushdi!\n💰 Hisobingizdan *%.0f so'm* yechildi.", b.BotUsername, price)
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
		All(&activeBots)

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

		if b.PaidUntil.Before(now) {
			log.Printf("⏰ Bot @%s muddati tugagan, billing checker hal qiladi", b.BotUsername)
		}

		go ReconnectBot(b)
		restored++
	}

	log.Printf("✅ RestoreActiveBots: %d ta bot qayta ishga tushirildi", restored)
}
