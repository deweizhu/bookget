package rslru

type Response struct {
	IsAvailable                     bool        `json:"isAvailable"`
	IsAuthorizationRequired         bool        `json:"isAuthorizationRequired"`
	IsGosuslugiVerificationRequired bool        `json:"isGosuslugiVerificationRequired"`
	Formats                         []string    `json:"formats"`
	PageCount                       int         `json:"pageCount"`
	IsSearchable                    bool        `json:"isSearchable"`
	HasTextLayer                    bool        `json:"hasTextLayer"`
	OwnershipSystem                 string      `json:"ownershipSystem"`
	AccessLevel                     string      `json:"accessLevel"`
	AccessInformationMessage        interface{} `json:"accessInformationMessage"`
	Description                     struct {
		Author  interface{} `json:"author"`
		Title   string      `json:"title"`
		Imprint string      `json:"imprint"`
	} `json:"description"`
	PrintAccess struct {
		IsPrintable             bool `json:"isPrintable"`
		IsPrintableWhenLoggedIn bool `json:"isPrintableWhenLoggedIn"`
	} `json:"printAccess"`
	ViewAccess struct {
		AvailablePdfPages []struct {
			Min int `json:"min"`
			Max int `json:"max"`
		} `json:"availablePdfPages"`
		AvailableEpubPercent    interface{} `json:"availableEpubPercent"`
		PreviewPdfPages         interface{} `json:"previewPdfPages"`
		OutOfPreviewRangeAction interface{} `json:"outOfPreviewRangeAction"`
	} `json:"viewAccess"`
	DownloadAccess struct {
		IsDownloadable      bool          `json:"isDownloadable"`
		DownloadableFormats []interface{} `json:"downloadableFormats"`
		ForbiddenReasonText interface{}   `json:"forbiddenReasonText"`
	} `json:"downloadAccess"`
	HasAudio            bool   `json:"hasAudio"`
	HasWordCoordinates  bool   `json:"hasWordCoordinates"`
	ReadingSessionId    string `json:"readingSessionId"`
	AllowedAccessTokens struct {
		Pdf bool `json:"pdf"`
	} `json:"allowedAccessTokens"`
}
