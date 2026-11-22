package visitsview

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/listview"
)

const (
	yesterday = "2026-11-09T09:00:00Z"
	today     = "2026-11-10T20:00:00Z"
	future    = "2026-12-01T09:00:00Z"
)

func input(status int) statusInput {
	return statusInput{status: status, now: testNow}
}

func TestDerive(t *testing.T) {
	t.Run("booked beats the fse status", func(t *testing.T) {
		in := input(5)
		in.booked = true
		got := derive(in)
		assert.Equal(t, Status{Key: "booked", Label: "Prenotata", Tone: listview.ToneBooked, Kind: KindBooked}, got)
	})

	t.Run("status 3 is to book and bookable", func(t *testing.T) {
		got := derive(input(3))
		assert.Equal(t, "prescribed", got.Key)
		assert.Equal(t, "Da prenotare", got.Label)
		assert.Equal(t, listview.ToneToBook, got.Tone)
		assert.Equal(t, KindOfficial, got.Kind)
		assert.True(t, got.CanBook)
		assert.False(t, got.Manual)
	})

	t.Run("status 5 is pending", func(t *testing.T) {
		got := derive(input(5))
		assert.Equal(t, "Presa in carico", got.Label)
		assert.Equal(t, listview.TonePending, got.Tone)
	})

	t.Run("status 30 is booked tone without a note when not unbooked", func(t *testing.T) {
		got := derive(input(30))
		assert.Equal(t, "Prenotata", got.Label)
		assert.Equal(t, listview.ToneBooked, got.Tone)
		assert.Empty(t, got.Note)
		assert.False(t, got.CanBook)
	})

	t.Run("status 7 is pending", func(t *testing.T) {
		assert.Equal(t, listview.TonePending, derive(input(7)).Tone)
	})

	t.Run("statuses 8 and 9 are done", func(t *testing.T) {
		assert.Equal(t, listview.ToneDone, derive(input(8)).Tone)
		assert.Equal(t, listview.ToneDone, derive(input(9)).Tone)
	})

	t.Run("status 4 is closed", func(t *testing.T) {
		got := derive(input(4))
		assert.Equal(t, "Annullata", got.Label)
		assert.Equal(t, listview.ToneClosed, got.Tone)
	})

	t.Run("unknown status groups under other with a numbered label", func(t *testing.T) {
		got := derive(input(99))
		assert.Equal(t, "other", got.Key)
		assert.Equal(t, "Stato 99", got.Label)
		assert.Equal(t, listview.ToneClosed, got.Tone)
		assert.Equal(t, 99, got.Code)
	})

	t.Run("status 30 unbooked is to verify with the default note", func(t *testing.T) {
		in := input(30)
		in.unbooked = true
		got := derive(in)
		assert.Equal(t, "unverified", got.Key)
		assert.Equal(t, "Da verificare", got.Label)
		assert.Equal(t, listview.TonePending, got.Tone)
		assert.Equal(t, fseBookedNote, got.Note)
		assert.True(t, got.CanBook)
		assert.True(t, got.VerifiedAt.IsZero())
	})
}

func TestDeriveManual(t *testing.T) {
	t.Run("uses the override code and flags it manual", func(t *testing.T) {
		code := 8
		in := input(3)
		in.override = &code
		got := derive(in)
		assert.Equal(t, "done", got.Key)
		assert.Equal(t, "Erogata", got.Label)
		assert.Equal(t, listview.ToneDone, got.Tone)
		assert.True(t, got.Manual)
		assert.Equal(t, KindOfficial, got.Kind)
	})

	t.Run("override to status 3 can be booked", func(t *testing.T) {
		code := 3
		in := input(5)
		in.override = &code
		got := derive(in)
		assert.Equal(t, "Da prenotare", got.Label)
		assert.True(t, got.Manual)
		assert.True(t, got.CanBook)
	})

	t.Run("ignores booking, expiry and verdict", func(t *testing.T) {
		code := 5
		in := input(5)
		in.override = &code
		in.booked = true
		in.expiresOn = at(yesterday)
		in.unbooked = true
		in.verdict = verdict(cup.VerdictHandled, "m")
		got := derive(in)
		assert.Equal(t, "Presa in carico", got.Label)
		assert.True(t, got.Manual)
		assert.Empty(t, got.Note)
	})

	t.Run("is not manual without an override", func(t *testing.T) {
		assert.False(t, derive(input(3)).Manual)
	})
}

