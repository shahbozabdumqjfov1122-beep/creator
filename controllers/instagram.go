package controllers

//
//import (
//	"creator/models"
//	"fmt"
//	"log"
//	"os"
//	"os/exec"
//	"path/filepath"
//	"regexp"
//	"strings"
//	"time"
//
//	"github.com/beego/beego/v2/client/orm"
//	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
//)
//
//var instagramURLRegex = regexp.MustCompile(`(?:https?://)?(?:www\.)?instagram\.com/(?:reel|p|tv|stories)/[^\s]+`)
//
//type IGGraphQLResponse struct {
//	Items []struct {
//		VideoVersions []struct {
//			URL string `json:"url"`
//		} `json:"video_versions"`
//	} `json:"items"`
//}
//
//// RapidAPI'dan keladigan javob strukturasi
//type ApiResponse struct {
//	Status    bool   `json:"status"`
//	MediaType string `json:"media_type"`
//	URL       string `json:"url"` // Videoning tayyor .mp4 havolasi
//}
//
//// ====================== ASOSIY HANDLER ======================
//func HandleInstagramBotMessage(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
//	userID := msg.From.ID
//	chatID := msg.Chat.ID
//
//	botUser := GetOrCreateBotUser(b, userID, msg.From)
//
//	if botUser.IsBlocked {
//		sendUserBot(bot, chatID, "🚫 Siz botdan foydalanishdan bloklangansiz!")
//		return
//	}
//
//	// ==================== ADMIN PANEL ====================
//	if isAdmin(b, userID) {
//		mu.Lock()
//		state, exists := adminState[userID]
//		mu.Unlock()
//
//		if exists {
//			if state == "wait_channel" || state == "wait_link" {
//				HandleAdminCommands(bot, b, msg)
//				return
//			}
//
//			if strings.HasPrefix(state, "waiting_broadcast_message:") {
//				target := strings.TrimPrefix(state, "waiting_broadcast_message:")
//				mu.Lock()
//				delete(adminState, userID)
//				mu.Unlock()
//				RunBroadcast(bot, b, msg, target)
//				return
//			}
//
//			if RouteUserManagementState(bot, b, msg, state) {
//				return
//			}
//		}
//
//		switch msg.Text {
//		case "/admin":
//			showInstagramAdminPanel(bot, chatID)
//			return
//		case "➕ Kanal qo‘shish", "/addchannel":
//			HandleAdminCommands(bot, b, msg)
//			return
//		case "➖ Kanal o‘chirish", "/delchannel":
//			ShowChannelsToDelete(bot, b, chatID)
//			return
//		case "👥 Foydalanuvchilar":
//			showUsersPanel(bot, chatID)
//			return
//		case "📊 Statistika":
//			showInstagramStatistics(bot, b, chatID)
//			return
//		case "📢 Reklama":
//			startBroadcast(bot, chatID)
//			return
//		case "👥 Hammaga", "⭐ VIP'larga", "👤 Oddiylarga":
//			handleBroadcastCommand(bot, b, msg)
//			return
//		case "👤 Adminlar":
//			showAdminsPanel(bot, chatID)
//			return
//		case "➕ Admin qo'shish", "➖ Admin o'chirish", "📋 Adminlar ro'yxati":
//			if msg.Text == "➕ Admin qo'shish" {
//				startAdminAdd(bot, chatID, userID)
//			} else if msg.Text == "➖ Admin o'chirish" {
//				startAdminRemove(bot, chatID, userID)
//			} else {
//				showAdminsList(bot, b, chatID)
//			}
//			return
//		case "📋 VIP ro'yxati":
//			showUserList(bot, chatID, b.Id, true, false)
//			return
//		case "📋 Blok ro'yxati":
//			showUserList(bot, chatID, b.Id, false, true)
//			return
//		case "⬅️ Orqaga":
//			showInstagramAdminPanel(bot, chatID)
//			return
//		}
//	}
//
//	// ==================== ODDIY FOYDALANUVCHI ====================
//	if !botUser.IsVip && !CheckSubscription(bot, b, userID) {
//		ShowMembership(bot, b, chatID)
//		return
//	}
//
//	if instagramURLRegex.MatchString(msg.Text) {
//		handleInstagramDownload(bot, b, msg)
//		return
//	}
//
//	switch msg.Text {
//	case "/start", "/help":
//		handleInstagramStart(bot, b, msg)
//	default:
//		sendUserBot(bot, chatID, "🔗 Instagram reel/post linkini yuboring.\n\nMisol: `https://www.instagram.com/reel/Cabc123/`")
//	}
//}
//
//// ====================== LAYOUTS ======================
//func showInstagramAdminPanel(bot *tgbotapi.BotAPI, chatID int64) {
//	keyboard := tgbotapi.NewReplyKeyboard(
//		tgbotapi.NewKeyboardButtonRow(
//			tgbotapi.NewKeyboardButton("📊 Statistika"),
//			tgbotapi.NewKeyboardButton("👥 Foydalanuvchilar"),
//		),
//		tgbotapi.NewKeyboardButtonRow(
//			tgbotapi.NewKeyboardButton("➕ Kanal qo‘shish"),
//			tgbotapi.NewKeyboardButton("➖ Kanal o‘chirish"),
//		),
//		tgbotapi.NewKeyboardButtonRow(
//			tgbotapi.NewKeyboardButton("📢 Reklama"),
//		),
//		tgbotapi.NewKeyboardButtonRow(
//			tgbotapi.NewKeyboardButton("👤 Adminlar"),
//		),
//	)
//
//	msg := tgbotapi.NewMessage(chatID, "🛠 *Instagram Bot Admin Paneli*")
//	msg.ParseMode = "Markdown"
//	msg.ReplyMarkup = keyboard
//	bot.Send(msg)
//}
//
//// ====================== MUKAMMAL API DOWNLOADER ======================
//func downloadInstagramVideo(targetURL string) (string, error) {
//	tmpDir := os.TempDir()
//
//	output := filepath.Join(tmpDir, fmt.Sprintf("insta_%d.%%(ext)s", time.Now().Unix()))
//
//	ytDlpPath := `C:\Users\Premium\AppData\Local\Python\pythoncore-3.14-64\Scripts\yt-dlp.exe`
//
//	cmd := exec.Command(
//		ytDlpPath,
//		"--no-playlist",
//		"-o", output,
//		targetURL,
//	)
//	out, err := cmd.CombinedOutput()
//	if err != nil {
//		return "", fmt.Errorf("yt-dlp xatosi: %v\n\n%s", err, string(out))
//	}
//
//	files, err := filepath.Glob(strings.Replace(output, "%(ext)s", "*", 1))
//	if err != nil || len(files) == 0 {
//		return "", fmt.Errorf("video topilmadi")
//	}
//
//	return files[0], nil
//}
//
//// ====================== STATISTIKA ======================
//func showInstagramStatistics(bot *tgbotapi.BotAPI, b *models.CreatedBot, chatID int64) {
//	o := orm.NewOrm()
//	totalUsers, _ := o.QueryTable(new(models.BotUser)).Filter("Bot__Id", b.Id).Count()
//	vipUsers, _ := o.QueryTable(new(models.BotUser)).Filter("Bot__Id", b.Id).Filter("IsVip", true).Count()
//
//	text := fmt.Sprintf("📊 *Instagram Bot Statistika*\n\n"+
//		"👥 Jami foydalanuvchilar: *%d*\n"+
//		"⭐ VIP foydalanuvchilar: *%d*", totalUsers, vipUsers)
//
//	sendUserBot(bot, chatID, text)
//}
//
//func handleInstagramDownload(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
//	chatID := msg.Chat.ID
//	statusMsg := sendUserBot(bot, chatID, "⏳ Video yuklanmoqda... (Katta videolarga biroz vaqt ketishi mumkin)")
//
//	videoPath, err := downloadInstagramVideo(msg.Text)
//	if err != nil {
//		sendUserBot(bot, chatID, "❌ Yuklab bo‘lmadi: "+err.Error())
//		log.Println("Instagram error:", err)
//		return
//	}
//	defer os.Remove(videoPath)
//
//	file, err := os.Open(videoPath)
//	if err != nil {
//		sendUserBot(bot, chatID, "❌ Faylni ochishda xatolik yuz berdi.")
//		return
//	}
//	defer file.Close()
//
//	video := tgbotapi.NewVideo(chatID, tgbotapi.FileReader{
//		Name:   "instagram_video.mp4",
//		Reader: file,
//	})
//	video.Caption = "✅ @shadowwing_uz tizimi orqali yuklab berildi!"
//
//	_, err = bot.Send(video)
//	if err != nil {
//		file.Seek(0, 0)
//		doc := tgbotapi.NewDocument(chatID, tgbotapi.FileReader{
//			Name:   "instagram_video.mp4",
//			Reader: file,
//		})
//		doc.Caption = "✅ Yuklandi (Hujjat holatida)!"
//		bot.Send(doc)
//	}
//
//	delMsg := tgbotapi.NewDeleteMessage(chatID, statusMsg.MessageID)
//	bot.Send(delMsg)
//
//	saveDownloadStat(b, msg.From.ID)
//}
//
//func saveDownloadStat(b *models.CreatedBot, userID int64) {}
//
//func handleInstagramStart(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
//	text := "👋 *Instagram Downloader Botga xush kelibsiz!*\n\n" +
//		"🔗 Instagram reel yoki post linkini yuboring.\n" +
//		"Men sizga uni yuklab beraman."
//	sendUserBot(bot, msg.Chat.ID, text)
//}
//
//func handleBroadcastCommand(bot *tgbotapi.BotAPI, b *models.CreatedBot, msg *tgbotapi.Message) {
//	userID := msg.From.ID
//	var target string
//	switch msg.Text {
//	case "👥 Hammaga":
//		target = "all"
//	case "⭐ VIP'larga":
//		target = "vip"
//	case "👤 Oddiylarga":
//		target = "regular"
//	}
//
//	mu.Lock()
//	adminState[userID] = "waiting_broadcast_message:" + target
//	mu.Unlock()
//
//	sendUserBot(bot, msg.Chat.ID, "✉️ Yubormoqchi bo‘lgan xabarni jo‘nating (matn, rasm, video yoki forward):")
//}
