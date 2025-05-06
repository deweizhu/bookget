package onbdigital

type Response struct {
	ImageData []struct {
		ImageID     string `json:"imageID"`
		OrderNumber string `json:"orderNumber"`
		QueryArgs   string `json:"queryArgs"`
	} `json:"imageData"`
}
