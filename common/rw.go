package common

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strings"
)

func ReadTasksFromInput(filename string) ([]Task, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	var tasks []Task
	for scanner.Scan() {
		URL := strings.TrimSpace(scanner.Text())

		parsedURL, err := url.Parse(URL)
		if err != nil {
			fmt.Println("Error extracting domain from URL", err)
			continue
		}

		tasks = append(tasks, Task{
			Domain: parsedURL.Hostname(),
			URL:    URL,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}
