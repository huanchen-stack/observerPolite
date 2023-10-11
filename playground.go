package main

import (
	"fmt"
	"io"
	"net/http"
)

func GetRobotsTxt(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func main() {
	s, _ := GetRobotsTxt("https://www.google.com/robots.txt")
	fmt.Println(s)
}
