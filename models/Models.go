package models

import (
	"time"

	"github.com/beego/beego/v2/client/orm"
	_ "github.com/lib/pq"
)

type UserBot struct {
	Id        int64     `orm:"auto;pk"`
	TgId      int64     `orm:"unique;column(tg_id)"`
	Username  string    `orm:"size(100);null"`
	Balance   float64   `orm:"digits(18);decimals(2)"`
	CreatedAt time.Time `orm:"auto_now_add;type(datetime);column(created_at)"`
}

func (u *UserBot) TableName() string {
	return "user_bot"
}

type CreatedBot struct {
	Id                 int64     `orm:"auto;pk"`
	Owner              *UserBot  `orm:"rel(fk);column(owner_id)"`       // Kim yaratdi (Foreign Key)
	BotType            *BotType  `orm:"rel(fk);column(bot_type_id)"`    // Qanday bot (Foreign Key)
	Token              string    `orm:"size(200);unique"`               // BotFather tokeni
	BotUsername        string    `orm:"size(100);column(bot_username)"` // @username
	BotName            string    `orm:"size(200);column(bot_name)"`
	IsActive           bool      `orm:"default(true);column(is_active)"`
	UserCount          int64     `orm:"default(0);column(user_count)"`
	IsSuspended        bool      `orm:"default(false);column(is_suspended)"`         // To'xtatilganmi (admin qo'lda)
	IsBalanceSuspended bool      `orm:"default(false);column(is_balance_suspended)"` // Balans tugagani uchun to'xtatilgan
	LastChargedAt      time.Time `orm:"type(datetime);null;column(last_charged_at)"` // Oxirgi marta kunlik to'lov yechilgan vaqt
	TrialEndsAt        time.Time `orm:"type(datetime);null;column(trial_ends_at)"`   // Trial tugash vaqti
	PaidUntil          time.Time `orm:"type(datetime);null;column(paid_until)"`      // To'lov qilingan sana
	CreatedAt          time.Time `orm:"auto_now_add;type(datetime);column(created_at)"`
	UpdatedAt          time.Time `orm:"auto_now;type(datetime);column(updated_at)"`
}

func (c *CreatedBot) TableName() string {
	return "created_bot"
}

type BotInvoice struct {
	Id          int64       `orm:"auto;pk"`
	Bot         *CreatedBot `orm:"rel(fk);null;column(bot_id)"`                        // Qaysi bot orqali pul tushyapti (ixtiyoriy)
	UserId      int64       `orm:"column(user_id)"`                                    // To'lov qilgan odamning Telegram ID si
	Amount      float64     `orm:"digits(12);decimals(2)"`                             // Foydalanuvchi kiritgan sof summa
	FinalAmount float64     `orm:"unique;digits(12);decimals(2);column(final_amount)"` // Tiyini bilan unikal jami summa
	Diff        int         `orm:"column(diff)"`                                       // Qo'shilgan farq (Masalan: 11)
	Status      string      `orm:"size(20);default(pending)"`                          // "pending", "paid", "expired"
	CreatedAt   time.Time   `orm:"auto_now_add;type(datetime);column(created_at)"`
	ExpiresAt   time.Time   `orm:"type(datetime);column(expires_at)"` // 5 daqiqalik cheklov
}

func (b *BotInvoice) TableName() string {
	return "bot_invoice"
}

type BotType struct {
	Id          int64  `orm:"auto;pk"`
	Name        string `orm:"size(100)"`
	Code        string `orm:"size(50);unique"`
	Description string `orm:"size(500);null"`
	IsActive    bool   `orm:"default(true);column(is_active)"`
}

func (b *BotType) TableName() string {
	return "bot_type"
}

type Anime struct {
	Id         int64       `orm:"auto;pk"`
	Bot        *CreatedBot `orm:"rel(fk);column(bot_id)"`
	Name       string      `orm:"size(300)"`
	Code       string      `orm:"size(100)"`
	PhotoID    string      `orm:"size(500);column(photo_id)"`
	PartsCount int         `orm:"default(0);column(parts_count)"`
	IsActive   bool        `orm:"default(true);column(is_active)"`
	CreatedAt  time.Time   `orm:"auto_now_add;type(datetime);column(created_at)"`
	UpdatedAt  time.Time   `orm:"auto_now;type(datetime);column(updated_at)"`
}

func (a *Anime) TableName() string {
	return "anime"
}

