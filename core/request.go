package core

import "net/http"

const UserAgent = "packwiz/packwiz"

func GetWithUA(url string, contentType string) (resp *http.Response, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", contentType)
	return http.DefaultClient.Do(req)
}
