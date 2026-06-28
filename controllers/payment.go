package controllers

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"strconv"
	"time"

	"creator/models"
	"creator/services"

	"github.com/beego/beego/v2/client/orm"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// AdminChatID - to'lov so'rovlari yuboriladigan admin chat ID
const AdminChatID int64 = 7518992824

// ============================================================
// 1. INVOICE YARATISH
// ============================================================

// ProcessTopUpAmount - Foydalanuvchi kiritgan summani tekshirib, unikal invoice yaratadi
func ProcessTopUpAmount(chatID int64, userID int64, textAmount string) {
	inputAmount, err := strconv.ParseFloat(textAmount, 64)
	if err != nil || inputAmount < 1000 {
		msg := tgbotapi.NewMessage(chatID, "Iltimos, minimal 1 000 so'm bo'lgan faqat son kiriting:")
		CreatorBot.Send(msg)
		return
	}

	o := orm.NewOrm()
	now := time.Now()
	expires := now.Add(1 * time.Hour) // 🎯 endi 1 soat

	var finalAmount float64
	var randomDiff int

	// Unikal summa topguncha aylanamiz
	for {
		nBig, _ := rand.Int(rand.Reader, big.NewInt(99))
		randomDiff = int(nBig.Int64()) + 1
		finalAmount = inputAmount + float64(randomDiff)

		exists := o.QueryTable(new(models.BotInvoice)).
			Filter("FinalAmount", finalAmount).
			Filter("Status", "pending").
			Filter("ExpiresAt__gt", now).
			Exist()

		if !exists {
			break
		}
	}

	invoice := &models.BotInvoice{
		Bot:         nil,
		UserId:      userID,
		Amount:      inputAmount,
		FinalAmount: finalAmount,
		Diff:        randomDiff,
		Status:      "pending",
		ExpiresAt:   expires,
	}

	_, insertErr := o.Insert(invoice)
	if insertErr != nil {
		log.Printf("Invoice saqlashda xato: %v", insertErr)
		msg := tgbotapi.NewMessage(chatID, "max : 999999999")
		CreatorBot.Send(msg)
		return
	}

	mu.Lock()
	delete(userState, chatID)
	mu.Unlock()

	responseText := fmt.Sprintf(
		"💳 Karta raqami: `9860 0803 8859 7462`\n"+
			"👤 Karta egasi: A.SH\n\n"+
			"💰 To'lov summasi: `%.0f` so'm\n"+
			"➕ Qo'shimcha summa: %d so'm (to'lovni tasdiqlash uchun)\n\n"+
			"⚠️ Diqqat:\n"+
			"⏱ To'lovni amalga oshirish uchun 1 soat vaqt beriladi.\n"+
			"❌ 1 soatdan keyin to'lov qilsangiz, hisobingizga mablag' tushmaydi.\n\n"+
			"✅ To'lovni amalga oshirgandan keyin pastdagi tugmani bosing:",
		invoice.FinalAmount, invoice.Diff,
	)
	keyboard := RangliKlaviatura{
		InlineKeyboard: [][]RangliTugma{
			{
				{
					Text:         "✅ To'lov qildim",
					CallbackData: fmt.Sprintf("paid_claim:%d", invoice.Id),
					Style:        "success", // Yashil chiroyli rang
				},
			},
		},
	}

	msg := tgbotapi.NewMessage(chatID, responseText)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard // Rangli klaviaturani ulaymiz
	CreatorBot.Send(msg)
}

// ============================================================
// 2. FOYDALANUVCHI "TO'LOV QILDIM" TUGMASINI BOSGANDA
// ============================================================

