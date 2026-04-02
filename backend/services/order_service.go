package services

import (
	"math"

	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/models"
)

const minGap = 1e-9

// MidPosition returns the midpoint between two positions.
// If a == 0 and b == 0 (both zero), returns 1000.
func MidPosition(a, b float64) float64 {
	if b == 0 && a == 0 {
		return 1000
	}
	mid := (a + b) / 2
	if math.Abs(b-a) < minGap {
		return -1 // signal rebalance needed
	}
	return mid
}

// PositionAfter returns a position just after `after`.
func PositionAfter(after float64) float64 {
	if after == 0 {
		return 1000
	}
	return after + 1000
}

// RebalanceColumns renumbers all columns in a project with positions 1000, 2000, 3000...
func RebalanceColumns(projectID uint) error {
	var columns []models.Column
	if err := database.DB.Where("project_id = ?", projectID).Order("position asc").Find(&columns).Error; err != nil {
		return err
	}
	for i, col := range columns {
		newPos := float64((i + 1) * 1000)
		if err := database.DB.Model(&col).Update("position", newPos).Error; err != nil {
			return err
		}
	}
	return nil
}

// RebalanceCards renumbers all cards in a column with positions 1000, 2000, 3000...
func RebalanceCards(columnID uint) error {
	var cards []models.Card
	if err := database.DB.Where("column_id = ?", columnID).Order("position asc").Find(&cards).Error; err != nil {
		return err
	}
	for i, card := range cards {
		newPos := float64((i + 1) * 1000)
		if err := database.DB.Model(&card).Update("position", newPos).Error; err != nil {
			return err
		}
	}
	return nil
}
