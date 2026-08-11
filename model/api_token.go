package model

import "gorm.io/gorm"

type ApiToken struct {
	Common
	UserID          uint64 `json:"user_id"`
	Token           string `gorm:"-" json:"-"`
	TokenCiphertext []byte `gorm:"column:token_ciphertext;type:BLOB;not null" json:"-"`
	TokenHash       []byte `gorm:"column:token_hash;type:BLOB;size:32;not null;uniqueIndex" json:"-"`
	Note            string `json:"note"`
}

func (t *ApiToken) BeforeSave(_ *gorm.DB) error {
	value, err := encryptSecret(t.Token)
	if err != nil {
		return err
	}
	t.TokenCiphertext = value
	return nil
}

func (t *ApiToken) AfterFind(_ *gorm.DB) error {
	value, err := decryptSecret(t.TokenCiphertext)
	if err != nil {
		return err
	}
	t.Token = value
	return nil
}