// adminga tasdiqlash so'rovini yuboradi
func HandlePaidClaim(chatID int64, userID int64, username string, invoiceID int64) {
	o := orm.NewOrm()

	var invoice models.BotInvoice
	err := o.QueryTable(new(models.BotInvoice)).Filter("Id", invoiceID).One(&invoice)
	if err != nil {
		send(chatID, "❌ Bu to'lov so'rovi topilmadi.", nil)
		return
	}

	if invoice.UserId != userID {
		send(chatID, "❌ Bu sizga tegishli to'lov so'rovi emas.", nil)
		return
	}

	if invoice.Status != "pending" {
		switch invoice.Status {
		case "paid":
			send(chatID, "✅ Bu to'lov allaqachon tasdiqlangan.", nil)
		case "rejected":
			send(chatID, "❌ Bu to'lov rad etilgan.", nil)
		case "expired":
			send(chatID, "⏱ Bu to'lovning muddati tugagan. Iltimos, qaytadan \"Balansni to'ldirish\"ni bosing.", nil)
		default:
			send(chatID, "⚠️ Bu to'lov bo'yicha amal qilib bo'lmaydi.", nil)
		}
		return
	}

	if time.Now().After(invoice.ExpiresAt) {
		send(chatID, "⏱ Bu to'lovning muddati tugagan. Iltimos, qaytadan \"Balansni to'ldirish\"ni bosing.", nil)
		return
	}

	usernameDisplay := "Noma'lum"
	if username != "" {
		usernameDisplay = "@" + username
	}

	adminText := fmt.Sprintf(
		"🔔 Yangi to'lov so'rovi!\n\n"+
			"👤 Foydalanuvchi: %s (ID: `%d`)\n"+
			"💰 Talab qilingan summa: `%.0f` so'm\n"+
			"💳 To'liq to'lash kerak bo'lgan summa: `%.0f` so'm\n\n"+
			"❓ Ushbu foydalanuvchidan kartaga pul keldimi?",
		usernameDisplay, userID, invoice.Amount, invoice.FinalAmount,
	)
	adminKeyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Ha, keldi", fmt.Sprintf("admin_approve:%d", invoice.Id)),
			tgbotapi.NewInlineKeyboardButtonData("❌ Yo'q", fmt.Sprintf("admin_reject:%d", invoice.Id)),
		),
	)
	adminMsg := tgbotapi.NewMessage(AdminChatID, adminText)
	// adminMsg.ParseMode = "Markdown"  // 🎯 olib tashlandi — username'dagi maxsus belgilar xato chiqarmasligi uchun
	adminMsg.ReplyMarkup = adminKeyboard
	_, sendErr := CreatorBot.Send(adminMsg)
	if sendErr != nil {
		log.Printf("❌ Adminga xabar yuborishda xato: %v", sendErr)
	}
	send(chatID, "📨 So'rovingiz adminga yuborildi. Tasdiqlanishini kuting...", nil)
}

// ============================================================
// 3. ADMIN TASDIQLASA YOKI RAD ETSA
// ============================================================

// HandleAdminApprove - admin "✅ Ha, keldi" tugmasini bosganda
func HandleAdminApprove(invoiceID int64) {
	o := orm.NewOrm()

	var invoice models.BotInvoice
	err := o.QueryTable(new(models.BotInvoice)).Filter("Id", invoiceID).One(&invoice)
	if err != nil {
		send(AdminChatID, "❌ Invoice topilmadi.", nil)
		return
	}

	if invoice.Status != "pending" {
		send(AdminChatID, "⚠️ Bu invoice allaqachon ko'rib chiqilgan.", nil)
		return
	}

	invoice.Status = "paid"
	_, updateErr := o.Update(&invoice, "Status")
	if updateErr != nil {
		log.Printf("Invoice statusini yangilashda xato: %v", updateErr)
		send(AdminChatID, "❌ Statusni yangilashda xatolik yuz berdi.", nil)
		return
	}

	topUpUserBalance(invoice.UserId, invoice.Amount)

	successText := fmt.Sprintf(
		"✅ To'lov tasdiqlandi!\n\n"+
			"💰 Hisob to'ldirildi: `%.0f` so'm\n"+
			"🎉 Mablag' hisobingizga muvaffaqiyatli tushdi!",
		invoice.Amount,
	)
	send(invoice.UserId, successText, nil)
	sendMarkdown(invoice.UserId, successText)
	// HandleAdminApprove oxirida, successText yuborilgandan keyin qo'shing:
	topUpUserBalance(invoice.UserId, invoice.Amount)

	// ... successText yuborilgandan keyin
	services.ResumeBotsAfterTopUp(invoice.UserId) // ← qo'shimcha chaqiruv
	send(AdminChatID, fmt.Sprintf("✅ Tasdiqlandi: UserID=%d, Summa=%.0f so'm", invoice.UserId, invoice.Amount), nil)

	log.Printf("✅ To'lov admin tomonidan tasdiqlandi: UserID=%d, Summa=%.0f", invoice.UserId, invoice.Amount)
}

