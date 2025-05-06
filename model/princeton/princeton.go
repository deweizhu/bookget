package princeton

// Graphql 查manifestUrl
type Graphql struct {
	Data struct {
		ResourcesByOrangelightIds []struct {
			Id          string `json:"id"`
			Url         string `json:"url"`
			ManifestUrl string `json:"manifestUrl"`
		} `json:"resourcesByOrangelightIds"`
	} `json:"data"`
}

type ResponseManifest struct {
	Manifests []struct {
		Id string `json:"@id"`
	} `json:"manifests"`
}

// Manifest 查info.json
type ResponseManifest2 struct {
	Sequences []struct {
		Canvases []struct {
			Images []struct {
				Resource struct {
					Service struct {
						Id string `json:"@id"`
					} `json:"service"`
				} `json:"resource"`
			} `json:"images"`
		} `json:"canvases"`
	} `json:"sequences"`
}
