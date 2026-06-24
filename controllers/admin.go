package controllers

import (
	"creator/services"
	"golang.org/x/crypto/bcrypt"
	"strconv"
	"time"

	"creator/models"

	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
)

type AdminController struct {
	beego.Controller
}

// Login sahifasi
func (c *AdminController) LoginPage() {
	if c.GetSession("admin_logged_in") != nil {
		c.Redirect("/admin/dashboard", 302)
		return
	}
	c.TplName = "login.html"
}

func (c *AdminController) Login() {
	username := c.GetString("username")
	password := c.GetString("password")

	o := orm.NewOrm()
	admin := models.Admin{}

	err := o.QueryTable("admin").
		Filter("username", username).
		Filter("is_active", true).
		One(&admin)

	if err != nil {
		c.Data["Error"] = "Login yoki parol noto'g'ri!"
		c.TplName = "login.html"
		return
	}

	// Parol tekshirish
	err = bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(password))
	if err != nil {
		c.Data["Error"] = "Login yoki parol noto'g'ri!"
		c.TplName = "login.html"
		return
	}

	// Sessiya ma'lumotlarini saqlash
	c.SetSession("admin_logged_in", true)
	c.SetSession("admin_id", admin.Id)
	c.SetSession("admin_username", admin.Username)
	c.SetSession("admin_role", admin.Role) // Role ni ham saqlaymiz

	c.Redirect("/admin/dashboard", 302)
}

func (c *AdminController) isAdminAllowed() bool {
	role := c.GetSession("admin_role")
	if role == nil {
		return false
	}
	roleStr := role.(string)
	return roleStr == "superadmin" || roleStr == "admin"
}

func (c *AdminController) Dashboard() {
	// Role tekshiruvi
	if !c.isAdminAllowed() {
		c.Redirect("/admin/login", 302)
		return
	}

	o := orm.NewOrm()

	// Statistikalar
	userCount, _ := o.QueryTable(new(models.UserBot)).Count()
	botCount, _ := o.QueryTable(new(models.CreatedBot)).Count()
	activeBotCount, _ := o.QueryTable(new(models.CreatedBot)).Filter("IsActive", true).Count()
	inactiveBotCount, _ := o.QueryTable(new(models.CreatedBot)).Filter("IsActive", false).Count()
	botTypeCount, _ := o.QueryTable(new(models.BotType)).Count()

	// Unikal foydalanuvchilar
	var botUserCount int64
	err := o.Raw("SELECT COUNT(DISTINCT tg_id) FROM bot_user").QueryRow(&botUserCount)
	if err != nil {
		botUserCount = 0
	}

	// Oxirgi botlar
	var latestBots []models.CreatedBot
	_, err = o.QueryTable(new(models.CreatedBot)).
		RelatedSel("BotType", "Owner").
		OrderBy("-Id").
		Limit(20).
		All(&latestBots)
	if err != nil {
		latestBots = make([]models.CreatedBot, 0)
	}

	// Oxirgi foydalanuvchilar
	var users []*models.BotUser
	_, usersErr := o.QueryTable(new(models.BotUser)).
		RelatedSel("Bot").
		OrderBy("-JoinedAt").
		Limit(100).
		All(&users)
	if usersErr != nil {
		users = make([]*models.BotUser, 0)
	}

	// Ma'lumotlarni shablonga uzatish
	c.Data["UserCount"] = userCount
	c.Data["BotCount"] = botCount
	c.Data["ActiveBotCount"] = activeBotCount
	c.Data["InactiveBotCount"] = inactiveBotCount
	c.Data["BotUserCount"] = botUserCount
	c.Data["BotTypeCount"] = botTypeCount
	c.Data["LatestBots"] = latestBots
	c.Data["Users"] = users
	c.Data["RealUserCount"] = len(users)

	c.TplName = "index.html"
}

// ==================== LOGOUT ====================
func (c *AdminController) Logout() {
	c.DestroySession()
	c.Redirect("/admin/login", 302)
}