func TestDeriveExpiry(t *testing.T) {
	t.Run("expired when expiresOn is yesterday", func(t *testing.T) {
		in := input(3)
		in.expiresOn = at(yesterday)
		got := derive(in)
		assert.Equal(t, KindExpired, got.Kind)
		assert.Equal(t, "expired", got.Key)
		assert.Equal(t, "Scaduta", got.Label)
		assert.Equal(t, listview.ToneClosed, got.Tone)
		assert.Equal(t, at(yesterday), got.ExpiresOn)
		assert.False(t, got.ExpiryEstimated)
	})

	t.Run("flags an estimated expiry", func(t *testing.T) {
		in := input(3)
		in.expiresOn = at(yesterday)
		in.estimated = true
		got := derive(in)
		assert.Equal(t, KindExpired, got.Kind)
		assert.True(t, got.ExpiryEstimated)
	})

	t.Run("expires an unbooked status 30 row", func(t *testing.T) {
		in := input(30)
		in.expiresOn = at(yesterday)
		in.unbooked = true
		got := derive(in)
		assert.Equal(t, KindExpired, got.Kind)
		assert.Equal(t, "Scaduta", got.Label)
	})

	t.Run("flags an estimated expiry on a status 30 row", func(t *testing.T) {
		in := input(30)
		in.expiresOn = at(yesterday)
		in.unbooked = true
		in.estimated = true
		got := derive(in)
		assert.Equal(t, KindExpired, got.Kind)
		assert.True(t, got.ExpiryEstimated)
	})

	t.Run("keeps an unbooked status 30 row to verify while not expired", func(t *testing.T) {
		in := input(30)
		in.expiresOn = at(future)
		in.unbooked = true
		got := derive(in)
		assert.Equal(t, KindOfficial, got.Kind)
		assert.Equal(t, "Da verificare", got.Label)
	})

	t.Run("not expired when expiresOn is today", func(t *testing.T) {
		in := input(3)
		in.expiresOn = at(today)
		assert.Equal(t, KindOfficial, derive(in).Kind)
	})

	t.Run("not expired when expiresOn is in the future", func(t *testing.T) {
		in := input(3)
		in.expiresOn = at(future)
		assert.Equal(t, KindOfficial, derive(in).Kind)
	})

	t.Run("stays booked when an active booking exists", func(t *testing.T) {
		in := input(3)
		in.expiresOn = at(yesterday)
		in.booked = true
		assert.Equal(t, KindBooked, derive(in).Kind)
	})

	t.Run("stays official for a status 8 row", func(t *testing.T) {
		in := input(8)
		in.expiresOn = at(yesterday)
		got := derive(in)
		assert.Equal(t, KindOfficial, got.Kind)
		assert.Equal(t, listview.ToneDone, got.Tone)
	})
}

