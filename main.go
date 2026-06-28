package main

import (
	"creator/controllers"
	"creator/database"
	"creator/models"
	"creator/services"
	"fmt"
	"log"

	_ "creator/routers"

	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	database.InitDB()
	database.SeedBotTypes()

	creatorToken, err := beego.AppConfig.String("creator_bot_token")
	if err != nil || creatorToken == "" {
		log.Fatal("❌ creator_bot_token conf/app.conf da yo'q!")
	}
	fmt.Println("Token:", creatorToken)

	if err := controllers.InitCreatorBot(creatorToken); err != nil {
		log.Fatalf("❌ Creator bot xatosi: %v", err)
	}

	services.OnMessage = controllers.HandleUserBotMessage
	services.OnCallback = controllers.HandleUserBotCallbackQuery
	services.OnJoinRequest = controllers.SaveJoinRequest

	services.SetSharedCreatorBot(controllers.CreatorBot)

	services.RestoreActiveBots()
	services.StartFastBillingChecker()
	services.StartDailyBillingScheduler()

	controllers.StartExpiredInvoiceCleaner()

	CreateDefaultAdmin()

	beego.Run()
}

func CreateDefaultAdmin() {
	o := orm.NewOrm()

	defaultPassword, err := beego.AppConfig.String("admin_panel_pass")
	if err != nil || defaultPassword == "" {
		defaultPassword = "admin123"
	}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcrypt.DefaultCost)

	admin := models.Admin{Username: "admin"}
	if o.Read(&admin, "Username") == nil {
		admin.Password = string(hashedPassword)
		o.Update(&admin, "Password")
		fmt.Println("🔄 Admin paroli yangi [admin_panel_pass] ga yangilandi!")
	} else {
		admin.Password = string(hashedPassword)
		admin.FullName = "Super Administrator"
		admin.Email = "admin@yourdomain.com"
		admin.Role = "superadmin"
		admin.IsActive = true
		o.Insert(&admin)
		fmt.Println("🎉 Yangi default admin yaratildi!")
	}

	fmt.Println("   Login : admin")
	fmt.Printf("   Parol  : %s\n", defaultPassword)
}