func (c *AdminController) GetBotDetails() {
	o := orm.NewOrm()
	botIdStr := c.Ctx.Input.Param(":id")

	var bot models.CreatedBot
	err := o.QueryTable(new(models.CreatedBot)).
		Filter("Id", botIdStr).
		RelatedSel("BotType", "Owner").
		One(&bot)

	if err != nil {
		c.Redirect("/admin", 302)
		return
	}

	var users []*models.BotUser
	_, usersErr := o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", bot.Id).
		OrderBy("-JoinedAt").
		All(&users)

	if usersErr != nil {
		users = make([]*models.BotUser, 0)
	}
	c.Data["Bot"] = &bot
	c.Data["Users"] = users
	c.Data["IsEditing"] = false
	c.Data["Title"] = bot.BotName + " - Boshqarish"

	contentCount, videoCount, adminCount := getBotStats(o, &bot)
	c.Data["ContentCount"] = contentCount
	c.Data["VideoCount"] = videoCount
	c.Data["AdminCount"] = adminCount

	c.TplName = "index.html"
}

func (c *AdminController) EditBotPage() {
	o := orm.NewOrm()
	botIdStr := c.Ctx.Input.Param(":id")

	var bot models.CreatedBot
	err := o.QueryTable(new(models.CreatedBot)).Filter("Id", botIdStr).RelatedSel("BotType", "Owner").One(&bot)
	if err != nil {
		c.Redirect("/admin", 302)
		return
	}

	var users []*models.BotUser
	_, _ = o.QueryTable(new(models.BotUser)).Filter("Bot__Id", bot.Id).All(&users)

	c.Data["Bot"] = &bot
	c.Data["Users"] = users
	c.Data["IsEditing"] = true

	contentCount, videoCount, adminCount := getBotStats(o, &bot)
	c.Data["ContentCount"] = contentCount
	c.Data["VideoCount"] = videoCount
	c.Data["AdminCount"] = adminCount

	c.TplName = "index.html"
}

func (c *AdminController) AllBotUsers() {
	o := orm.NewOrm()
	var users []*models.BotUser

	// Qidiruv parametrini olish: /admin/all-users?tg_id=123456789
	searchTgId := c.GetString("tg_id")

	qs := o.QueryTable(new(models.BotUser)).OrderBy("-JoinedAt")

	if searchTgId != "" {
		// Telegram ID bo'yicha aniq qidirish
		tgIdInt, err := strconv.ParseInt(searchTgId, 10, 64)
		if err == nil {
			qs = qs.Filter("TgId", tgIdInt)
		} else {
			// Agar son bo'lmasa, bo'sh natija qaytaramiz
			c.Data["Users"] = []*models.BotUser{}
			c.Data["BotUserCount"] = 0
			c.Data["IsAllUsersPage"] = true
			c.Data["SearchTgId"] = searchTgId
			c.TplName = "index.html"
			return
		}
	}

	_, err := qs.All(&users)
	if err != nil {
		users = []*models.BotUser{}
	}

	// Har bir user uchun botini alohida yuklaymiz (RelatedSel INNER JOIN qilib userlarni yo'qotmasligi uchun)
	for _, u := range users {
		if u.Bot != nil {
			if readErr := o.Read(u.Bot); readErr != nil {
				u.Bot = nil
			}
		}
	}

	var botUserCount int64
	_ = o.Raw("SELECT COUNT(DISTINCT tg_id) FROM bot_user").QueryRow(&botUserCount)

	c.Data["Users"] = users
	c.Data["BotUserCount"] = botUserCount
	c.Data["IsAllUsersPage"] = true
	c.Data["SearchTgId"] = searchTgId // formada kiritilgan qiymatni saqlab qolish uchun

	c.TplName = "index.html"
}

func (c *AdminController) AllAdmins() {
	o := orm.NewOrm()
	var admins []*models.UserBot

	_, err := o.QueryTable(new(models.UserBot)).OrderBy("-Id").All(&admins)
	if err != nil {
		admins = []*models.UserBot{}
	}

	c.Data["Admins"] = admins
	c.Data["IsAllAdminsPage"] = true

	c.TplName = "index.html"
}

