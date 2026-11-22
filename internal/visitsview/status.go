package visitsview

import (
	"fmt"
	"slices"
	"time"

	"fuck-toscana-salute/internal/agenda"
	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/itfmt"
	"fuck-toscana-salute/internal/listview"
)

const (
	statusToBook     = 3
	statusFSEBooked  = 30
	cancelledBooking = "DISDETTO"

	keyBooked     = "booked"
	keyUnverified = "unverified"
	keyHandled    = "handled"
	keyExpired    = "expired"

	labelBooked     = "Prenotata"
	labelUnverified = "Da verificare"
	labelHandled    = "Gestita altrove"
	labelExpired    = "Scaduta"
	labelOther      = "Altro stato"

	fseBookedNote = "FSE la segna come prenotata, ma nel CUP non c'è un tuo appuntamento: può essere già gestita altrove (prenotazione o accettazione) o ancora prenotabile"
)

type statusDef struct {
	key      string
	label    string
	tone     string
	excluded bool
}

var statusDefs = []statusDef{
	{key: keyBooked, label: labelBooked, tone: listview.ToneBooked},
	{key: cup.StatusPrescribed.Key(), label: cup.StatusPrescribed.Label(), tone: listview.ToneToBook},
	{key: keyUnverified, label: labelUnverified, tone: listview.TonePending},
	{key: keyHandled, label: labelHandled, tone: listview.ToneClosed},
	{key: cup.StatusTakenInCharge.Key(), label: cup.StatusTakenInCharge.Label(), tone: listview.TonePending},
	{key: cup.StatusPartiallyDone.Key(), label: cup.StatusPartiallyDone.Label(), tone: listview.TonePending},
	{key: cup.StatusSuspended.Key(), label: cup.StatusSuspended.Label(), tone: listview.ToneClosed},
	{key: cup.StatusDone.Key(), label: cup.StatusDone.Label(), tone: listview.ToneDone, excluded: true},
	{key: cup.StatusDoneAfterCancel.Key(), label: cup.StatusDoneAfterCancel.Label(), tone: listview.ToneDone, excluded: true},
	{key: cup.StatusCancelled.Key(), label: cup.StatusCancelled.Label(), tone: listview.ToneClosed, excluded: true},
	{key: keyExpired, label: labelExpired, tone: listview.ToneClosed, excluded: true},
	{key: cup.KeyOther, label: labelOther, tone: listview.ToneClosed},
}

var officialTones = map[int]string{
	statusToBook:                   listview.ToneToBook,
	int(cup.StatusCancelled):       listview.ToneClosed,
	int(cup.StatusTakenInCharge):   listview.TonePending,
	int(cup.StatusPartiallyDone):   listview.TonePending,
	int(cup.StatusDone):            listview.ToneDone,
	int(cup.StatusDoneAfterCancel): listview.ToneDone,
	statusFSEBooked:                listview.ToneBooked,
}

func statusRank(key string) int {
	at := slices.IndexFunc(statusDefs, func(d statusDef) bool { return d.key == key })
	if at < 0 {
		return len(statusDefs)
	}
	return at
}

func defaultExcluded() map[string]bool {
	excluded := map[string]bool{}
	for _, d := range statusDefs {
		if d.excluded {
			excluded[d.key] = true
		}
	}
	return excluded
}

func officialTone(code int) string {
	if tone, ok := officialTones[code]; ok {
		return tone
	}
	return listview.ToneClosed
}

func official(code int) Status {
	s := cup.Status(code)
	return Status{Key: s.Key(), Label: s.Label(), Tone: officialTone(code), Kind: KindOfficial, Code: code, CanBook: code == statusToBook}
}

type statusInput struct {
	status    int
	expiresOn time.Time
	estimated bool
	unbooked  bool
	verdict   *agenda.VerdictEntry
	booked    bool
	override  *int
	now       time.Time
}

func derive(in statusInput) Status {
	s := deriveBase(in)
	if s.Kind == KindBooked {
		return s
	}
	s.ExpiresOn = in.expiresOn
	s.ExpiryEstimated = in.estimated
	s.Details = details(s)
	return s
}

func deriveBase(in statusInput) Status {
	if in.override != nil {
		s := official(*in.override)
		s.Manual = true
		return s
	}
	if in.booked {
		return Status{Key: keyBooked, Label: labelBooked, Tone: listview.ToneBooked, Kind: KindBooked}
	}
	open := in.status == statusToBook || in.status == statusFSEBooked
	if open && !in.expiresOn.IsZero() && itfmt.Day(in.expiresOn) < itfmt.Day(in.now) {
		return Status{Key: keyExpired, Label: labelExpired, Tone: listview.ToneClosed, Kind: KindExpired}
	}
	if open && in.unbooked && in.verdict != nil {
		if s, ok := fromVerdict(in); ok {
			return s
		}
	}
	if in.status == statusFSEBooked && in.unbooked {
		return unverified(in)
	}
	return official(in.status)
}

func fromVerdict(in statusInput) (Status, bool) {
	switch in.verdict.Verdict.Reason {
	case cup.VerdictBookable:
		s := official(statusToBook)
		s.VerifiedAt = in.verdict.VerifiedAt
		return s, true
	case cup.VerdictHandled:
		return Status{
			Key:        keyHandled,
			Label:      labelHandled,
			Tone:       listview.ToneClosed,
			Kind:       KindOfficial,
			Code:       in.status,
			Note:       in.verdict.Verdict.Message,
			VerifiedAt: in.verdict.VerifiedAt,
		}, true
	case cup.VerdictExpired:
		return Status{
			Key:        keyExpired,
			Label:      labelExpired,
			Tone:       listview.ToneClosed,
			Kind:       KindExpired,
			Note:       in.verdict.Verdict.Message,
			VerifiedAt: in.verdict.VerifiedAt,
		}, true
	}
	return Status{}, false
}

func unverified(in statusInput) Status {
	s := Status{
		Key:     keyUnverified,
		Label:   labelUnverified,
		Tone:    listview.TonePending,
		Kind:    KindOfficial,
		Code:    in.status,
		Note:    fseBookedNote,
		CanBook: true,
	}
	if in.verdict != nil {
		s.Note = in.verdict.Verdict.Message
		s.VerifiedAt = in.verdict.VerifiedAt
	}
	return s
}

func details(s Status) []string {
	lines := []string{}
	if s.Note != "" {
		lines = append(lines, s.Note)
	}
	suffix := ""
	if s.ExpiryEstimated {
		suffix = " (stimata)"
	}
	if !s.ExpiresOn.IsZero() && s.Kind == KindExpired {
		lines = append(lines, fmt.Sprintf("scaduta il %s%s", itfmt.Date(s.ExpiresOn), suffix))
	}
	if !s.ExpiresOn.IsZero() && s.Kind == KindOfficial && s.Code == statusToBook {
		lines = append(lines, fmt.Sprintf("scade il %s%s", itfmt.Date(s.ExpiresOn), suffix))
	}
	if !s.VerifiedAt.IsZero() {
		lines = append(lines, "Verificato con il CUP il "+itfmt.ShortDateTime(s.VerifiedAt))
	}
	return lines
}
