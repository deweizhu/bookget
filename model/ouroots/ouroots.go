package ouroots

type ResponseLoginAnonymousUser struct {
	StatusCode string `json:"statusCode"`
	Msg        string `json:"msg"`
	Token      string `json:"token"`
}

type ResponseCatalogImage struct {
	StatusCode string `json:"statusCode"`
	Msg        string `json:"msg"`
	ImagePath  string `json:"imagePath"`
	ImageSize  int    `json:"imageSize"`
	DocPath    string `json:"docPath"`
}

type Volume struct {
	Name     string `json:"name"`
	Pages    int    `json:"pages"`
	VolumeId int    `json:"volumeId"`
}

type Catalogue struct {
	Key           string `json:"_key"`
	Id            string `json:"_id"`
	Rev           string `json:"_rev"`
	BatchID       string `json:"batchID"`
	PageProp      string `json:"page_prop"`
	BookId        string `json:"book_id"`
	ChapterName   string `json:"chapter_name"`
	SerialNum     string `json:"serial_num"`
	AdminId       string `json:"adminId"`
	CreateTime    int64  `json:"createTime"`
	IsLike        bool   `json:"isLike"`
	IsCollect     bool   `json:"isCollect"`
	ViewNum       int    `json:"viewNum"`
	LikeNum       int    `json:"likeNum"`
	CollectionNum int    `json:"collectionNum"`
	ShareNum      int    `json:"shareNum"`
	VolumeID      int    `json:"volumeID"`
	EndNum        *int   `json:"end_num"`
	VolumeNum     string `json:"volume_num,omitempty"`
	PageNum       string `json:"page_num,omitempty"`
}
type ResponseVolume struct {
	StatusCode string      `json:"statusCode"`
	Msg        string      `json:"msg"`
	Volume     []Volume    `json:"volume"`
	Catalogue  []Catalogue `json:"catalogue"`
}