func (c *AdminController) GetBotDetail() {
	o := orm.NewOrm()

	idStr := c.Ctx.Input.Param(":id")
	botId, _ := strconv.ParseInt(idStr, 10, 64)

	// 1. Bot ma'lumotlarini olish
	bot := models.CreatedBot{Id: botId}
	err := o.QueryTable(new(models.CreatedBot)).
		Filter("Id", botId).
		RelatedSel("Owner", "BotType").
		One(&bot)

	if err != nil {
		// Agar bot topilmasa, HTML sahifaga xatolik yuboramiz yoki 404 beramiz
		c.Data["Error"] = "Xatolik: Bot topilmadi yoki o'chirilgan!"
		c.TplName = "error.html" // yoki index.html ichida xatolikni tekshirasiz
		return
	}

	// 2. Botga tegishli foydalanuvchilarni olish
	var users []*models.BotUser
	_, err = o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", botId).
		OrderBy("-JoinedAt").
		All(&users)

	if err != nil {
		// Foydalanuvchilarni olishda xatolik bo'lsa log yozish yoki boshqarish mumkin
		users = []*models.BotUser{} // bo'sh saqlaymiz
	}

	// 3. Ma'lumotlarni HTML (Template) ga uzatish
	c.Data["Bot"] = &bot
	c.Data["Users"] = users
	c.Data["IsEditing"] = false

	contentCount, videoCount, adminCount := getBotStats(o, &bot)
	c.Data["ContentCount"] = contentCount
	c.Data["VideoCount"] = videoCount
	c.Data["AdminCount"] = adminCount
	c.Data["RealUserCount"] = len(users) // 🔑 shu

	c.TplName = "index.html"
}

func (c *AdminController) ToggleBot() {
	o := orm.NewOrm()
	idStr := c.Ctx.Input.Param(":id")
	botId, _ := strconv.ParseInt(idStr, 10, 64)

	var bot models.CreatedBot
	err := o.QueryTable(new(models.CreatedBot)).
		Filter("Id", botId).
		RelatedSel("BotType", "Owner").
		One(&bot)

	if err == nil {
		bot.IsActive = !bot.IsActive
		o.Update(&bot, "IsActive", "UpdatedAt")

		if bot.IsActive && !bot.IsSuspended {
			services.StartBot(&bot) // ✅ to'g'ri chaqiruv
		} else {
			services.StopBot(bot.Id)
		}
	}
	c.Redirect("/admin/bots/"+idStr, 302)
}

func (c *AdminController) RegenerateToken() {
	o := orm.NewOrm()
	idStr := c.Ctx.Input.Param(":id")
	botId, _ := strconv.ParseInt(idStr, 10, 64)

	var bot models.CreatedBot
	err := o.QueryTable(new(models.CreatedBot)).
		Filter("Id", botId).
		RelatedSel("BotType", "Owner").
		One(&bot)

	if err == nil {
		bot.Token = "YANGI_TOKEN_KODI"
		o.Update(&bot, "Token", "UpdatedAt")

		if bot.IsActive {
			services.StartBot(&bot) // ✅ to'g'ri chaqiruv
		}
	}
	c.Redirect("/admin/bots/"+idStr, 302)
}

func (c *AdminController) DeleteBot() {
	o := orm.NewOrm()
	idStr := c.Ctx.Input.Param(":id")
	botId, _ := strconv.ParseInt(idStr, 10, 64)

	// Avval botni to'xtatamiz
	services.StopBot(botId)

	_, err := o.QueryTable(new(models.BotUser)).Filter("Bot__Id", botId).Delete()
	if err != nil {
		c.Ctx.WriteString("Bot foydalanuvchilarini o'chirishda xatolik: " + err.Error())
		return
	}

	bot := models.CreatedBot{Id: botId}
	_, err = o.Delete(&bot)
	if err != nil {
		c.Ctx.WriteString("Botni o'chirishda xatolik: " + err.Error())
		return
	}

	c.Redirect("/", 302)
}

