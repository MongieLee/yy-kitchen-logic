package models

import (
	"time"

	"github.com/uptrace/bun"
)

// AppVersion represents a published Android APK release.
// swagger:model
type AppVersion struct {
	bun.BaseModel `bun:"table:app_versions"`

	ID              int64      `bun:",pk,autoincrement" json:"id"`
	VersionCode     int64      `bun:",notnull,unique" json:"version_code" example:"12" doc:"版本号(数字,升序递增)"`
	VersionName     string     `bun:",notnull" json:"version_name" example:"1.2.0" doc:"版本名(展示用)"`
	DownloadURL     string     `bun:",notnull" json:"download_url" example:"https://cdn.example.com/app/1.2.0.apk"`
	ReleaseNotes    string     `bun:"" json:"release_notes" example:"修复若干问题"`
	MinSupportedCode int64     `bun:",notnull,default:0" json:"min_supported_code" example:"10" doc:"低于该版本号需强制更新"`
	CreatedAt       time.Time  `bun:",nullzero" json:"created_at"`
	UpdatedAt       time.Time  `bun:",nullzero" json:"updated_at"`
	DeletedAt       *time.Time `bun:",soft_delete,nullzero" json:"-"`
}
