package njuedu

type Catalog struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    []struct {
		BookId         string        `json:"bookId"`
		BookName       string        `json:"bookName"`
		VolumeNum      string        `json:"volumeNum"`
		ImgDescription interface{}   `json:"imgDescription"`
		Catalogues     []interface{} `json:"catalogues"`
	} `json:"data"`
}
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Title      string   `json:"title"`
		ServerBase string   `json:"serverBase"`
		Images     []string `json:"images"`
	} `json:"data"`
}

type Detail struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    []struct {
		Id             int    `json:"id"`
		BookId         string `json:"bookId"`
		Num            int    `json:"num"`
		AttributeId    int    `json:"attributeId"`
		AttributeValue string `json:"attributeValue"`
		Operator       int    `json:"operator"`
		OperatorName   string `json:"operatorName"`
		CreateTime     int    `json:"createTime"`
		UpdateTime     int    `json:"updateTime"`
		TypeId         int    `json:"typeId"`
		Captions       string `json:"captions"`
	} `json:"data"`
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

type ResponseTiles struct {
	Tiles map[string]Item `json:"tiles"`
}
