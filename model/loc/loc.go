package loc

type ManifestsJson struct {
	Resources []struct {
		Caption string        `json:"caption"`
		Files   [][]ImageFile `json:"files"`
		Image   string        `json:"image"`
		Url     string        `json:"url"`
	} `json:"resources"`
}
type ImageFile struct {
	Height   *int   `json:"height"`
	Levels   int    `json:"levels"`
	Mimetype string `json:"mimetype"`
	Url      string `json:"url"`
	Width    *int   `json:"width"`
	Info     string `json:"info,omitempty"`
	Size     int    `json:"size,omitempty"`
}
