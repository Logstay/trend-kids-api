package ports

type SaveProduct struct {
	Name        string
	Category    string
	Description string
	PriceCents  int64
	Stock       int
}

type UpdateProduct struct {
	UID         string
	Name        string
	Category    string
	Description string
	PriceCents  int64
	Active      bool
}

type AdjustStock struct {
	UID      string
	Delta    int
	Reason   string
	AdminUID string
}

const MaxReasonLen = 200
