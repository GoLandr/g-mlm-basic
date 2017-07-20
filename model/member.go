package model

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"time"

	gorm "gopkg.in/jinzhu/gorm.v1"

	"github.com/twinj/uuid"
)

//Member 数据库对象
type Member struct {
	ID         string         `gorm:"column:id"`
	CardNo     sql.NullString `gorm:"column:cardno"`
	Phone      sql.NullString `gorm:"column:phone"`
	Level      sql.NullString `gorm:"column:level"`
	CreateTime time.Time      `gorm:"column:createtime"`
	Reference  sql.NullString `gorm:"column:reference_id"`
	Name       sql.NullString `gorm:"column:name"`
}

const (
	regular = "^(13[0-9]|14[57]|15[0-35-9]|18[07-9])\\d{8}$"
)

//ValidatePhone 校验手机号格式
func ValidatePhone(mobileNum string) bool {
	reg := regexp.MustCompile(regular)
	return reg.MatchString(mobileNum)
}

//NewMember 空Member
func NewMember() *Member {
	return &Member{}
}

const (
	//ResOK 返回码
	ResOK = "200"
	//ResInvalid 无效参数
	ResInvalid = "412"
	//ResPhoneInvalid 无效手机号
	ResPhoneInvalid = "4121"
	//ResNotFound 没有对应记录
	ResNotFound = "404"
	//ResFound 成功找到
	ResFound = "200"
	//ResWrongSQL 查询语句错误
	ResWrongSQL = "500"
	//ResFail 异常
	ResFail = "500"
	//ResFailCreateMember 创建用户异常
	ResFailCreateMember = "501"
)

//FindByPhoneOrCardno 按手机号查找, 找不到时,按卡号查找
// error code:
//	ResInvalid  phone,cardno 不能都为空
//	ResPhoneInvalid phone 无效
//	ResNotFound  找不到记录
//	ResFound  成功找到
func (m *Member) FindByPhoneOrCardno(db *gorm.DB, phone string, cardno string) (string, error) {
	if len(phone) == 0 && len(cardno) == 0 {
		return ResInvalid, errors.New("请输入手机号或卡号")
	}
	if len(phone) != 0 {
		code, err := m.FindByPhone(db, phone)
		if err != nil {
			return code, err
		}
		return ResFound, nil
	}
	return m.FindByCardno(db, cardno)
}

//FindByID 按id查找
func (m *Member) FindByID(db *gorm.DB, id string) error {
	db1 := db.Where("id=?", id).Find(&m)
	return db1.Error
}

//FindByPhone 按电话查找
func (m *Member) FindByPhone(db *gorm.DB, phone string) (string, error) {
	if !ValidatePhone(phone) {
		return ResPhoneInvalid, errors.New("无效电话号码")
	}
	db1 := db.First(&m, "phone=?", phone)
	//fmt.Println("FindByPhone",phone, db1.Error)
	if db1.RecordNotFound() {
		return ResNotFound, sql.ErrNoRows
	}
	if db1.Error != nil {
		return ResWrongSQL, db1.Error
	}
	return ResFound, nil
}

//FindByCardno 按卡号查找
func (m *Member) FindByCardno(db *gorm.DB, cardno string) (string, error) {
	db1 := db.First(&m, "cardno=?", cardno)
	if db1.RecordNotFound() {
		return ResNotFound, sql.ErrNoRows
	}
	if db1.Error != nil {
		return ResWrongSQL, db1.Error
	}
	return ResFound, nil
}

func (m *Member) String() string {
	p, _ := m.Phone.Value()
	c, _ := m.CardNo.Value()
	r, _ := m.Reference.Value()
	return fmt.Sprintf("id=%s,p=%s,c=%s,ref=%s,time=%s", m.ID, p, c, r, m.CreateTime)
}

//FindByInfo reference 满足 phone No.按phone算, 否则按卡号查询
func (m *Member) FindByInfo(db *gorm.DB, reference string) (string, error) {
	if len(reference) == 0 {
		return ResInvalid, errors.New("无引荐人卡号,或手机号")
	}
	if ValidatePhone(reference) {
		code, err := m.FindByPhone(db, reference)
		fmt.Println("ref:", reference, err, code)
		if err != nil {
			return code, err
		}
		return ResFound, nil
	}
	return m.FindByCardno(db, reference)

}

//AddNewMember 查找推荐用户,添加新用户
func AddNewMember(db *gorm.DB, phone string, cardno string, reference string, level string, name string) *Member {
	ref := NewMember()
	var referenceID string

	if len(reference) > 0 {
		_, err := ref.FindByInfo(db, reference)
		if err == nil {
			referenceID = ref.ID
		}
	}
	if err := ref.CreateMember(db, phone, cardno, referenceID, level, name); err != nil {
		log.Println(err, phone, cardno)
		return nil
	}
	return ref
}

//CreateMember 简单创建用户
func (m *Member) CreateMember(db *gorm.DB, phone string, cardno string, reference string, level string, name string) error {
	m.FillNewMember(phone, cardno, reference, level, name)
	db.Create(m)
	if db.NewRecord(m) {
		return errors.New("用户创建失败")
	}

	go CreateLevels(db, m) //异步创建族谱, 快速返回用户创建请求
	return nil
}

//FillNewMember 填充新member对象
func (m *Member) FillNewMember(phone string, cardno string, reference string, level string, name string) (*Member, error) {
	m.ID = uuid.NewV4().String()
	m.Phone.Scan(phone)
	if len(cardno) != 0 {
		no, err := strconv.Atoi(cardno)
		if err == nil {
			UpdateNewCard(no)
		}
	} else {
		cardno = GetNewCard()
	}
	m.CardNo.Scan(cardno)

	if len(reference) > 0 {
		m.Reference.Scan(reference)
	}
	if len(level) != 0 {
		m.Level.String, m.Level.Valid = level, true
	}
	if len(name) > 0 {
		m.Name.Scan(name)
	}
	m.CreateTime = time.Now()
	return m, nil
}
