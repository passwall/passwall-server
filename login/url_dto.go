package login

type Url struct {
	Name string `json:"url"`
}

type Urls struct {
	Items []Url `json:"urls"`
}

func (urls *Urls) AddItem(item Url) {
	urls.Items = append(urls.Items, item)
}
