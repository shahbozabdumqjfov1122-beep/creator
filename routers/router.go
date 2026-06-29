package routers

import (
	"creator/controllers"
	beego "github.com/beego/beego/v2/server/web"
)

func init() {
	// ====================== PUBLIC ROUTES ======================
	beego.Router("/admin/login", &controllers.AdminController{}, "get:LoginPage;post:Login")

	// ====================== ADMIN ROUTES (Himoyalangan) ======================
	beego.Router("/admin/logout", &controllers.AdminController{}, "get:Logout")

	// Asosiy sahifalar
	beego.Router("/", &controllers.AdminController{}, "get:Dashboard")
	beego.Router("/admin/dashboard", &controllers.AdminController{}, "get:Dashboard")
	beego.Router("/admin/bots", &controllers.AdminController{}, "get:Dashboard")

	// Bot boshqaruv
	beego.Router("/admin/bots/:id", &controllers.AdminController{}, "get:GetBotDetail")
	beego.Router("/admin/bots/:id/toggle", &controllers.AdminController{}, "get:ToggleBot")
	beego.Router("/admin/bots/:id/regenerate-token", &controllers.AdminController{}, "get:RegenerateToken")
	beego.Router("/admin/bots/:id/delete", &controllers.AdminController{}, "get:DeleteBot")
	beego.Router("/admin/bots/:id/edit", &controllers.AdminController{}, "get:EditBot;post:UpdateBot")

	// Bot Users
	beego.Router("/admin/bot-users/:id", &controllers.AdminController{}, "get:BotUserDetail")
	beego.Router("/admin/bot-users/:id/block", &controllers.AdminController{}, "post:BlockBotUser")
	beego.Router("/admin/bot-users/:id/vip", &controllers.AdminController{}, "post:MakeBotUserVip")
	beego.Router("/admin/all-users", &controllers.AdminController{}, "get:AllBotUsers")

	// Admin Users
	beego.Router("/admin/users", &controllers.AdminController{}, "get:AllAdmins")
	beego.Router("/admin/users/:id/balance", &controllers.AdminController{}, "post:UpdateAdminBalance")
	beego.Router("/admin/users/:id/balance/reset", &controllers.AdminController{}, "post:ResetAdminBalance")

	// Join Requests
	beego.Router("/admin/join-requests", &controllers.AdminController{}, "get:AllJoinRequests")
	beego.Router("/admin/join-requests/delete/:id", &controllers.AdminController{}, "get:DeleteJoinRequest")

	// Yangi (toggle-block ga moslashtirdik)
	beego.Router("/admin/bot-users/:id/toggle-block", &controllers.AdminController{}, "get:BlockBotUser") // GET yoki POST

	beego.InsertFilter("/admin/*", beego.BeforeRouter, controllers.AdminAuthFilter)
	beego.InsertFilter("/", beego.BeforeRouter, controllers.AdminAuthFilter)

}
