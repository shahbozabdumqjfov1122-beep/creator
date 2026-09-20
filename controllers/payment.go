package controllers

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"creator/models"
	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// AdminChatID - zaxira (qo'lda tasdiqlash) so'rovlari yuboriladigan admin chat ID
const AdminChatID int64 = 7518992824

const (
	// Karta oxirgi 4 raqami. SMS ichida shu raqam bo'lmasa, SMS e'tiborga olinmaydi.
	// Agar SMS'da karta raqami chiqmasa, "" qilib qo'ying.
	CardLast4 = "7462"

	// "To'lov qildim" bosilgandan keyin, avtomatik tasdiq kelmasa,
	// shuncha vaqtdan so'ng adminga zaxira so'rov yuboriladi.
	claimFallbackDelay = 3 * time.Minute
)

// Bitta invoice uchun adminga faqat bir marta so'rov yuborilishi uchun
var claimedInvoices sync.Map

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
	expires := now.Add(1 * time.Hour)

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
			"➕ Qo'shimcha summa: %d so'm (to'lovni avtomatik aniqlash uchun)\n\n"+
			"⚠️ Diqqat:\n"+
			"🔢 Aynan shu summani o'tkazing — bir tiyin ham kam yoki ko'p bo'lmasin!\n"+
			"⏱ To'lovni amalga oshirish uchun 1 soat vaqt beriladi.\n"+
			"❌ 1 soatdan keyin to'lov qilsangiz, hisobingizga mablag' tushmaydi.\n\n"+
			"🤖 To'lov kartaga tushishi bilan hisobingiz avtomatik to'ldiriladi.\n"+
			"Agar 2-3 daqiqada tushmasa, pastdagi tugmani bosing:",
		invoice.FinalAmount, invoice.Diff,
	)
	keyboard := RangliKlaviatura{
		InlineKeyboard: [][]RangliTugma{
			{
				{
					Text:         "✅ To'lov qildim",
					CallbackData: fmt.Sprintf("paid_claim:%d", invoice.Id),
					Style:        "success",
				},
			},
		},
	}

	msg := tgbotapi.NewMessage(chatID, responseText)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	CreatorBot.Send(msg)
}

// ============================================================
// 2. AVTOMATIK QABUL QILISH (HUMOcard userbot; SMS webhook ixtiyoriy)
// ============================================================
//
// Ishlash tartibi:
//  1. Karta ulangan telefonga kelgan bank SMS'i (Humo/Uzcard xabarnoma)
//     "SMS Forwarder" ilovasi orqali shu webhook'ga POST qilinadi.
//  2. SMS matnidan summa ajratib olinadi.
//  3. Summa kutilayotgan (pending) invoice'ning FinalAmount'iga teng bo'lsa,
//     invoice avtomatik "paid" qilinadi va balans to'ldiriladi.
//
// Asosiy usul: humo_userbot.go HUMOcard botidan kelgan xabarni to'g'ridan-to'g'ri
// processIncomingSMS'ga uzatadi (webhook kerak emas).
//
// Ixtiyoriy SMS webhook sozlash (faqat SMS Forwarder ishlatsangiz):
//   - app.conf: payment_webhook_secret = uzun_tasodifiy_kalit
//   - Router'ga qo'shing (masalan, routers/router.go):
//         web.Handler("/payment/sms", http.HandlerFunc(controllers.SMSWebhookHandler))
//   - SMS Forwarder'da: POST https://SIZNING-DOMEN/payment/sms
//         Header:  X-Webhook-Secret: <kalit>
//         Body:    {"text": "<SMS matni>"}   (ilovadagi placeholder'ni qo'ying)

var (
	amountRegex = regexp.MustCompile(`(?i)(?:^|[^\d.,])(\d{1,3}(?:[ \x{00a0}.,]\d{3})+(?:[.,]\d{1,2})?|\d+(?:[.,]\d{1,2})?)\s*(?:UZS|so'm|so‘m|som|sum|сум)`)

	// Balans/qoldiq summasini to'lov summasi deb adashtirmaslik uchun
	balanceWords = []string{"balans", "balance", "баланс", "qoldiq", "ostatok", "остаток", "dostupno", "доступно", "💰"}

	// Chiqim SMS'larini o'tkazib yuborish uchun
	outgoingWords = []string{"spisanie", "списание", "oplata", "оплата", "pokupka", "покупка", "snyatie", "снятие", "chiqim", "purchase", "withdrawal", "➖"}
)

