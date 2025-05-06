package korea

type Response struct {
	ImgInfos []struct {
		BookNum  string `json:"bookNum"`
		Num      string `json:"num"`
		BookPath string `json:"bookPath"`
		ImgNum   string `json:"imgNum"`
		Fname    string `json:"fname"`
	} `json:"imgInfos"`
	BookNum string `json:"bookNum"`
}

type PartialCanvases struct {
	Directory string
	Title     string
	Canvases  []string
}
