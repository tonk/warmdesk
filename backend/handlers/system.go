package handlers

import (
	"fmt"
	"net/http"
	"net/smtp"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tonk/coworker/config"
	"github.com/tonk/coworker/database"
	"github.com/tonk/coworker/models"
	"gorm.io/gorm/clause"
)

const (
	settingRegistrationEnabled    = "registration_enabled"
	settingDefaultDateTimeFormat  = "default_date_time_format"
	settingDefaultTimezone        = "default_timezone"
	settingDefaultTheme           = "default_theme"
	settingDefaultFont            = "default_font"
	settingDefaultFontSize        = "default_font_size"
	settingDefaultLocale          = "default_locale"
	settingSMTPHost               = "smtp_host"
	settingSMTPPort               = "smtp_port"
	settingSMTPFrom               = "smtp_from"
	settingSMTPUsername           = "smtp_username"
	settingSMTPPassword           = "smtp_password"
	settingSessionTimeoutMinutes  = "session_timeout_minutes"
	settingCompanyName            = "company_name"
	settingCompanyLogo            = "company_logo"
	settingDefaultColumns         = "default_columns"
)

var systemSettingDefaults = map[string]string{
	settingRegistrationEnabled:   "true",
	settingDefaultDateTimeFormat: "YYYY-MM-DD HH:mm",
	settingDefaultTimezone:       "UTC",
	settingDefaultTheme:          "system",
	settingDefaultFont:           "system",
	settingDefaultFontSize:       "14",
	settingDefaultLocale:         "en",
	settingSMTPHost:               "",
	settingSMTPPort:               "587",
	settingSMTPFrom:               "",
	settingSMTPUsername:           "",
	settingSMTPPassword:           "",
	settingSessionTimeoutMinutes:  "60",
	settingCompanyName:            "",
	settingCompanyLogo:            "",
	settingDefaultColumns:         "Backlog",
}

// InitSystemDefaults seeds the in-memory defaults from the config file so that
// settings not yet stored in the database reflect the operator's preferences.
func InitSystemDefaults(cfg *config.Config) {
	if cfg.DefaultLocale != "" {
		systemSettingDefaults[settingDefaultLocale] = cfg.DefaultLocale
	}
	if cfg.SMTP.Host != "" {
		systemSettingDefaults[settingSMTPHost] = cfg.SMTP.Host
	}
	if cfg.SMTP.Port != 0 {
		systemSettingDefaults[settingSMTPPort] = fmt.Sprintf("%d", cfg.SMTP.Port)
	}
	if cfg.SMTP.From != "" {
		systemSettingDefaults[settingSMTPFrom] = cfg.SMTP.From
	}
	if cfg.SMTP.Username != "" {
		systemSettingDefaults[settingSMTPUsername] = cfg.SMTP.Username
	}
	if cfg.SMTP.Password != "" {
		systemSettingDefaults[settingSMTPPassword] = cfg.SMTP.Password
	}
}

// GetSMTPSettings returns the current SMTP configuration from the database.
// Used by the email service so changes take effect without a restart.
func GetSMTPSettings() config.SMTPConfig {
	all := loadAllSettings()
	port, _ := strconv.Atoi(all[settingSMTPPort])
	if port == 0 {
		port = 587
	}
	return config.SMTPConfig{
		Host:     all[settingSMTPHost],
		Port:     port,
		From:     all[settingSMTPFrom],
		Username: all[settingSMTPUsername],
		Password: all[settingSMTPPassword],
	}
}

// GetSystemSettings returns public system settings (registration + global UI defaults).
func GetSystemSettings(c *gin.Context) {
	all := loadAllSettings()
	timeoutMinutes, _ := strconv.Atoi(all[settingSessionTimeoutMinutes])
	c.JSON(http.StatusOK, gin.H{
		"registration_enabled":        all[settingRegistrationEnabled] != "false",
		"default_date_time_format":    all[settingDefaultDateTimeFormat],
		"default_timezone":            all[settingDefaultTimezone],
		"default_theme":               all[settingDefaultTheme],
		"default_font":                all[settingDefaultFont],
		"default_font_size":           all[settingDefaultFontSize],
		"default_locale":              all[settingDefaultLocale],
		"session_timeout_minutes":     timeoutMinutes,
		"company_name":                all[settingCompanyName],
		"company_logo":                all[settingCompanyLogo],
	})
}

// AdminGetSystemSettings returns all system settings for admins.
// The SMTP password is never sent back — only smtp_password_set (bool) is included.
func AdminGetSystemSettings(c *gin.Context) {
	all := loadAllSettings()
	// Mask the password: send only whether one is configured
	passwordSet := all[settingSMTPPassword] != ""
	delete(all, settingSMTPPassword)
	all["smtp_password_set"] = fmt.Sprintf("%t", passwordSet)
	c.JSON(http.StatusOK, all)
}

