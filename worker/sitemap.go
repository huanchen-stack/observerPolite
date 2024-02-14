package worker

import (
	"io"
	"net/http"
	cm "observerPolite/common"
)

//func (gw *GeneralWorker) SitemapFetch(URL string) {
//	sitemap.SetFetch(myFetch)
//
//	//fmt.Println("--- fetching site map for url", URL)
//	smap, err := sitemap.Get(URL+"/sitemap.xml", nil)
//	if err != nil {
//		return
//	}
//	gw.SPConn.WriteChan <- smap
//	//fmt.Println("adding sitemap.xml of", URL, "to db")
//}

func myFetch(URL string, options interface{}) ([]byte, error) {
	req, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		return []byte{}, err
	}

	// Set User-Agent
	req.Header.Set("User-Agent", cm.GlobalConfig.UserAgent)

	// Set timeout
	client := http.Client{
		Timeout: cm.GlobalConfig.Timeout,
	}

	// Fetch data
	res, err := client.Do(req)
	if err != nil {
		return []byte{}, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return []byte{}, err
	}

	return body, err
}
