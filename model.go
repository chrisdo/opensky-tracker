package openskytracker

type FlightsResponse struct {
	Error   string   `json:"error"`
	Flights []Flight `json:"flights"`
}

type Flight struct {
	ID       string  `json:"id,omitempty"`
	Lat      float64 `json:"lat,omitempty"`
	Lon      float64 `json:"lon,omitempty"`
	Heading  float64 `json:"heading,omitempty"`
	Altitude float64 `json:"altitude,omitempty"`
	Callsign string  `json:"callsign,omitempty"`
}
