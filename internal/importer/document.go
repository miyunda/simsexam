package importer

type Document struct {
	HeaderSubjectSlug string
	Manifest          Manifest
	Questions         []Question
}

type Manifest struct {
	Slug            string
	Title           string
	Description     string
	DurationMinutes int
	QuestionCount   int
	AccessLevel     string
	Status          string
	Version         string
}

type Question struct {
	Key         string
	Type        string
	Stem        string
	Explanation string
	Options     []Option
	Line        int
}

type Option struct {
	Text      string
	IsCorrect bool
	Line      int
}

type ParseError struct {
	Line    int
	Message string
}

func (e *ParseError) Error() string {
	if e.Line > 0 {
		return "line " + itoa(e.Line) + ": " + e.Message
	}
	return e.Message
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}

	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}

	var digits [20]byte
	i := len(digits)
	for v > 0 {
		i--
		digits[i] = byte('0' + (v % 10))
		v /= 10
	}

	return sign + string(digits[i:])
}
