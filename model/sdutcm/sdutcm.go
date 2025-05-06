package sdutcm

type PagePicTxt struct {
	Url       string `json:"url"`
	Text      string `json:"text"`
	Charmax   int    `json:"charmax"`
	ColNum    int    `json:"colNum"`
	PageNum   string `json:"pageNum"`
	ImageList struct {
	} `json:"imageList"`
}
type VolumeList struct {
	List []struct {
		ShortTitle string `json:"short_title"`
		ContentId  string `json:"content_id"`
		Lshh       string `json:"lshh"`
		Title      string `json:"title"`
	} `json:"list"`
}
