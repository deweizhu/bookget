package war

type PartialVolumes struct {
	Directory string
	Title     string
	Volumes   []string
}

type DetailsInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Result  struct {
		CollectNum string `json:"collectNum"`
		Info       Info   `json:"info"`
	} `json:"result"`
}

type Info struct {
	Notes                   string      `json:"notes"`
	SecondResponsibleNation interface{} `json:"secondResponsibleNation"`
	Language                []string    `json:"language"`
	Title                   string      `json:"title"`
	OriginalPlace           []string    `json:"originalPlace"`
	Duration                string      `json:"duration"`
	SeriesVolume            string      `json:"seriesVolume"`
	SeriesSubName           string      `json:"seriesSubName"`
	PublishEvolution        interface{} `json:"publishEvolution"`
	ImageUrl                string      `json:"imageUrl"`
	StartPageId             string      `json:"startPageId"`
	PageAmount              string      `json:"pageAmount"`
	Id                      string      `json:"id"`
	Place                   []string    `json:"place"`
	CreateTimeStr           interface{} `json:"createTimeStr"`
	RedFlag                 string      `json:"redFlag"`
	TimeRange               string      `json:"timeRange"`
	FirstResponsible        []string    `json:"firstResponsible"`
	PublishTimeAll          string      `json:"publishTimeAll"`
	PublishName             string      `json:"publishName"`
	KeyWords                []string    `json:"keyWords"`
	PublishTime             string      `json:"publishTime"`
	Amount                  interface{} `json:"amount"`
	OrgName                 string      `json:"orgName"`
	DocFormat               string      `json:"docFormat"`
	DocType                 string      `json:"docType"` //ts=图书，qk=期刊，bz=报纸
	SeriesName              string      `json:"seriesName"`
	IsResearch              string      `json:"isResearch"`
	IiifObj                 struct {
		FileCode    string      `json:"fileCode"`
		UniqTag     interface{} `json:"uniqTag"`
		VolumeInfo  interface{} `json:"volumeInfo"`
		DirName     string      `json:"dirName"`
		DirCode     string      `json:"dirCode"`
		CurrentPage string      `json:"currentPage"`
		StartPageId string      `json:"startPageId"`
		ImgUrl      string      `json:"imgUrl"`
		Content     string      `json:"content"`
		JsonUrl     string      `json:"jsonUrl"`
		IsUp        interface{} `json:"isUp"`
	} `json:"iiifObj"`
	FileCode               string      `json:"fileCode"`
	FirstCreationWay       []string    `json:"firstCreationWay"`
	ContentDesc            string      `json:"contentDesc"`
	DownloadSum            string      `json:"downloadSum"`
	Version                string      `json:"version"`
	Url                    string      `json:"url"`
	FirstResponsibleNation interface{} `json:"firstResponsibleNation"`
	CreateTime             interface{} `json:"createTime"`
	PublishCycle           []string    `json:"publishCycle"`
	OriginalTitle          string      `json:"originalTitle"`
	Publisher              []string    `json:"publisher"`
	VolumeInfoAllStr       string      `json:"volumeInfoAllStr"`
	SecondCreationWay      interface{} `json:"secondCreationWay"`
	Roundup                string      `json:"roundup"`
	SecondResponsible      []string    `json:"secondResponsible"`
	Remarks                string      `json:"remarks"`
}

type Manifest struct {
	Sequences []struct {
		Canvases []struct {
			Height string `json:"height"`
			Images []struct {
				Id         string `json:"@id"`
				Type       string `json:"@type"`
				Motivation string `json:"motivation"`
				On         string `json:"on"`
				Resource   struct {
					Format  string `json:"format"`
					Height  string `json:"height"`
					Id      string `json:"@id"`
					Type    string `json:"@type"`
					Service struct {
						Protocol string `json:"protocol"`
						Profile  string `json:"profile"`
						Width    int    `json:"width"`
						Id       string `json:"@id"`
						Context  string `json:"@context"`
						Height   int    `json:"height"`
					} `json:"service"`
					Width string `json:"width"`
				} `json:"resource"`
			} `json:"images"`
			Id    string `json:"@id"`
			Type  string `json:"@type"`
			Label string `json:"label"`
			Width string `json:"width"`
		} `json:"canvases"`
		Id               string `json:"@id"`
		Type             string `json:"@type"`
		Label            string `json:"label"`
		ViewingDirection string `json:"viewingDirection"`
		ViewingHint      string `json:"viewingHint"`
	} `json:"sequences"`
}

type Qk struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Result  []struct {
		Title string `json:"title"`
		List  []struct {
			Year     string `json:"year"`
			DataList []struct {
				Id        string `json:"id"`
				Directory string `json:"directory"`
			} `json:"dataList"`
		} `json:"list"`
	} `json:"result"`
}

type FindDirectoryByMonth struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Result  []struct {
		Date    string `json:"date"`
		IiifObj struct {
			FileCode    string      `json:"fileCode"`
			UniqTag     string      `json:"uniqTag"`
			VolumeInfo  interface{} `json:"volumeInfo"`
			DirName     string      `json:"dirName"`
			DirCode     string      `json:"dirCode"`
			CurrentPage string      `json:"currentPage"`
			StartPageId string      `json:"startPageId"`
			ImgUrl      string      `json:"imgUrl"`
			Content     string      `json:"content"`
			JsonUrl     string      `json:"jsonUrl"`
			IsUp        interface{} `json:"isUp"`
		} `json:"iiifObj"`
		DirCode     string `json:"dirCode"`
		StartPageId string `json:"startPageId"`
		Id          string `json:"id"`
	} `json:"result"`
}
