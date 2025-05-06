package iiif

// ManifestResponse by view-source:https://iiif.lib.harvard.edu/manifests/drs:53262215
type ManifestResponse struct {
	Sequences []struct {
		Canvases []struct {
			Id   string `json:"@id"`
			Type string `json:"@type"`
			//兼容某些不正规的网站竟然用了string类型，见https://digitalarchive.npm.gov.tw/Antique/setJsonU?uid=58102&Dept=U
			//Height int    `json:"height"`
			Images []struct {
				Id       string `json:"@id"`
				Type     string `json:"@type"`
				On       string `json:"on"`
				Resource struct {
					Id     string `json:"@id"`
					Type   string `json:"@type"`
					Format string `json:"format"`
					//兼容digitalarchive.npm.gov.tw
					//Height  int    `json:"height"`
					Service struct {
						Id string `json:"@id"`
					} `json:"service"`
					Width int `json:"width"`
				} `json:"resource"`
			} `json:"images"`
			Label string `json:"label"`
			//Width int    `json:"width"`
		} `json:"canvases"`
	} `json:"sequences"`
}

// ManifestV3Response  https://iiif.io/api/presentation/3.0/#52-manifest
type ManifestV3Response struct {
	Id    string `json:"id"`
	Type  string `json:"type"`
	Label struct {
		None []string `json:"none"`
	} `json:"label"`
	Height   int `json:"height"`
	Width    int `json:"width"`
	Canvases []struct {
		Id     string `json:"id"`
		Type   string `json:"type"`
		Height int    `json:"height"`
		Width  int    `json:"width"`
		Items  []struct {
			Id    string `json:"id"`
			Type  string `json:"type"`
			Items []struct {
				Id         string `json:"id"`
				Type       string `json:"type"`
				Motivation string `json:"motivation"`
				Body       struct {
					Id      string `json:"id"`
					Type    string `json:"type"`
					Format  string `json:"format"`
					Service []struct {
						Id   string `json:"id"`
						Type string `json:"type"`
						//![ See https://da.library.pref.osaka.jp/api/items/03-0000183/manifest.json
						Id_   string `json:"@id"`
						Type_ string `json:"@type"`
						//]!
						Profile string `json:"profile"`
					} `json:"service"`
					Height int `json:"height"`
					Width  int `json:"width"`
				} `json:"body"`
				Target string `json:"target"`
			} `json:"items"`
		} `json:"items"`
	} `json:"items"`
	Annotations []struct {
		Id    string        `json:"id"`
		Type  string        `json:"type"`
		Items []interface{} `json:"items"`
	} `json:"annotations"`
}

type ManifestPresentation struct {
	Context string `json:"@context"`
	Id      string `json:"id"`
}
