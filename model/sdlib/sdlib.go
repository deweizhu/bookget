package sdlib

import "time"

type Item struct {
	Id                 string      `json:"id"`
	BusinessId         string      `json:"businessId"`
	CreateBy           interface{} `json:"createBy"`
	CreateTime         time.Time   `json:"createTime"`
	DelFlag            int         `json:"delFlag"`
	DeptId             interface{} `json:"deptId"`
	FileExtension      string      `json:"fileExtension"`
	FileSize           int         `json:"fileSize"`
	FileType           int         `json:"fileType"`
	Name               string      `json:"name"`
	OrignalName        string      `json:"orignalName"`
	Content            interface{} `json:"content"`
	SimpleContent      interface{} `json:"simpleContent"`
	Sort               int         `json:"sort"`
	SystemEnglishName  interface{} `json:"systemEnglishName"`
	SystemId           int         `json:"systemId"`
	Url                string      `json:"url"`
	UseType            int         `json:"useType"`
	VolumeName         string      `json:"volumeName"`
	CatalogId          int64       `json:"catalogId"`
	ResourceFileSearch interface{} `json:"resourceFileSearch"`
	MoveFlag           interface{} `json:"moveFlag"`
}

type Intro struct {
	ResId                   string      `json:"resId"`
	Name                    string      `json:"name"`
	PublishAddress          string      `json:"publishAddress"`
	Edition                 string      `json:"edition"`
	Binding                 string      `json:"binding"`
	VolumeNum               int         `json:"volumeNum"`
	SubmitDept              string      `json:"submitDept"`
	Language                string      `json:"language"`
	Type                    string      `json:"type"`
	NationalNumber          string      `json:"nationalNumber"`
	RegistrationNumber      interface{} `json:"registrationNumber"`
	CensusNumber            interface{} `json:"censusNumber"`
	Responsibility          string      `json:"responsibility"`
	ResponsibilityDesc      string      `json:"responsibilityDesc"`
	ResponsibilityMode      string      `json:"responsibilityMode"`
	OtherResponsibility     string      `json:"otherResponsibility"`
	OtherResponsibilityDesc string      `json:"otherResponsibilityDesc"`
	OtherResponsibilityMode string      `json:"otherResponsibilityMode"`
	PublisherName           string      `json:"publisherName"`
	PublicationPlace        interface{} `json:"publicationPlace"`
	PublicationMethod       interface{} `json:"publicationMethod"`
	VersionDesc             string      `json:"versionDesc"`
	Proofreading            interface{} `json:"proofreading"`
	PublicationDateYear     string      `json:"publicationDateYear"`
	PublicationDateCalendar string      `json:"publicationDateCalendar"`
	ViewCount               int         `json:"viewCount"`
	ShowFlag                int         `json:"showFlag"`
	UploadFlag              int         `json:"uploadFlag"`
	Dynasty                 string      `json:"dynasty"`
	LibraryId               string      `json:"libraryId"`
	EditionDesc             string      `json:"editionDesc"`
	DelFlag                 int         `json:"delFlag"`
	CreateBy                int         `json:"createBy"`
	CreateTime              string      `json:"createTime"`
	UpdateBy                interface{} `json:"updateBy"`
	UpdateTime              interface{} `json:"updateTime"`
	TypeName                string      `json:"typeName"`
	LabelNames              string      `json:"labelNames"`
	TypeId                  string      `json:"typeId"`
	LabelId                 string      `json:"labelId"`
	AncientBookFlag         int         `json:"ancientBookFlag"`
}
type ResponseIntro struct {
	Msg  string  `json:"msg"`
	Code int     `json:"code"`
	Data []Intro `json:"data"`
}

type Response struct {
	Msg  string `json:"msg"`
	Code int    `json:"code"`
	Data []Item `json:"data"`
}
