package database

import (
	"creator/models"
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
)

func InitDB() {
	host := beego.AppConfig.DefaultString("db_host", "localhost")
	port := beego.AppConfig.DefaultString("db_port", "5432")
	name := beego.AppConfig.DefaultString("db_name", "creator")
	user := beego.AppConfig.DefaultString("db_user", "admin")
	pass := beego.AppConfig.DefaultString("db_pass", "admin")

	dsn := fmt.Sprintf(
		"host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
		host, port, name, user, pass,
	)

	orm.RegisterDriver("postgres", orm.DRPostgres)
	orm.RegisterDataBase("default", "postgres", dsn)
	orm.RunSyncdb("default", false, true) // Birinchi argument FALSE bo'lishi shart!
}

func SeedBotTypes() {
	o := orm.NewOrm()

	types := []models.BotType{
		{
			Name:        "Anime bot",
			Code:        "anime",
			Description: "Anime ma'lumotlari, rasmlar, personajlar",
		},
		{
			Name:        "Oshxona boti",
			Code:        "food",
			Description: "Retseptlar, ovqat buyurtma",
		},
		{
			Name:        "Ma'lumot boti",
			Code:        "info",
			Description: "Umumiy ma'lumot beruvchi bot",
		},
		{
			Name:        "Do'kon boti",
			Code:        "shop",
			Description: "Mahsulot sotish boti",
		},
	}

	for _, t := range types {
		o.ReadOrCreate(&t, "Code")
	}
}
