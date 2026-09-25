package services

import (
	"errors"
	"strconv"
	"strings"

	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/models"
)

// ErrCardNotFound is returned by FindCardByKey when no live card or alias matches.
var ErrCardNotFound = errors.New("card not found")

// ParseCardKey splits a card key such as "PRJ-12" into its upper-cased prefix
// and number. ok is false when ref is not of that form.
func ParseCardKey(ref string) (prefix string, number int, ok bool) {
	i := strings.LastIndex(ref, "-")
	if i <= 0 {
		return "", 0, false
	}
	n, err := strconv.Atoi(ref[i+1:])
	if err != nil || n < 1 {
		return "", 0, false
	}
	return strings.ToUpper(ref[:i]), n, true
}

// FindCardByKey resolves a card key to its (non-deleted) card. The live key
// wins; otherwise a CardKeyAlias left behind by a move between projects is
// followed, so old references keep working after the card was renumbered.
func FindCardByKey(prefix string, number int) (*models.Card, error) {
	prefix = strings.ToUpper(prefix)
	var card models.Card
	err := database.DB.
		Joins("JOIN projects ON projects.id = cards.project_id").
		Where("projects.key_prefix = ? AND cards.card_number = ?", prefix, number).
		First(&card).Error
	if err == nil {
		return &card, nil
	}
	return FindCardByAlias(prefix, number)
}

// FindCardByAlias resolves only a former key (see CardKeyAlias), ignoring
// live keys.
func FindCardByAlias(prefix string, number int) (*models.Card, error) {
	var alias models.CardKeyAlias
	if database.DB.Where("key_prefix = ? AND card_number = ?", strings.ToUpper(prefix), number).First(&alias).Error != nil {
		return nil, ErrCardNotFound
	}
	var card models.Card
	if database.DB.First(&card, alias.CardID).Error != nil {
		return nil, ErrCardNotFound
	}
	return &card, nil
}
