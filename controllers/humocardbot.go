package controllers

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"creator/models"

	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"
)

// Xabarlari kuzatiladigan botlar.
var paymentBotUsernames = []string{"humocardbot"}

// Sessiya bazada shu nom bilan saqlanadi
const userbotSessionName = "humo_userbot"

// Kod kiritish uchun terminal yo'q bo'lsa true bo'ladi
var userbotNeedsLogin atomic.Bool

func isPaymentBot(username string) bool {
	u := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(username), "@"))
	for _, name := range paymentBotUsernames {
		n := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(name), "@"))
		if u == n {
			return true
		}
	}
	return false
}

// ============================================================
// SESSIYANI BAZADA SAQLASH
// ============================================================

// dbSessionStorage - gotd session.Storage interfeysini PostgreSQL orqali amalga oshiradi
type dbSessionStorage struct {
	name string
}

func (s *dbSessionStorage) LoadSession(ctx context.Context) ([]byte, error) {
	o := orm.NewOrm()

	var rec models.UserbotSession
	err := o.QueryTable(new(models.UserbotSession)).Filter("Name", s.name).One(&rec)
	if err == orm.ErrNoRows {
		return nil, session.ErrNotFound // sessiya hali yo'q, yangi kirish kerak
	}
	if err != nil {
		return nil, err
	}
	return []byte(rec.Data), nil
}

func (s *dbSessionStorage) StoreSession(ctx context.Context, data []byte) error {
	o := orm.NewOrm()

	var rec models.UserbotSession
	err := o.QueryTable(new(models.UserbotSession)).Filter("Name", s.name).One(&rec)
	if err == orm.ErrNoRows {
		rec = models.UserbotSession{Name: s.name, Data: string(data)}
		_, insErr := o.Insert(&rec)
		return insErr
	}
	if err != nil {
		return err
	}

	rec.Data = string(data)
	_, updErr := o.Update(&rec, "Data", "UpdatedAt")
	return updErr
}

// ============================================================
// USERBOT
// ============================================================

// StartHumoUserbot - Telegram akkauntingizga ulanib xabarlarni kuzatadi.
func StartHumoUserbot() {
	apiID, _ := beego.AppConfig.Int("userbot_api_id")
	apiHash, _ := beego.AppConfig.String("userbot_api_hash")
	phone, _ := beego.AppConfig.String("userbot_phone")
	password, _ := beego.AppConfig.String("userbot_password")

	if apiID == 0 || apiHash == "" || phone == "" {
		log.Println("ℹ️ Userbot sozlanmagan (userbot_api_id/hash/phone), avto-qabul o'chirilgan")
		return
	}

	go func() {
		for {
			if err := runHumoUserbot(apiID, apiHash, phone, password); err != nil {
				log.Printf("⚠️ Userbot to'xtadi: %v", err)
			}

			// Terminal yo'q va sessiya yo'q bo'lsa, qayta-qayta kod so'ramaymiz
			if userbotNeedsLogin.Load() {
				log.Println("❌ Userbot sessiyasi yo'q yoki eskirgan. Serverda qo'lda kirish kerak: `systemctl stop creator`, keyin `./creator` ni ishga tushirib, Telegram kodini kiriting.")
				send(AdminChatID, "⚠️ Userbot Telegram'ga kira olmadi. Sessiya yo'q yoki tugatilgan. Serverda qo'lda kirish kerak, avto-qabul hozir ishlamaydi.", nil)
				return
			}

			log.Println("🔄 Userbot 10 soniyadan keyin qayta ulanadi")
			time.Sleep(10 * time.Second)
		}
	}()
}

func runHumoUserbot(apiID int, apiHash, phone, password string) error {
	dispatcher := tg.NewUpdateDispatcher()

	// Update'larni to'g'ri qabul qilish uchun manager
	gaps := updates.New(updates.Config{
		Handler: dispatcher,
	})

	client := telegram.NewClient(apiID, apiHash, telegram.Options{
		UpdateHandler:  gaps,
		SessionStorage: &dbSessionStorage{name: userbotSessionName},
	})

	dispatcher.OnNewMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewMessage) error {
		msg, ok := u.Message.(*tg.Message)
		if !ok || msg.Out {
			return nil
		}
		peer, ok := msg.PeerID.(*tg.PeerUser)
		if !ok {
			return nil
		}

		sender, ok := e.Users[peer.UserID]
		if !ok || !isPaymentBot(sender.Username) {
			return nil // boshqa chatlar e'tiborga olinmaydi
		}

		log.Printf("📩 %s dan xabar keldi, tekshirilmoqda...", sender.Username)
		processIncomingSMS(msg.Message)
		return nil
	})

	return client.Run(context.Background(), func(ctx context.Context) error {
		flow := auth.NewFlow(
			auth.Constant(phone, password, auth.CodeAuthenticatorFunc(askLoginCode)),
			auth.SendCodeOptions{},
		)
		if err := client.Auth().IfNecessary(ctx, flow); err != nil {
			return fmt.Errorf("kirishda xato: %w", err)
		}

		user, err := client.Self(ctx)
		if err != nil {
			return fmt.Errorf("self olishda xato: %w", err)
		}

		log.Println("🤖 Userbot ulandi, HUMOcard xabarlari kuzatilmoqda")

		return gaps.Run(ctx, client.API(), user.ID, updates.AuthOptions{
			IsBot: false,
		})
	})
}

// askLoginCode - birinchi ishga tushirishda Telegram yuborgan kodni terminaldan so'raydi
func askLoginCode(ctx context.Context, _ *tg.AuthSentCode) (string, error) {
	fmt.Print("📲 Telegram'ga kelgan kodni kiriting: ")
	code, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		userbotNeedsLogin.Store(true) // terminal yo'q (masalan, systemd): qayta urinmaymiz
		return "", err
	}
	return strings.TrimSpace(code), nil
}
