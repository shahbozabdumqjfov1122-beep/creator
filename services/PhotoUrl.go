package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"creator/models"

	"github.com/beego/beego/v2/client/orm"
)

type getChatResponse struct {
	Ok     bool `json:"ok"`
	Result struct {
		Photo struct {
			BigFileId string `json:"big_file_id"`
		} `json:"photo"`
	} `json:"result"`
}

type getFileResponse struct {
	Ok     bool `json:"ok"`
	Result struct {
		FilePath string `json:"file_path"`
	} `json:"result"`
}

// FetchAndSaveBotPhoto bot o'z tokeni orqali profil rasmini oladi,
// lokal diskka saqlaydi va web orqali ochiladigan yo'lni qaytaradi.
func FetchAndSaveBotPhoto(botToken string, tgBotId int64, dbBotId int64) (string, error) {
	// 1. getChat orqali big_file_id olish
	chatUrl := fmt.Sprintf("https://api.telegram.org/bot%s/getChat?chat_id=%d", botToken, tgBotId)
	resp, err := http.Get(chatUrl)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var chatResp getChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", err
	}
	if !chatResp.Ok || chatResp.Result.Photo.BigFileId == "" {
		return "", fmt.Errorf("bot rasmga ega emas")
	}

	// 2. getFile orqali file_path olish
	fileUrl := fmt.Sprintf("https://api.telegram.org/bot%s/getFile?file_id=%s", botToken, chatResp.Result.Photo.BigFileId)
	resp2, err := http.Get(fileUrl)
	if err != nil {
		return "", err
	}
	defer resp2.Body.Close()

	body2, _ := io.ReadAll(resp2.Body)
	var fileResp getFileResponse
	if err := json.Unmarshal(body2, &fileResp); err != nil {
		return "", err
	}
	if !fileResp.Ok {
		return "", fmt.Errorf("file_path olinmadi")
	}

	// 3. Faylni yuklab olish
	downloadUrl := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", botToken, fileResp.Result.FilePath)
	imgResp, err := http.Get(downloadUrl)
	if err != nil {
		return "", err
	}
	defer imgResp.Body.Close()

	// 4. Papka yaratish
	if err := os.MkdirAll("static/bot_photos", 0755); err != nil {
		return "", err
	}

	savePath := fmt.Sprintf("static/bot_photos/%d.jpg", dbBotId)
	out, err := os.Create(savePath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, imgResp.Body); err != nil {
		return "", err
	}

	return "/" + savePath, nil
}

// SyncAllMissingPhotos - rasm yo'q barcha botlarning profil rasmini yuklab oladi
func SyncAllMissingPhotos() {
	o := orm.NewOrm()

	var bots []models.CreatedBot

	cond := orm.NewCondition().
		And("PhotoUrl__isnull", true).
		Or("PhotoUrl", "")

	_, err := o.QueryTable(new(models.CreatedBot)).
		SetCond(cond).
		All(&bots)

	if err != nil {
		log.Printf("botlarni olishda xato: %v", err)
		return
	}

	if len(bots) == 0 {
		log.Println("Barcha botlarda rasm mavjud")
		return
	}

	log.Printf("%d ta botda rasm yo‘q, yuklab olish boshlandi...", len(bots))

	for _, bot := range bots {
		if bot.Token == "" {
			continue
		}

		path, err := FetchAndSaveBotPhoto(bot.Token, bot.TgId, bot.Id)
		if err != nil {
			log.Printf("Bot ID %d (%s) rasm ololmadi: %v", bot.Id, bot.BotName, err)
			continue
		}

		bot.PhotoUrl = path
		if _, err := o.Update(&bot, "PhotoUrl"); err != nil {
			log.Printf("Bot ID %d photo_url saqlanmadi: %v", bot.Id, err)
			continue
		}

		log.Printf("Bot ID %d rasm saqlandi: %s", bot.Id, path)
		time.Sleep(400 * time.Millisecond)
	}

	log.Println("Barcha eski botlarning rasmlari yakunlandi")
}

type getUserProfilePhotosResponse struct {
	Ok     bool `json:"ok"`
	Result struct {
		TotalCount int `json:"total_count"`
		Photos     [][]struct {
			FileId string `json:"file_id"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		} `json:"photos"`
	} `json:"result"`
}

// FetchAndSaveUserPhoto - bot tokeni orqali foydalanuvchi profil rasmini oladi
func FetchAndSaveUserPhoto(botToken string, userTgId int64, dbUserId int64) (string, error) {
	savePath := fmt.Sprintf("static/user_photos/%d.jpg", dbUserId)
	webPath := "/" + savePath

	// 1. Fayl allaqachon bormi? → qayta yuklamaymiz
	if _, err := os.Stat(savePath); err == nil {
		return webPath, nil
	}

	// 2. Telegramdan olish
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUserProfilePhotos?user_id=%d&limit=1", botToken, userTgId)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var photosResp getUserProfilePhotosResponse
	if err := json.Unmarshal(body, &photosResp); err != nil {
		return "", err
	}
	if !photosResp.Ok || photosResp.Result.TotalCount == 0 || len(photosResp.Result.Photos) == 0 {
		return "", fmt.Errorf("foydalanuvchida rasm yo'q")
	}

	photoSizes := photosResp.Result.Photos[0]
	fileId := photoSizes[len(photoSizes)-1].FileId

	fileUrl := fmt.Sprintf("https://api.telegram.org/bot%s/getFile?file_id=%s", botToken, fileId)
	resp2, err := http.Get(fileUrl)
	if err != nil {
		return "", err
	}
	defer resp2.Body.Close()

	body2, _ := io.ReadAll(resp2.Body)
	var fileResp getFileResponse
	if err := json.Unmarshal(body2, &fileResp); err != nil {
		return "", err
	}
	if !fileResp.Ok {
		return "", fmt.Errorf("file_path olinmadi")
	}

	downloadUrl := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", botToken, fileResp.Result.FilePath)
	imgResp, err := http.Get(downloadUrl)
	if err != nil {
		return "", err
	}
	defer imgResp.Body.Close()

	if err := os.MkdirAll("static/user_photos", 0755); err != nil {
		return "", err
	}

	out, err := os.Create(savePath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, imgResp.Body); err != nil {
		return "", err
	}

	return webPath, nil
}
func SyncAllMissingUserPhotos() {
	o := orm.NewOrm()

	var users []models.BotUser
	cond := orm.NewCondition().
		And("PhotoUrl__isnull", true).
		Or("PhotoUrl", "")

	_, err := o.QueryTable(new(models.BotUser)).
		SetCond(cond).
		RelatedSel("Bot"). // Bot.Token kerak
		All(&users)

	if err != nil {
		log.Printf("foydalanuvchilarni olishda xato: %v", err)
		return
	}

	log.Printf("%d ta foydalanuvchida rasm yo‘q...", len(users))

	for _, user := range users {
		if user.Bot == nil || user.Bot.Token == "" {
			continue
		}

		path, err := FetchAndSaveUserPhoto(user.Bot.Token, user.TgId, user.Id)
		if err != nil {
			log.Printf("User ID %d rasm ololmadi: %v", user.Id, err)
			continue
		}

		user.PhotoUrl = path
		if _, err := o.Update(&user, "PhotoUrl"); err != nil {
			log.Printf("User ID %d photo_url saqlanmadi: %v", user.Id, err)
			continue
		}

		log.Printf("User ID %d rasm saqlandi", user.Id)
		time.Sleep(350 * time.Millisecond)
	}

	log.Println("Foydalanuvchilar rasmlari yakunlandi")
}