// SMSWebhookHandler - telefondan kelgan SMS'ni qabul qiladi
func SMSWebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	secret, _ := beego.AppConfig.String("payment_webhook_secret")
	if secret == "" {
		log.Println("❌ payment_webhook_secret app.conf da yo'q, webhook o'chirilgan")
		http.Error(w, "server misconfigured", http.StatusInternalServerError)
		return
	}

	got := r.Header.Get("X-Webhook-Secret")
	if got == "" {
		got = r.URL.Query().Get("key")
	}
	if subtle.ConstantTimeCompare([]byte(got), []byte(secret)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	text := readSMSText(r)
	if strings.TrimSpace(text) == "" {
		http.Error(w, "empty text", http.StatusBadRequest)
		return
	}

	processIncomingSMS(text)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// readSMSText - JSON yoki form ko'rinishidagi so'rovdan SMS matnini oladi
func readSMSText(r *http.Request) string {
	keys := []string{"text", "message", "body", "sms", "content"}

	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			return ""
		}
		for _, k := range keys {
			if v, ok := payload[k].(string); ok && v != "" {
				return v
			}
		}
		return ""
	}

	_ = r.ParseForm()
	for _, k := range keys {
		if v := r.FormValue(k); v != "" {
			return v
		}
	}
	return ""
}

// extractAmounts - xabar matnidan summalarni ajratib oladi.
// Humo bot formati: kirim summasi "➕" belgili qatorda, balans esa "💰" qatorida.
// Shuning uchun "➕" bor bo'lsa, faqat shu qatorlardan summa olinadi.
func extractAmounts(text string) []float64 {
	var plusLines []string
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "➕") {
			plusLines = append(plusLines, line)
		}
	}
	if len(plusLines) > 0 {
		text = strings.Join(plusLines, "\n")
	}

	var result []float64
	for _, loc := range amountRegex.FindAllStringSubmatchIndex(text, -1) {
		// Summadan oldingi ~20 belgida "balans", "💰" kabi belgi bo'lsa, o'tkazib yuboramiz
		start := loc[2]
		from := start - 20
		if from < 0 {
			from = 0
		}
		prefix := strings.ToLower(text[from:start])
		skip := false
		for _, w := range balanceWords {
			if strings.Contains(prefix, w) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		if val, ok := parseAmount(text[loc[2]:loc[3]]); ok {
			result = append(result, val)
		}
	}
	return result
}

// parseAmount - "1.000,00", "50 037.00", "1,000.50", "1000" kabi formatlarni songa aylantiradi.
// Faqat butun summalar qabul qilinadi (tiyinli summa e'tiborga olinmaydi).
func parseAmount(token string) (float64, bool) {
	token = strings.NewReplacer(" ", "", "\u00a0", "").Replace(token)
	if token == "" {
		return 0, false
	}

	lastDot := strings.LastIndex(token, ".")
	lastComma := strings.LastIndex(token, ",")

	decimalPos := -1
	switch {
	case lastDot >= 0 && lastComma >= 0:
		// Ikkalasi ham bor: oxirgisi o'nlik ajratgich
		decimalPos = lastDot
		if lastComma > lastDot {
			decimalPos = lastComma
		}
	case lastDot >= 0:
		if strings.Count(token, ".") == 1 && len(token)-lastDot-1 != 3 {
			decimalPos = lastDot
		}
	case lastComma >= 0:
		if strings.Count(token, ",") == 1 && len(token)-lastComma-1 != 3 {
			decimalPos = lastComma
		}
	}

	intPart, frac := token, ""
	if decimalPos >= 0 {
		intPart, frac = token[:decimalPos], token[decimalPos+1:]
	}
	intPart = strings.NewReplacer(".", "", ",", "").Replace(intPart)

	numStr := intPart
	if frac != "" {
		numStr += "." + frac
	}

	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, false
	}
	if math.Abs(val-math.Round(val)) > 0.001 {
		return 0, false
	}
	return math.Round(val), true
}

