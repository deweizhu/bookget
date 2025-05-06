package szLib

type ResultVolumes struct {
	Meta struct {
		Topic  string `json:"topic"`
		Uri856 struct {
			Image struct {
				Brief     []interface{} `json:"brief"`
				Publish   []interface{} `json:"publish"`
				IndexPage []interface{} `json:"indexPage"`
			} `json:"image"`
			Pdf struct {
				FullText []interface{} `json:"fullText"`
			} `json:"pdf"`
		} `json:"uri_856"`
		UAbstract         string `json:"U_Abstract"`
		USubject          string `json:"U_Subject"`
		UContributor      string `json:"U_Contributor"`
		UAuthor           string `json:"U_Author"`
		UTitle            string `json:"U_Title"`
		UPublishYear      string `json:"U_PublishYear"`
		UZuozheFangshi    string `json:"U_Zuozhe_Fangshi"`
		ULibrary          string `json:"U_Library"`
		UZhuangzhenxinshi string `json:"U_Zhuangzhenxinshi"`
		UPublisher        string `json:"U_Publisher"`
		UPlace            string `json:"U_Place"`
		UCunjuan          string `json:"U_Cunjuan"`
		UZuozheShijian    string `json:"U_Zuozhe_Shijian"`
		USeries           string `json:"U_Series"`
		UZiliaojibie      string `json:"U_Ziliaojibie"`
		UGuest            string `json:"U_Guest"`
		UTimingJianti     string `json:"U_Timing_Jianti"`
		UKeywords         string `json:"U_Keywords"`
		ULibCallno        string `json:"U_LibCallno"`
		UVersionLeixin    string `json:"U_Version_Leixin"`
		USubjectRegular2  string `json:"U_Subject_Regular2"`
		USubjectRegular   string `json:"U_Subject_Regular"`
		UExpectDate       string `json:"U_ExpectDate"`
		UPage             string `json:"U_Page"`
		UVersion          string `json:"U_Version"`
		UPuchabianhao     string `json:"U_Puchabianhao"`
		UCallno           string `json:"U_Callno"`
		UPublish          string `json:"U_Publish"`
		UFence            string `json:"U_Fence"`
		Volume            string `json:"volume"`
	} `json:"meta"`
	Volumes []Directory `json:"directory"`
}

type Directory struct {
	Name     string `json:"name"`
	Volume   string `json:"volume"`
	Page     string `json:"page"`
	HasText  string `json:"has_text"`
	Children []struct {
		Volume   string        `json:"volume"`
		Children []interface{} `json:"children"`
		Page     string        `json:"page"`
	} `json:"children"`
}

type ResultPage struct {
	TextInfo struct {
	} `json:"text_info"`
	PicInfo struct {
		Description string `json:"description"`
		Tolvol      string `json:"tolvol"`
		Period      string `json:"period"`
		Topic       string `json:"topic"`
		Title       string `json:"title"`
		Path        string `json:"path"`
	} `json:"pic_info"`
	BookImageUrl string `json:"book_image_url"`
}
