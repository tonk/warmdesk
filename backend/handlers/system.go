package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tonk/coworker/database"
	"github.com/tonk/coworker/models"
)

const (
	settingRegistrationEnabled    = "registration_enabled"
	settingDefaultDateTimeFormat  = "default_date_time_format"
	settingDefaultTimezone        = "default_timezone"
	settingDefaultTheme           = "default_theme"
	settingDefaultFont            = "default_font"
	settingDefaultFontSize        = "default_font_size"
)

var systemSettingDefaults = map[string]string{
	settingRegistrationEnabled:   "true",
	settingDefaultDateTimeFormat: "YYYY-MM-DD HH:mm",
	settingDefaultTimezone:       "UTC",
	settingDefaultTheme:          "system",
	settingDefaultFont:           "system",
	settingDefaultFontSize:       "14",
}

// GetSystemSettings returns public system settings (registration + global UI defaults).
func GetSystemSettings(c *gin.Context) {
	all := loadAllSettings()
	c.JSON(http.StatusOK, gin.H{
		"registration_enabled":        all[settingRegistrationEnabled] != "false",
		"default_date_time_format":    all[settingDefaultDateTimeFormat],
		"default_timezone":            all[settingDefaultTimezone],
		"default_theme":               all[settingDefaultTheme],
		"default_font":                all[settingDefaultFont],
		"default_font_size":           all[settingDefaultFontSize],
	})
}

// AdminGetSystemSettings returns all system settings for admins.
func AdminGetSystemSettings(c *gin.Context) {
	all := loadAllSettings()
	c.JSON(http.StatusOK, all)
}

// AdminUpdateSystemSettings updates system settings.
func AdminUpdateSystemSettings(c *gin.Context) {
	var req struct {
		RegistrationEnabled   *bool  `json:"registration_enabled"`
		DefaultDateTimeFormat string `json:"default_date_time_format"`
		DefaultTimezone       string `json:"default_timezone"`
		DefaultTheme          string `json:"default_theme"`
		DefaultFont           string `json:"default_font"`
		DefaultFontSize       string `json:"default_font_size"`
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
		database.DB.Save(&models.SystemSetting{Key: settingRegistrationEnabled, Value: val})
	}
	if req.DefaultDateTimeFormat != "" {
		database.DB.Save(&models.SystemSetting{Key: settingDefaultDateTimeFormat, Value: req.DefaultDateTimeFormat})
	}
	if req.DefaultTimezone != "" {
		database.DB.Save(&models.SystemSetting{Key: settingDefaultTimezone, Value: req.DefaultTimezone})
	}
	if req.DefaultTheme == "light" || req.DefaultTheme == "dark" || req.DefaultTheme == "system" {
		database.DB.Save(&models.SystemSetting{Key: settingDefaultTheme, Value: req.DefaultTheme})
	}
	if req.DefaultFont != "" {
		database.DB.Save(&models.SystemSetting{Key: settingDefaultFont, Value: req.DefaultFont})
	}
	if req.DefaultFontSize != "" {
		database.DB.Save(&models.SystemSetting{Key: settingDefaultFontSize, Value: req.DefaultFontSize})
	}

	AdminGetSystemSettings(c)
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
	}
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