type AnimePart struct {
	Id        int64     `orm:"auto;pk"`
	Anime     *Anime    `orm:"rel(fk);column(anime_id)"`
	Kind      string    `orm:"size(20);column(kind)"`
	FileID    string    `orm:"size(500);column(file_id)"`
	MessageID int       `orm:"column(message_id)"`
	PartOrder int       `orm:"default(0);column(part_order)"`
	CreatedAt time.Time `orm:"auto_now_add;type(datetime);column(created_at)"`
}

func (p *AnimePart) TableName() string {
	return "anime_part"
}

type BotUser struct {
	Id        int64       `orm:"auto;pk"`
	Bot       *CreatedBot `orm:"rel(fk);column(bot_id)"`
	TgId      int64       `orm:"column(tg_id)"`
	Username  string      `orm:"size(100);null"`
	FirstName string      `orm:"size(200);column(first_name);null"`
	LastName  string      `orm:"size(200);column(last_name);null"`
	IsVip     bool        `orm:"default(false)"`
	IsAdmin   bool        `orm:"default(false);column(is_admin)"`
	IsBlocked bool        `orm:"default(false)"`
	JoinedAt  time.Time   `orm:"auto_now_add;type(datetime);column(joined_at)"`
	Balance   float64     `orm:"digits(18);decimals(2)"`
	UpdatedAt time.Time   `orm:"auto_now;type(datetime);column(updated_at)"`
}

func (b *BotUser) TableName() string {
	return "bot_user"
}

type BotChannel struct {
	Id         int64       `orm:"auto;pk"`
	Bot        *CreatedBot `orm:"rel(fk);column(bot_id)"`
	ChannelID  int64       `orm:"column(channel_id)"`
	InviteLink string      `orm:"size(500);column(invite_link)"`
	IsActive   bool        `orm:"default(true);column(is_active)"`
	CreatedAt  time.Time   `orm:"auto_now_add;type(datetime);column(created_at)"`
}

func (b *BotChannel) TableName() string {
	return "bot_channel"
}

type BotJoinRequest struct {
	Id        int64       `orm:"auto;pk"`
	Bot       *CreatedBot `orm:"rel(fk);column(bot_id)"`
	TgId      int64       `orm:"column(tg_id)"`
	ChannelID int64       `orm:"column(channel_id)"`
}

func (b *BotJoinRequest) TableName() string {
	return "bot_join_request"
}

type Kino struct {
	Id         int64       `orm:"auto;pk"`
	Bot        *CreatedBot `orm:"rel(fk);column(bot_id)"`
	Name       string      `orm:"size(300)"`                      // Kino nomi
	Code       string      `orm:"size(100)"`                      // Kino kodi (qidiruv uchun)
	PhotoID    string      `orm:"size(500);column(photo_id)"`     // Kino afishasi/rasmi IDsi
	Year       int         `orm:"default(0);column(year)"`        // Kinosining chiqarilgan yili
	PartsCount int         `orm:"default(0);column(parts_count)"` // Qismlar soni
	IsActive   bool        `orm:"default(true);column(is_active)"`
	CreatedAt  time.Time   `orm:"auto_now_add;type(datetime);column(created_at)"`
	UpdatedAt  time.Time   `orm:"auto_now;type(datetime);column(updated_at)"`
}
type KinoPart struct {
	Id        int64     `orm:"auto;pk"`
	Kino      *Kino     `orm:"rel(fk);column(kino_id)"`
	Kind      string    `orm:"size(20);column(kind)"`
	FileID    string    `orm:"size(500);column(file_id)"`
	MessageID int       `orm:"column(message_id)"`
	PartOrder int       `orm:"default(0);column(part_order)"`
	CreatedAt time.Time `orm:"auto_now_add;type(datetime);column(created_at)"`
}

func (p *KinoPart) TableName() string {
	return "kino_part"
}

type Admin struct {
	Id        int       `orm:"pk;auto"`                 // Primary key
	Username  string    `orm:"size(50);unique"`         // Login
	Password  string    `orm:"size(255)"`               // Hashlangan parol
	Email     string    `orm:"size(100);null"`          // Email (ixtiyoriy)
	FullName  string    `orm:"size(100);null"`          // To'liq ism
	Role      string    `orm:"size(20);default(admin)"` // admin, superadmin va h.k.
	IsActive  bool      `orm:"default(true)"`
	CreatedAt time.Time `orm:"auto_now_add;type(datetime)"`
	UpdatedAt time.Time `orm:"auto_now;type(datetime)"`
}

func init() {
	orm.RegisterModel(
		new(UserBot),
		new(BotType),
		new(CreatedBot),
		new(BotUser),
		new(Anime),
		new(AnimePart),
		new(BotChannel),
		new(BotJoinRequest),
		new(BotInvoice),
		new(Kino),
		new(KinoPart),
		new(Admin),
	)
}
