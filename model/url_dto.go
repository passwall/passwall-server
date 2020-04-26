package model

// URLs ...
type URLs struct {
	Items []string `json:"URLs"`
}

// AddItem ...
func (urls *URLs) AddItem(item string) {
	urls.Items = append(urls.Items, item)
}