// AdminUpdateSystemSettings updates system settings.
func AdminUpdateSystemSettings(c *gin.Context) {
	var req struct {
		RegistrationEnabled    *bool   `json:"registration_enabled"`
		DefaultDateTimeFormat  string  `json:"default_date_time_format"`
		DefaultTimezone        string  `json:"default_timezone"`
		DefaultTheme           string  `json:"default_theme"`
		DefaultFont            string  `json:"default_font"`
		DefaultFontSize        string  `json:"default_font_size"`
		DefaultLocale          string  `json:"default_locale"`
		SMTPHost               *string `json:"smtp_host"`
		SMTPPort               string  `json:"smtp_port"`
		SMTPFrom               *string `json:"smtp_from"`
		SMTPUsername           *string `json:"smtp_username"` // pointer so empty string clears it
		SMTPPassword           *string `json:"smtp_password"` // pointer so empty string clears it
		SessionTimeoutMinutes  *int    `json:"session_timeout_minutes"`
		CompanyName            *string `json:"company_name"`
		CompanyLogo            *string `json:"company_logo"`
		DefaultColumns         *string `json:"default_columns"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.RegistrationEnabled != nil {
		val := "true"
		if !*req.RegistrationEnabled {
			val = "false"
		}
		saveSetting(settingRegistrationEnabled, val)
	}
	if req.DefaultDateTimeFormat != "" {
		saveSetting(settingDefaultDateTimeFormat, req.DefaultDateTimeFormat)
	}
	if req.DefaultTimezone != "" {
		saveSetting(settingDefaultTimezone, req.DefaultTimezone)
	}
	if req.DefaultTheme == "light" || req.DefaultTheme == "dark" || req.DefaultTheme == "system" {
		saveSetting(settingDefaultTheme, req.DefaultTheme)
	}
	if req.DefaultFont != "" {
		saveSetting(settingDefaultFont, req.DefaultFont)
	}
	if req.DefaultFontSize != "" {
		saveSetting(settingDefaultFontSize, req.DefaultFontSize)
	}
	validLocales := map[string]bool{"en": true, "nl": true, "de": true, "fr": true, "es": true}
	if validLocales[req.DefaultLocale] {
		saveSetting(settingDefaultLocale, req.DefaultLocale)
	}
	// SMTP — only save fields that were explicitly included in the request
	// (pointer fields: nil means "not sent", so don't overwrite; empty string clears)
	if req.SMTPHost != nil {
		saveSetting(settingSMTPHost, *req.SMTPHost)
	}
	if req.SMTPPort != "" {
		saveSetting(settingSMTPPort, req.SMTPPort)
	}
	if req.SMTPFrom != nil {
		saveSetting(settingSMTPFrom, *req.SMTPFrom)
	}
	if req.SMTPUsername != nil {
		saveSetting(settingSMTPUsername, *req.SMTPUsername)
	}
	if req.SMTPPassword != nil {
		saveSetting(settingSMTPPassword, *req.SMTPPassword)
	}
	if req.SessionTimeoutMinutes != nil {
		timeout := *req.SessionTimeoutMinutes
		if timeout < 0 {
			timeout = 0
		}
		saveSetting(settingSessionTimeoutMinutes, fmt.Sprintf("%d", timeout))
	}
	if req.CompanyName != nil {
		saveSetting(settingCompanyName, *req.CompanyName)
	}
	if req.CompanyLogo != nil {
		saveSetting(settingCompanyLogo, *req.CompanyLogo)
	}
	if req.DefaultColumns != nil {
		saveSetting(settingDefaultColumns, *req.DefaultColumns)
	}

	AdminGetSystemSettings(c)
}

// AdminSendTestEmail sends a test email to verify the current SMTP configuration.
func AdminSendTestEmail(c *gin.Context) {
	var req struct {
		To string `json:"to" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email address is required"})
		return
	}

	cfg := GetSMTPSettings()
	if cfg.Host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SMTP host is not configured"})
		return
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	from := cfg.From
	if from == "" {
		from = "coworker@localhost"
	}
	body := "This is a test email from Coworker. Your SMTP configuration is working correctly."
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: Coworker SMTP test\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, req.To, body)

	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}
	if err := smtp.SendMail(addr, auth, from, []string{req.To}, []byte(msg)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Test email sent to " + req.To})
}

// IsRegistrationEnabled is a helper used by the Register handler.
func IsRegistrationEnabled() bool {
	var setting models.SystemSetting
	result := database.DB.First(&setting, "key = ?", settingRegistrationEnabled)
	if result.Error != nil {
		return true // default: enabled
	}
	return setting.Value != "false"
}

// GetGlobalDefaults returns the global default settings for new users.
func GetGlobalDefaults() map[string]string {
	all := loadAllSettings()
	return map[string]string{
		"date_time_format": all[settingDefaultDateTimeFormat],
		"timezone":         all[settingDefaultTimezone],
		"theme":            all[settingDefaultTheme],
		"font":             all[settingDefaultFont],
		"font_size":        all[settingDefaultFontSize],
		"locale":           all[settingDefaultLocale],
	}
}

// saveSetting upserts a system setting by key (INSERT … ON CONFLICT UPDATE).
// GORM's plain Save() with a non-zero string primary key issues only an UPDATE,
// which silently does nothing when the row doesn't exist yet.
func saveSetting(key, value string) {
	database.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&models.SystemSetting{Key: key, Value: value})
}

// loadAllSettings reads all system settings from DB and fills in defaults for missing keys.
func loadAllSettings() map[string]string {
	result := map[string]string{}
	for k, v := range systemSettingDefaults {
		result[k] = v
	}
	var settings []models.SystemSetting
	database.DB.Find(&settings)
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result
}
