package drugsview

import (
	"strconv"

	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/listview"
)

type statusDef struct {
	key   string
	label string
	tone  string
}

const otherLabel = "Altro stato"

var statusDefs = []statusDef{
	{"prescribed", "Da ritirare", listview.ToneToBook},
	{"taken_in_charge", "Presa in carico", listview.ToneClosed},
	{"suspended", "Sospesa", listview.ToneClosed},
	{"partially_done", "Erogata in parte", listview.TonePending},
	{"done", "Erogata", listview.ToneDone},
	{"done_after_cancel", "Erogata dopo annullamento", listview.ToneDone},
	{"cancelled", "Annullata", listview.ToneClosed},
	{"booked", "Prenotata", listview.ToneClosed},
	{cup.KeyOther, otherLabel, listview.ToneClosed},
}

var defaultExcluded = []string{"done", "done_after_cancel", "cancelled"}

func statusOf(code int) Status {
	key := cup.Status(code).Key()
	if key == cup.KeyOther {
		return Status{Key: key, Label: "Stato " + strconv.Itoa(code), Tone: listview.ToneClosed}
	}
	for _, def := range statusDefs {
		if def.key == key {
			return Status{Key: def.key, Label: def.label, Tone: def.tone}
		}
	}
	return Status{Key: cup.KeyOther, Label: "Stato " + strconv.Itoa(code), Tone: listview.ToneClosed}
}

func statusRank(key string) int {
	for i, def := range statusDefs {
		if def.key == key {
			return i
		}
	}
	return len(statusDefs)
}