func (c *AdminController) EditBot() {
	o := orm.NewOrm()
	idStr := c.Ctx.Input.Param(":id")
	botId, _ := strconv.ParseInt(idStr, 10, 64)

	bot := models.CreatedBot{Id: botId}
	err := o.QueryTable(new(models.CreatedBot)).
		Filter("Id", botId).
		RelatedSel("Owner", "BotType").
		One(&bot)

	if err != nil {
		c.Ctx.WriteString("Bot topilmadi!")
		return
	}

	var users []*models.BotUser
	_, userErr := o.QueryTable(new(models.BotUser)).Filter("Bot__Id", botId).OrderBy("-JoinedAt").All(&users)
	if userErr != nil {
		users = make([]*models.BotUser, 0)
	}

	c.Data["Bot"] = &bot
	c.Data["Users"] = users
	c.Data["IsEditing"] = true

	c.TplName = "index.html"
}

func (c *AdminController) UpdateBot() {
	o := orm.NewOrm()
	idStr := c.Ctx.Input.Param(":id")
	botId, _ := strconv.ParseInt(idStr, 10, 64)

	var bot models.CreatedBot
	err := o.QueryTable(new(models.CreatedBot)).
		Filter("Id", botId).
		RelatedSel("BotType", "Owner").
		One(&bot)

	if err != nil {
		c.Ctx.WriteString("Xatolik: Bot topilmadi!")
		return
	}

	inputToken := c.GetString("token")
	tgName, tgUsername, tgErr := getBotInfoFromTelegram(inputToken)

	if tgErr == nil {
		bot.BotName = tgName
		bot.BotUsername = "@" + tgUsername
		bot.Token = inputToken
	} else {
		bot.BotName = c.GetString("bot_name")
		bot.BotUsername = c.GetString("bot_username")
		bot.Token = inputToken
	}

	bot.UpdatedAt = time.Now()
	_, updateErr := o.Update(&bot, "BotName", "BotUsername", "Token", "UpdatedAt")
	if updateErr != nil {
		c.Ctx.WriteString("Bazaga saqlashda xatolik: " + updateErr.Error())
		return
	}

	// Token yangilangach botni restart qilamiz
	if bot.IsActive && !bot.IsSuspended {
		services.StartBot(&bot) // ✅ to'g'ri chaqiruv
	}

	c.Redirect("/admin/bots/"+idStr, 302)
}

func (c *AdminController) BotUserDetail() {
	o := orm.NewOrm()
	idStr := c.Ctx.Input.Param(":id")
	userId, _ := strconv.ParseInt(idStr, 10, 64)

	var botUser models.BotUser
	err := o.QueryTable(new(models.BotUser)).
		Filter("Id", userId).
		RelatedSel("Bot").
		One(&botUser)

	if err != nil {
		c.Data["Error"] = "Foydalanuvchi topilmadi!"
		c.TplName = "error.html"
		return
	}

	c.Data["BotUser"] = &botUser
	c.Data["IsBotUserDetailPage"] = true
	c.Data["Title"] = "Foydalanuvchi: " + botUser.Username

	c.TplName = "index.html"
}

func (c *AdminController) DeleteJoinRequest() {
	o := orm.NewOrm()
	idStr := c.Ctx.Input.Param(":id")
	reqId, _ := strconv.ParseInt(idStr, 10, 64)

	req := models.BotJoinRequest{Id: reqId}
	_, err := o.Delete(&req)

	if err != nil {
		c.Ctx.WriteString("Arizani o'chirishda xatolik yuz berdi: " + err.Error())
		return
	}

	// O'chirilgandan so'ng yana shu sahifaga qaytadi (Qayta yuklanadi)
	c.Redirect("/admin/join-requests", 302)
}

func (c *AdminController) AllJoinRequests() {
	o := orm.NewOrm()
	var requests []*models.BotJoinRequest

	// Diqqat: .RelatedSel() yoki .RelatedSel("Bot") majburiy!
	_, err := o.QueryTable(new(models.BotJoinRequest)).
		RelatedSel().
		OrderBy("-Id").
		All(&requests)

	if err != nil {
		// Xatolik nimaligini terminalda ko'rish uchun log bosamiz
		println("ORM Xatolik yuz berdi:", err.Error())
		requests = []*models.BotJoinRequest{}
	}

	// HTML-ga uzatiladigan nom shablondagi bilan bir xil bo'lishi shart (.Requests)
	c.Data["Requests"] = requests
	c.Data["IsJoinRequestsPage"] = true
	c.Data["Title"] = "Guruh/Kanalga Qo'shilish Arizalari"

	c.TplName = "index.html"
}