func TestDeriveVerdicts(t *testing.T) {
	unbooked30 := func() statusInput {
		in := input(30)
		in.unbooked = true
		return in
	}

	t.Run("bookable acts like an open prescription", func(t *testing.T) {
		in := unbooked30()
		in.verdict = verdict(cup.VerdictBookable, "Messaggio esempio")
		got := derive(in)
		assert.Equal(t, KindOfficial, got.Kind)
		assert.Equal(t, "Da prenotare", got.Label)
		assert.Equal(t, listview.ToneToBook, got.Tone)
		assert.True(t, got.CanBook)
		assert.Equal(t, at("2026-11-10T08:00:00Z"), got.VerifiedAt)
	})

	t.Run("handled is Gestita altrove with the message", func(t *testing.T) {
		in := unbooked30()
		in.verdict = verdict(cup.VerdictHandled, "Messaggio esempio")
		got := derive(in)
		assert.Equal(t, "handled", got.Key)
		assert.Equal(t, "Gestita altrove", got.Label)
		assert.Equal(t, listview.ToneClosed, got.Tone)
		assert.Equal(t, "Messaggio esempio", got.Note)
		assert.False(t, got.CanBook)
		assert.Equal(t, at("2026-11-10T08:00:00Z"), got.VerifiedAt)
	})

	t.Run("expired verdict is Scaduta with the message", func(t *testing.T) {
		in := unbooked30()
		in.verdict = verdict(cup.VerdictExpired, "Messaggio esempio")
		got := derive(in)
		assert.Equal(t, KindExpired, got.Kind)
		assert.Equal(t, "Scaduta", got.Label)
		assert.Equal(t, "Messaggio esempio", got.Note)
	})

	t.Run("other keeps Da verificare with the message", func(t *testing.T) {
		in := unbooked30()
		in.verdict = verdict(cup.VerdictOther, "Messaggio esempio")
		got := derive(in)
		assert.Equal(t, "Da verificare", got.Label)
		assert.Equal(t, "Messaggio esempio", got.Note)
		assert.Equal(t, at("2026-11-10T08:00:00Z"), got.VerifiedAt)
	})

	t.Run("ignores a verdict for a row that is not unbooked", func(t *testing.T) {
		in := input(30)
		in.verdict = verdict(cup.VerdictHandled, "m")
		assert.Equal(t, "Prenotata", derive(in).Label)
	})

	t.Run("applies a verdict to a status 3 unbooked row", func(t *testing.T) {
		in := input(3)
		in.unbooked = true
		in.verdict = verdict(cup.VerdictHandled, "m")
		assert.Equal(t, "Gestita altrove", derive(in).Label)
	})
}

func TestDetails(t *testing.T) {
	t.Run("lists note, expiry and verification for a handled row", func(t *testing.T) {
		in := input(3)
		in.unbooked = true
		in.expiresOn = at(future)
		in.verdict = verdict(cup.VerdictHandled, "Gestita altrove da altri")
		got := derive(in)
		assert.Equal(t, []string{
			"Gestita altrove da altri",
			"scade il mar 1 dic 2026",
			"Verificato con il CUP il 10/11 09:00",
		}, got.Details)
	})

	t.Run("says scade il for an open prescription with an expiry", func(t *testing.T) {
		in := input(3)
		in.expiresOn = at("2026-12-04T09:00:00Z")
		in.estimated = true
		assert.Equal(t, []string{"scade il ven 4 dic 2026 (stimata)"}, derive(in).Details)
	})

	t.Run("says scaduta il for an expired row", func(t *testing.T) {
		in := input(3)
		in.expiresOn = at("2026-11-06T09:00:00Z")
		assert.Equal(t, []string{"scaduta il ven 6 nov 2026"}, derive(in).Details)
	})

	t.Run("is empty for a booked row", func(t *testing.T) {
		in := input(3)
		in.booked = true
		in.expiresOn = at(future)
		assert.Empty(t, derive(in).Details)
	})

	t.Run("is empty and non-nil for a plain row", func(t *testing.T) {
		assert.Equal(t, []string{}, derive(input(5)).Details)
	})
}

func TestStatusRank(t *testing.T) {
	t.Run("follows the display order", func(t *testing.T) {
		assert.Less(t, statusRank("booked"), statusRank("prescribed"))
		assert.Less(t, statusRank("prescribed"), statusRank("unverified"))
		assert.Less(t, statusRank("expired"), statusRank("other"))
	})

	t.Run("unknown key ranks last", func(t *testing.T) {
		assert.Greater(t, statusRank("zzz"), statusRank("other"))
	})
}

func TestDefaultExcluded(t *testing.T) {
	t.Run("lists the closed statuses and expired", func(t *testing.T) {
		assert.Equal(t, map[string]bool{"done": true, "done_after_cancel": true, "cancelled": true, "expired": true}, defaultExcluded())
	})
}
