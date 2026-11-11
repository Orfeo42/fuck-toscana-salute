package referti

import "fmt"

type Kind string

const (
	KindLab         Kind = "lab"
	KindTransfusion Kind = "labtrasf"
	KindRadiology   Kind = "ris"
	KindDischarge   Kind = "ldo"
	KindER          Kind = "ps"
	KindSpecialist  Kind = "rsa"
	KindPathology   Kind = "rap"
)

var allKinds = []Kind{KindLab, KindTransfusion, KindRadiology, KindDischarge, KindER, KindSpecialist, KindPathology}

func (k Kind) Label() string {
	switch k {
	case KindLab:
		return "Laboratorio analisi"
	case KindTransfusion:
		return "Trasfusionale"
	case KindRadiology:
		return "Radiologia"
	case KindDischarge:
		return "Lettere di dimissione"
	case KindER:
		return "Pronto soccorso"
	case KindSpecialist:
		return "Specialistica ambulatoriale"
	case KindPathology:
		return "Anatomia patologica"
	}
	return string(k)
}

func ParseKind(s string) (Kind, error) {
	for _, k := range allKinds {
		if string(k) == s {
			return k, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownKind, s)
}
