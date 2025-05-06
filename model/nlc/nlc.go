package nlc

// 基础响应结构
type BaseResponse struct {
	Msg  string `json:"msg"`
	Code int    `json:"code"`
	Data Node   `json:"data"`
}

// 数据项结构
type DataItem struct {
	OrderSeq    string `json:"orderSeq"`
	ImageId     string `json:"imageId"`
	StructureId int    `json:"structureId"`
	PageNum     int    `json:"pageNum"`
}

// 通用节点结构（用于多级嵌套）
type Node struct {
	ImageIdList []DataItem `json:"imageIdList"`
	Total       int        `json:"total"`
}

type ImageData struct {
	Msg  string `json:"msg"`
	Code int    `json:"code"`
	Data struct {
		FileName    string `json:"fileName"`
		ImageId     int    `json:"imageId"`
		FilePath    string `json:"filePath"`
		StructureId int    `json:"structureId"`
		UpdateTime  string `json:"updateTime"`
		CreateTime  string `json:"createTime"`
		FileType    string `json:"fileType"`
	} `json:"data"`
}

// 目录
type StructureResponse struct {
	Code int      `json:"code"`
	Data []Volume `json:"data"`
}

type Volume struct {
	Children []CatalogItem `json:"children"`
}

type CatalogItem struct {
	Title    string        `json:"volumeTitleAndArticleTitle"`
	ImageIDs []interface{} `json:"imageIdList"`
	Children []CatalogItem `json:"children"`
}

type PageResponse struct {
	Data struct {
		ImageIDList []PageItem `json:"imageIdList"`
	} `json:"data"`
}

type PageItem struct {
	ImageID interface{} `json:"imageId"`
	PageNum interface{} `json:"pageNum"`
}
