package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicHolidays(t *testing.T) {
	nl, ok := PublicHolidays("nl", 2027)
	require.True(t, ok)
	dates := map[string]string{}
	for _, h := range nl {
		dates[h.Date.Format("2006-01-02")] = h.Name
	}
	assert.Contains(t, dates, "2027-01-01", "New Year")
	assert.Contains(t, dates, "2027-03-28", "Easter Sunday 2027")
	assert.Contains(t, dates, "2027-04-27", "King's Day")
	assert.Contains(t, dates, "2027-12-25", "Christmas")

	uk, ok := PublicHolidays("en", 2027)
	require.True(t, ok)
	assert.NotEmpty(t, uk)

	_, ok = PublicHolidays("xx", 2027)
	assert.False(t, ok)
}
