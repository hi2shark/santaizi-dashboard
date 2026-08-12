package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	ApiTokenPermissionRead   = "read"
	ApiTokenPermissionWrite  = "write"
	CtxKeyAPITokenPermission = "apiTokenPermission"
	CtxKeyIsAPI              = "isAPI"
)

type ApiToken struct {
	Common
	UserID          uint64     `json:"user_id"`
	Token           string     `gorm:"-" json:"-"`
	TokenCiphertext []byte     `gorm:"column:token_ciphertext;type:BLOB;not null" json:"-"`
	TokenHash       []byte     `gorm:"column:token_hash;type:BLOB;size:32;not null;uniqueIndex" json:"-"`
	Note            string     `json:"note"`
	Permission      string     `json:"permission" gorm:"default:write;not null"`
	ExpiresAt       *time.Time `json:"expires_at"`
	Enabled         bool       `json:"enabled" gorm:"default:true;not null"`
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

func (t *ApiToken) NormalizedPermission() string {
	if t.Permission == ApiTokenPermissionRead {
		return ApiTokenPermissionRead
	}
	return ApiTokenPermissionWrite
}

func (t *ApiToken) IsExpired() bool {
	return t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt)
}

func (t *ApiToken) IsActive() bool {
	return t.Enabled && !t.IsExpired()
}
