package facilities

type facilitiesError string

func (e facilitiesError) Error() string { return string(e) }

const (
	ErrMissingColumn facilitiesError = "facilities: missing column"
	ErrEmptyFile     facilitiesError = "facilities: empty file"
	ErrNoCSV         facilitiesError = "facilities: no csv file listed"
	ErrNoResource    facilitiesError = "facilities: resource not found"
	ErrStatus        facilitiesError = "facilities: unexpected status"
	ErrTooLarge      facilitiesError = "facilities: download too large"
)

const (
	SourceMinistero = "ministero"
	SourceRegione   = "regione"
)

type Facility struct {
	Code      string `db:"code"`
	Name      string `db:"name"`
	Comune    string `db:"comune"`
	Provincia string `db:"provincia"`
	Address   string `db:"address"`
	Source    string `db:"source"`
}