// processIncomingSMS - SMS'dan summani topib, mos invoice'ni tasdiqlaydi
func processIncomingSMS(text string) {
	lower := strings.ToLower(text)

	if CardLast4 != "" && !strings.Contains(text, CardLast4) {
		log.Printf("ℹ️ SMS e'tiborga olinmadi (karta raqami mos emas)")
		return
	}
	for _, w := range outgoingWords {
		if strings.Contains(lower, w) {
			log.Printf("ℹ️ SMS e'tiborga olinmadi (chiqim xabari)")
			return
		}
	}

	o := orm.NewOrm()
	amounts := extractAmounts(text)
	for _, amount := range amounts {
		var invoice models.BotInvoice
		err := o.QueryTable(new(models.BotInvoice)).
			Filter("FinalAmount", amount).
			Filter("Status", "pending").
			Filter("ExpiresAt__gt", time.Now()).
			One(&invoice)
		if err != nil {
			continue // bu summaga mos invoice yo'q
		}

		ok, confirmErr := confirmInvoice(invoice.Id, invoice.UserId, invoice.Amount)
		if confirmErr != nil {
			log.Printf("❌ Avto tasdiqlashda xato (invoice=%d): %v", invoice.Id, confirmErr)
			send(AdminChatID, fmt.Sprintf("❌ Avto tasdiqlashda xato: invoice=%d, UserID=%d. Qo'lda tekshiring.", invoice.Id, invoice.UserId), nil)
			return
		}
		if !ok {
			return // boshqa jarayon allaqachon tasdiqlagan
		}

		notifyPaid(invoice.UserId, invoice.Amount)
		send(AdminChatID, fmt.Sprintf("🤖 Avto tasdiqlandi: UserID=%d, Summa=%.0f so'm", invoice.UserId, invoice.Amount), nil)
		log.Printf("🤖 To'lov avtomatik tasdiqlandi: UserID=%d, Summa=%.0f", invoice.UserId, invoice.Amount)
		return
	}

	log.Printf("ℹ️ Mos invoice topilmadi. Xabardan olingan summalar: %v", amounts)
}

// ============================================================
// 3. FOYDALANUVCHI "TO'LOV QILDIM" TUGMASINI BOSGANDA (ZAXIRA)
// ============================================================

// HandlePaidClaim - avtomatik tasdiq kelmasa, 3 daqiqadan keyin adminga so'rov yuboradi
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

	// Bir invoice uchun faqat bitta zaxira taymer
	if _, loaded := claimedInvoices.LoadOrStore(invoiceID, true); loaded {
		send(chatID, "⏳ To'lovingiz tekshirilmoqda, iltimos kuting...", nil)
		return
	}

	send(chatID, "⏳ To'lovingiz tekshirilmoqda. Pul kartaga tushishi bilan hisobingiz avtomatik to'ldiriladi.", nil)

	time.AfterFunc(claimFallbackDelay, func() {
		defer claimedInvoices.Delete(invoiceID)
		sendAdminFallback(invoiceID, userID, username)
	})
}

