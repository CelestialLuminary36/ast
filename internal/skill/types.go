package skill

type Skill struct {
	ID           string
	Name         string
	Path         string
	Instructions string
	Tools        []byte
	Meta         map[string]any
}
