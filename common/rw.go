package common

import (
	"encoding/csv"
	"os"
	"strings"
)

func ReadTasksFromCsv(filename string) ([]Task, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	lines, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var tasks []Task
	for _, line := range lines {
		tasks = append(tasks, Task{
			IP:       strings.TrimSpace((line[0])),
			Domain:   strings.TrimSpace((line[1])),
			Endpoint: strings.TrimSpace(line[2]),
		})
	}

	return tasks, nil
}
