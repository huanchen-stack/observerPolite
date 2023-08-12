package common

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
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
		line := strings.TrimSpace(scanner.Text())
		URLs := strings.Split(line, ",")
		URL := strings.TrimSpace(URLs[0])
		parsedURL, err := url.Parse(URL)
		if err != nil {
			fmt.Println("Error extracting domain from URL", err)
			continue // TODO: check if this is still executed... should be handled before
		}
		src := "" // program still works even if no sources are provided
		if len(URLs) > 1 {
			src = strings.TrimSpace(URLs[1])
		}

		tasks = append(tasks, Task{
			SourceURL: src,
			Domain:    parsedURL.Hostname(),
			URL:       URL,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

func computeETag(data []byte) string {
	hash := sha1.New()
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}

func PrintResp(resp http.Response) RespPrint {
	storableResp := RespPrint{
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
	}

	if eTag, ok := resp.Header["Etag"]; ok {
		storableResp.ETag = strings.Trim(eTag[0], "\"")
	}
	buf := make([]byte, GlobalConfig.ESelfTagBuffLen)
	n, err := io.ReadAtLeast(resp.Body, buf, GlobalConfig.ESelfTagBuffLen) // n = min(len, N)
	if err != nil && err != io.ErrUnexpectedEOF {
		//	TODO: DO SOMETHING
	}
	storableResp.ESelfTag = computeETag(buf[:n])

	return storableResp
}

func PrintTask(task Task) TaskPrint {
	taskPrint := TaskPrint{
		SourceURL: task.SourceURL,
		Domain:    task.Domain,
		URL:       task.URL,
		IP:        task.IP,
	}

	taskPrint.RedirectChain = task.RedirectChain
	if task.Resp != nil {
		taskPrint.Resp = PrintResp(*task.Resp)
	}
	if task.Err != nil {
		taskPrint.Err = task.Err.Error()
	}

	if task.AutoRetryHTTPS != nil && task.AutoRetryHTTPS.Retried {
		taskPrint.AutoRetryHTTPS.Retried = task.AutoRetryHTTPS.Retried
		taskPrint.AutoRetryHTTPS.RedirectChain = task.AutoRetryHTTPS.RedirectChain
		if task.AutoRetryHTTPS.Resp != nil {
			taskPrint.AutoRetryHTTPS.Resp = PrintResp(*task.AutoRetryHTTPS.Resp)
		}
		if task.AutoRetryHTTPS.Err != nil {
			taskPrint.AutoRetryHTTPS.Err = (*task.AutoRetryHTTPS).Err.Error()
		}
	}
	return taskPrint
}
