package iiif

type ServerResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Title      string   `json:"title"`
		ServerBase string   `json:"serverBase"`
		Images     []string `json:"images"`
	} `json:"data"`
}

type BaseResponse struct {
	Tiles map[string]Item `json:"tiles"`
}

type Item struct {
	Extension   string `json:"extension"`
	Height      int    `json:"height"`
	Resolutions int    `json:"resolutions"`
	TileSize    struct {
		H int `json:"h"`
		W int `json:"w"`
	} `json:"tile_size"`
	TileSize2 struct {
		Height int `json:"height"`
		Width  int `json:"width"`
	} `json:"tileSize"`
	Width int `json:"width"`
}

type DziFormat struct {
	Xmlns    string `json:"xmlns"`
	Url      string `json:"Url"`
	Overlap  int    `json:"Overlap"`
	TileSize int    `json:"TileSize"`
	Format   string `json:"Format"`
	Size     struct {
		Width  int `json:"Width"`
		Height int `json:"Height"`
	} `json:"Size"`
}