func (c *AdminController) UpdateAdminBalance() {
	o := orm.NewOrm()
	idStr := c.Ctx.Input.Param(":id")
	adminId, _ := strconv.ParseInt(idStr, 10, 64)

	var admin models.UserBot
	err := o.QueryTable(new(models.UserBot)).Filter("Id", adminId).One(&admin)
	if err != nil {
		c.Ctx.WriteString("Admin topilmadi!")
		return
	}

	mode := c.GetString("mode") // "set" yoki "adjust"
	amountStr := c.GetString("amount")

	amount, parseErr := strconv.ParseFloat(amountStr, 64)
	if parseErr != nil {
		c.Redirect("/admin/users", 302)
		return
	}

	switch mode {
	case "set":
		// Aniq qiymat qilib o'rnatish
		admin.Balance = amount
	case "adjust":
		// Qo'shish/ayirish (amount manfiy bo'lsa ayriladi, masalan -5000)
		admin.Balance += amount
	default:
		c.Redirect("/admin/users", 302)
		return
	}

	// Balans manfiy bo'lib qolmasligi uchun himoya
	if admin.Balance < 0 {
		admin.Balance = 0
	}

	_, updateErr := o.Update(&admin, "Balance")
	if updateErr != nil {
		c.Ctx.WriteString("Balansni yangilashda xatolik: " + updateErr.Error())
		return
	}

	c.Redirect("/admin/users", 302)
}

func (c *AdminController) ResetAdminBalance() {
	o := orm.NewOrm()
	idStr := c.Ctx.Input.Param(":id")
	adminId, _ := strconv.ParseInt(idStr, 10, 64)

	var admin models.UserBot
	err := o.QueryTable(new(models.UserBot)).Filter("Id", adminId).One(&admin)
	if err != nil {
		c.Ctx.WriteString("Admin topilmadi!")
		return
	}

	admin.Balance = 0
	_, updateErr := o.Update(&admin, "Balance")
	if updateErr != nil {
		c.Ctx.WriteString("Balansni yangilashda xatolik: " + updateErr.Error())
		return
	}

	c.Redirect("/admin/users", 302)
}

func (c *AdminController) MakeBotUserVip() {
	o := orm.NewOrm()
	idStr := c.Ctx.Input.Param(":id")
	userId, _ := strconv.ParseInt(idStr, 10, 64)

	var botUser models.BotUser
	err := o.QueryTable(new(models.BotUser)).Filter("Id", userId).One(&botUser)
	if err == nil {
		botUser.IsVip = !botUser.IsVip
		o.Update(&botUser, "IsVip")
	}

	c.Redirect("/admin/bot-users/"+idStr, 302)
}

func (c *AdminController) BlockBotUser() {
	o := orm.NewOrm()
	idStr := c.Ctx.Input.Param(":id")
	userId, _ := strconv.ParseInt(idStr, 10, 64)

	var botUser models.BotUser
	err := o.QueryTable(new(models.BotUser)).Filter("Id", userId).One(&botUser)
	if err == nil {
		botUser.IsBlocked = !botUser.IsBlocked
		o.Update(&botUser, "IsBlocked")
	}

	c.Redirect("/admin/bot-users/"+idStr, 302)
}

func getBotStats(o orm.Ormer, bot *models.CreatedBot) (contentCount, videoCount, adminCount int64) {
	switch bot.BotType.Code {
	case "anime":
		contentCount, _ = o.QueryTable(new(models.Anime)).
			Filter("Bot__Id", bot.Id).
			Count()

		videoCount, _ = o.QueryTable(new(models.AnimePart)).
			Filter("Anime__Bot__Id", bot.Id).
			Count()

	case "kino":
		contentCount, _ = o.QueryTable(new(models.Kino)).
			Filter("Bot__Id", bot.Id).
			Count()

		videoCount, _ = o.QueryTable(new(models.KinoPart)).
			Filter("Kino__Bot__Id", bot.Id).
			Count()
	}

	adminCount, _ = o.QueryTable(new(models.BotUser)).
		Filter("Bot__Id", bot.Id).
		Filter("IsAdmin", true).
		Count()

	return
}
