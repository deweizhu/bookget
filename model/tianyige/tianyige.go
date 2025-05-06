package tianyige

type ResponseVolume struct {
	Code int      `json:"code"`
	Msg  string   `json:"msg"`
	Data []Volume `json:"data"`
}
type PageImage struct {
	Records     []ImageRecord `json:"records"`
	Total       int           `json:"total"`
	Size        int           `json:"size"`
	Current     int           `json:"current"`
	SearchCount bool          `json:"searchCount"`
	Pages       int           `json:"pages"`
}
type ImageRecord struct {
	ImageId     string      `json:"imageId"`
	ImageName   string      `json:"imageName"`
	DirectoryId string      `json:"directoryId"`
	FascicleId  string      `json:"fascicleId"`
	CatalogId   string      `json:"catalogId"`
	Sort        int         `json:"sort"`
	Type        int         `json:"type"`
	IsParse     interface{} `json:"isParse"`
	Description interface{} `json:"description"`
	Creator     string      `json:"creator"`
	CreateTime  string      `json:"createTime"`
	Updator     string      `json:"updator"`
	UpdateTime  string      `json:"updateTime"`
	IsDeleted   int         `json:"isDeleted"`
	OcrInfo     interface{} `json:"ocrInfo"`
	File        interface{} `json:"file"`
}

// 页面
type ResponsePage struct {
	Code int       `json:"code"`
	Msg  string    `json:"msg"`
	Data PageImage `json:"data"`
}

type ResponseFile struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		File []struct {
			FileName    string `json:"fileName"`
			FileSuffix  string `json:"fileSuffix"`
			FilePath    string `json:"filePath"`
			UpdateTime  string `json:"updateTime"`
			Sort        string `json:"sort"`
			CreateTime  string `json:"createTime"`
			FileSize    int    `json:"fileSize"`
			FileOldname string `json:"fileOldname"`
			FileInfoId  string `json:"fileInfoId"`
		} `json:"file"`
	} `json:"data"`
}

type Volume struct {
	FascicleId   string      `json:"fascicleId"`
	CatalogId    string      `json:"catalogId"`
	Name         string      `json:"name"`
	Introduction interface{} `json:"introduction"`
	GradeId      string      `json:"gradeId"`
	Sort         int         `json:"sort"`
	Creator      interface{} `json:"creator"`
	CreateTime   string      `json:"createTime"`
	Updator      interface{} `json:"updator"`
	UpdateTime   string      `json:"updateTime"`
	IsDeleted    int         `json:"isDeleted"`
	FilePath     interface{} `json:"filePath"`
	ImageCount   interface{} `json:"imageCount"`
}

type Catalog struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Records []struct {
			DirectoryId string      `json:"directoryId"`
			FascicleId  string      `json:"fascicleId"`
			CatalogId   string      `json:"catalogId"`
			Name        string      `json:"name"`
			Description interface{} `json:"description"`
			PageId      string      `json:"pageId"`
			GradeId     string      `json:"gradeId"`
			Region      string      `json:"region"`
			Sort        int         `json:"sort"`
			Creator     interface{} `json:"creator"`
			CreateTime  string      `json:"createTime"`
			Updator     interface{} `json:"updator"`
			UpdateTime  *string     `json:"updateTime"`
			IsDeleted   int         `json:"isDeleted"`
		} `json:"records"`
		Total       int  `json:"total"`
		Size        int  `json:"size"`
		Current     int  `json:"current"`
		SearchCount bool `json:"searchCount"`
		Pages       int  `json:"pages"`
	} `json:"data"`
}

type Parts map[string][]ImageRecord
