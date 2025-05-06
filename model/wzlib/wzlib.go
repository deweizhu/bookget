package wzlib

type Digital struct {
	ID                  int    `json:"ID"`
	SiteID              int    `json:"SiteID"`
	Title               string `json:"Title"`
	Author              string `json:"author"`
	Source              string `json:"source"`
	Txt                 string `json:"txt"`
	PdfUrl              string `json:"pdf_url"`
	DigitalResourceData []struct {
		Title string `json:"Title"`
		Url   string `json:"Url"`
	} `json:"DigitalResourceData"`
}

type Result []Item

type Item struct {
	Items []struct {
		Id          string `json:"_id"`
		DcPublisher string `json:"dc_publisher"`
		DcTitle     string `json:"dc_title"`
		WzlPdfUrl   string `json:"wzl_pdf_url"`
	} `json:"items"`
	Title string `json:"title"`
}

type PdfUrls []PdfUrl
type PdfUrl struct {
	Url  string
	Name string
}

type ResultPdf struct {
	Data struct {
		Id         string `json:"_id"`
		DcTitle    string `json:"dc_title"`
		ModelId    string `json:"model_id"`
		RelateName string `json:"relate_name"`
		WzlPdfUrl  string `json:"wzl_pdf_url"`
	} `json:"Data"`

	RelateList []interface{} `json:"RelateList"`
}
