package login

type Urls struct {
	Items []string `json:"URLs"`
}

func (urls *Urls) AddItem(item string) {
	urls.Items = append(urls.Items, item)
}