// sendAdminFallback - invoice hali ham pending bo'lsa, adminga qo'lda tasdiqlash so'rovini yuboradi
func sendAdminFallback(invoiceID int64, userID int64, username string) {
	o := orm.NewOrm()

	var invoice models.BotInvoice
	if err := o.QueryTable(new(models.BotInvoice)).Filter("Id", invoiceID).One(&invoice); err != nil {
		return
	}
	if invoice.Status != "pending" {
		return // avtomatik tasdiqlangan yoki muddati o'tgan
	}

	usernameDisplay := "Noma'lum"
	if username != "" {
		usernameDisplay = "@" + username
	}

	adminText := fmt.Sprintf(
		"🔔 Avtomatik tasdiqlanmagan to'lov!\n\n"+
			"👤 Foydalanuvchi: %s (ID: %d)\n"+
			"💰 Talab qilingan summa: %.0f so'm\n"+
			"💳 To'liq to'lash kerak bo'lgan summa: %.0f so'm\n\n"+
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
	adminMsg.ReplyMarkup = adminKeyboard
	if _, err := CreatorBot.Send(adminMsg); err != nil {
		log.Printf("❌ Adminga xabar yuborishda xato: %v", err)
	}
}

// ============================================================
// 4. ADMIN TASDIQLASA YOKI RAD ETSA (ZAXIRA)
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

	// Atomik: faqat "pending" bo'lsa tasdiqlanadi, ikki marta qo'shilib ketmaydi
	ok, confirmErr := confirmInvoice(invoice.Id, invoice.UserId, invoice.Amount)
	if confirmErr != nil {
		log.Printf("Tasdiqlashda xato: %v", confirmErr)
		send(AdminChatID, "❌ Tasdiqlashda xatolik yuz berdi.", nil)
		return
	}
	if !ok {
		send(AdminChatID, "⚠️ Bu invoice allaqachon ko'rib chiqilgan.", nil)
		return
	}

	notifyPaid(invoice.UserId, invoice.Amount)
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

	n, updateErr := o.QueryTable(new(models.BotInvoice)).
		Filter("Id", invoiceID).
		Filter("Status", "pending").
		Update(orm.Params{"Status": "rejected"})
	if updateErr != nil {
		log.Printf("Invoice statusini yangilashda xato: %v", updateErr)
		send(AdminChatID, "❌ Statusni yangilashda xatolik yuz berdi.", nil)
		return
	}
	if n == 0 {
		send(AdminChatID, "⚠️ Bu invoice allaqachon ko'rib chiqilgan.", nil)
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

// ============================================================
// 5. UMUMIY YORDAMCHI FUNKSIYALAR
// ============================================================

// confirmInvoice - bitta tranzaksiyada invoice'ni "paid" qiladi va balansni oshiradi.
// Invoice allaqachon "pending" bo'lmasa (false, nil) qaytaradi — balans ikki marta qo'shilmaydi.
func confirmInvoice(invoiceID int64, userID int64, amount float64) (bool, error) {
	o := orm.NewOrm()
	tx, err := o.Begin()
	if err != nil {
		return false, err
	}

	n, err := tx.QueryTable(new(models.BotInvoice)).
		Filter("Id", invoiceID).
		Filter("Status", "pending").
		Update(orm.Params{"Status": "paid"})
	if err != nil {
		_ = tx.Rollback()
		return false, err
	}
	if n == 0 {
		_ = tx.Rollback()
		return false, nil
	}

	n, err = tx.QueryTable(new(models.UserBot)).
		Filter("TgId", userID).
		Update(orm.Params{"Balance": orm.ColValue(orm.ColAdd, amount)})
	if err != nil {
		_ = tx.Rollback()
		return false, err
	}
	if n == 0 {
		_ = tx.Rollback()
		return false, fmt.Errorf("foydalanuvchi topilmadi (TgID: %d)", userID)
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// notifyPaid - foydalanuvchiga to'lov tasdiqlangani haqida xabar yuboradi
func notifyPaid(userID int64, amount float64) {
	successText := fmt.Sprintf(
		"✅ To'lov tasdiqlandi!\n\n"+
			"💰 Hisob to'ldirildi: `%.0f` so'm\n"+
			"🎉 Mablag' hisobingizga muvaffaqiyatli tushdi!",
		amount,
	)
	sendMarkdown(userID, successText)
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
// 6. MUDDATI O'TGAN INVOICELARNI TOZALASH
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

		// Atomik: shu payt avto-tasdiqlangan bo'lsa, "expired" qilib yubormaymiz
		n, updateErr := o.QueryTable(new(models.BotInvoice)).
			Filter("Id", inv.Id).
			Filter("Status", "pending").
			Update(orm.Params{"Status": "expired"})
		if updateErr != nil {
			log.Printf("Invoice statusini 'expired' ga o'zgartirishda xato (ID=%d): %v", inv.Id, updateErr)
			continue
		}
		if n == 0 {
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
