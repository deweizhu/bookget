package family

type ImageData struct {
	DgsNum      string
	WaypointURL string
	ImageURL    string
}
type ResultError struct {
	Error struct {
		Message     string   `json:"message"`
		FailedRoles []string `json:"failedRoles"`
		StatusCode  int      `json:"statusCode"`
	} `json:"error"`
}
