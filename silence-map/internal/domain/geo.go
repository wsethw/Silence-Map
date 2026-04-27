package domain

type Point struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Bounds struct {
	North float64 `json:"north"`
	South float64 `json:"south"`
	East  float64 `json:"east"`
	West  float64 `json:"west"`
}

func (b Bounds) Valid() bool {
	return b.North >= b.South &&
		b.East >= b.West &&
		b.North <= 90 &&
		b.South >= -90 &&
		b.East <= 180 &&
		b.West >= -180
}

func (b Bounds) Contains(p Point) bool {
	if !b.Valid() {
		return false
	}

	return p.Latitude <= b.North &&
		p.Latitude >= b.South &&
		p.Longitude <= b.East &&
		p.Longitude >= b.West
}
