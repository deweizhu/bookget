package family

type ImageData struct {
	DgsNum      string
	WaypointURL string
	ImageURL    string
}
type ResultError struct {
	Error struct {
		Message     string   `json:"message"`
		FailedRoles []string `json:"failedRoles"`
		StatusCode  int      `json:"statusCode"`
	} `json:"error"`
}

type ReqData struct {
	Type string `json:"type"`
	Args struct {
		ImageURL string `json:"imageURL"`
		State    struct {
			ImageOrFilmUrl     string `json:"imageOrFilmUrl"`
			ViewMode           string `json:"viewMode"`
			SelectedImageIndex int    `json:"selectedImageIndex"`
		} `json:"state"`
		Locale string `json:"locale"`
	} `json:"args"`
}

type Response struct {
	ImageURL string `json:"imageURL"`
	ArkId    string `json:"arkId"`
	DgsNum   string `json:"dgsNum"`
	Meta     struct {
		SourceDescriptions []struct {
			Id     string `json:"id"`
			About  string `json:"about"`
			Titles []struct {
				Value string `json:"value"`
				Lang  string `json:"lang,omitempty"`
			} `json:"titles"`
			Identifiers struct {
				HttpGedcomxOrgPrimary []string `json:"http://gedcomx.org/Primary"`
			} `json:"identifiers"`
			Descriptor struct {
				Resource string `json:"resource"`
			} `json:"descriptor,omitempty"`
		} `json:"sourceDescriptions"`
	} `json:"meta"`
}

type FilmDataReqData struct {
	Type string `json:"type"`
	Args struct {
		DgsNum string `json:"dgsNum"`
		State  struct {
			I                  string `json:"i"`
			Cat                string `json:"cat"`
			ImageOrFilmUrl     string `json:"imageOrFilmUrl"`
			CatalogContext     string `json:"catalogContext"`
			ViewMode           string `json:"viewMode"`
			SelectedImageIndex int    `json:"selectedImageIndex"`
		} `json:"state"`
		Locale    string `json:"locale"`
		SessionId string `json:"sessionId"`
		LoggedIn  bool   `json:"loggedIn"`
	} `json:"args"`
}

type FilmDataResponse struct {
	DgsNum             string      `json:"dgsNum"`
	Images             []string    `json:"images"`
	PreferredCatalogId string      `json:"preferredCatalogId"`
	Type               string      `json:"type"`
	WaypointCrumbs     interface{} `json:"waypointCrumbs"`
	WaypointURL        interface{} `json:"waypointURL"`
	Templates          struct {
		DasTemplate string `json:"dasTemplate"`
		DzTemplate  string `json:"dzTemplate"`
	} `json:"templates"`
}
