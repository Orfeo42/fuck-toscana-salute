package referti

import (
	"fmt"
	"strings"
)

type Kind string

const (
	KindLab         Kind = "lab"
	KindTransfusion Kind = "labtrasf"
	KindRadiology   Kind = "ris"
	KindDischarge   Kind = "ldo"
	KindER          Kind = "ps"
	KindSpecialist  Kind = "rsa"
	KindPathology   Kind = "rap"
	KindLocal       Kind = "local"
	KindFSE         Kind = "fse"
)

const refKeySeparator = "\x1f"

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
	case KindLocal:
		return "Caricato"
	case KindFSE:
		return "Fascicolo Sanitario"
	}
	return string(k)
}

func ParseKind(s string) (Kind, error) {
	if s == string(KindLocal) {
		return KindLocal, nil
	}
	for _, k := range allKinds {
		if string(k) == s {
			return k, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownKind, s)
}

func (r Ref) Key() string {
	return strings.Join([]string{string(r.Kind), r.AbstractID, r.DetailID, r.RepCode}, refKeySeparator)
}

func ParseRefKey(key string) (Ref, error) {
	parts := strings.Split(key, refKeySeparator)
	if len(parts) != 4 {
		return Ref{}, fmt.Errorf("%w: malformed ref key", ErrUnknownKind)
	}
	kind, err := ParseKind(parts[0])
	if err != nil {
		return Ref{}, err
	}
	return Ref{Kind: kind, AbstractID: parts[1], DetailID: parts[2], RepCode: parts[3]}, nil
}
