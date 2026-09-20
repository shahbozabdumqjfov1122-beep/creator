package controllers

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	beego "github.com/beego/beego/v2/server/web"
	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"
)

// Xabarlari kuzatiladigan botlar (kichik harflarda).
var paymentBotUsernames = []string{"@HUMOcardbot"}

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
				log.Printf("⚠️ Userbot to'xtadi: %v — 10 soniyadan keyin qayta ulanadi", err)
			}
			time.Sleep(10 * time.Second)
		}
	}()
}

func runHumoUserbot(apiID int, apiHash, phone, password string) error {
	dispatcher := tg.NewUpdateDispatcher()

	// Update'larni to'g'ri qabul qilish uchun manager yaratiladi
	gaps := updates.New(updates.Config{
		Handler: dispatcher,
	})

	client := telegram.NewClient(apiID, apiHash, telegram.Options{
		UpdateHandler:  gaps,
		SessionStorage: &session.FileStorage{Path: "userbot.session.json"},
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

		// Xabar matnini qayta ishlash funksiyasi
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

		// Update siklini ushlab turish uchun gaps.Run ishlatiladi
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
		return "", err
	}
	return strings.TrimSpace(code), nil
}