// HandleAdminReject - admin "❌ Yo'q" tugmasini bosganda
func HandleAdminReject(invoiceID int64) {
	o := orm.NewOrm()

	var invoice models.BotInvoice
	err := o.QueryTable(new(models.BotInvoice)).Filter("Id", invoiceID).One(&invoice)
	if err != nil {
		send(AdminChatID, "❌ Invoice topilmadi.", nil)
		return
	}

	if invoice.Status != "pending" {
		send(AdminChatID, "⚠️ Bu invoice allaqachon ko'rib chiqilgan.", nil)
		return
	}

	invoice.Status = "rejected"
	_, updateErr := o.Update(&invoice, "Status")
	if updateErr != nil {
		log.Printf("Invoice statusini yangilashda xato: %v", updateErr)
		send(AdminChatID, "❌ Statusni yangilashda xatolik yuz berdi.", nil)
		return
	}

	rejectText := fmt.Sprintf(
		"❌ To'lov tasdiqlanmadi\n\n"+
			"💰 Summa: `%.0f` so'm\n\n"+
			"Kartaga pul tushgani aniqlanmadi. Agar to'lov qilgan bo'lsangiz, qayta tekshirib, qaytadan urinib ko'ring yoki admin bilan bog'laning.",
		invoice.FinalAmount,
	)
	sendMarkdown(invoice.UserId, rejectText)

	send(AdminChatID, fmt.Sprintf("❌ Rad etildi: UserID=%d, Summa=%.0f so'm", invoice.UserId, invoice.FinalAmount), nil)

	log.Printf("❌ To'lov admin tomonidan rad etildi: UserID=%d, Summa=%.0f", invoice.UserId, invoice.FinalAmount)
}

// sendMarkdown - Markdown formatda oddiy xabar yuborish uchun yordamchi
func sendMarkdown(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	_, err := CreatorBot.Send(msg)
	if err != nil {
		log.Println("SEND ERROR:", err)
	}
}

// ============================================================
// 4. BALANSNI TO'LDIRISH
// ============================================================

// topUpUserBalance - Foydalanuvchi hisobini ma'lumotlar bazasida to'ldiradi
func topUpUserBalance(userID int64, amount float64) {
	o := orm.NewOrm()

	var user models.UserBot
	err := o.QueryTable(new(models.UserBot)).
		Filter("TgId", userID).
		One(&user)

	if err != nil {
		log.Printf("Foydalanuvchi topilmadi (TgID: %d): %v", userID, err)
		return
	}

	user.Balance += amount
	_, updateErr := o.Update(&user, "Balance")
	if updateErr != nil {
		log.Printf("Balansni yangilashda xato: %v", updateErr)
		return
	}
	o.Update(&user, "Balance")
	services.ResumeBotsAfterTopUp(userID)

	log.Printf("💰 Balans yangilandi: UserID=%d, +%.0f so'm", userID, amount)
}

// ============================================================
// 5. MUDDATI O'TGAN INVOICELARNI TOZALASH
// ============================================================

// StartExpiredInvoiceCleaner - Muddati o'tgan invoicelarni avtomatik "expired" ga o'zgartiradi
func StartExpiredInvoiceCleaner() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			expireOldInvoices()
		}
	}()
	log.Println("⏱ Invoice muddati tugashini tekshiruvchi scheduler ishga tushdi")
}

func expireOldInvoices() {
	o := orm.NewOrm()
	now := time.Now()

	var invoices []models.BotInvoice
	_, err := o.QueryTable(new(models.BotInvoice)).
		Filter("Status", "pending").
		Filter("ExpiresAt__lt", now).
		All(&invoices)

	if err != nil {
		log.Printf("Muddati o'tgan invoice'larni qidirishda xato: %v", err)
		return
	}

	for i := range invoices {
		inv := &invoices[i]

		inv.Status = "expired"
		_, updateErr := o.Update(inv, "Status")
		if updateErr != nil {
			log.Printf("Invoice statusini 'expired' ga o'zgartirishda xato (ID=%d): %v", inv.Id, updateErr)
			continue
		}

		cancelText := fmt.Sprintf(
			"To'lov bekor qilindi\n\n"+
				"Summa: `%.0f` so'm\n\n"+
				"⏱ 1 soat ichida to'lov amalga oshirilmadi yoki tasdiqlanmadi, shu sababli buyurtma bekor qilindi.\n"+
				"Qaytadan urinib ko'rish uchun \"Balansni to'ldirish\" tugmasini bosing.",
			inv.FinalAmount,
		)

		sendMarkdown(inv.UserId, cancelText)

		log.Printf("⏱ Invoice muddati tugadi: UserID=%d, Summa=%.0f", inv.UserId, inv.FinalAmount)
	}
}
