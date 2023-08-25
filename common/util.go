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

func PrintDstChange(ori string, dst string) DstChangePrint {
	dstL := strings.Split(dst, " ")
	dstURL := dstL[len(dstL)-1]
	dstParsed, err := url.Parse(dstURL)
	if err != nil {
		fmt.Println("do something") //TODO: fix this
	}
	oriParsed, err := url.Parse(ori)
	if err != nil {
		fmt.Println("do something") //TODO: fix this
	}

	dstChangePrint := DstChangePrint{}
	dstChangePrint.Scheme = dstParsed.Scheme != oriParsed.Scheme
	dstChangePrint.Hostname = dstParsed.Hostname() != oriParsed.Hostname()
	dstChangePrint.Path = dstParsed.Path != oriParsed.Path
	dstChangePrint.Query = dstParsed.RawQuery != oriParsed.RawQuery

	return dstChangePrint
}

func PrintTask(task Task) TaskPrint {
	taskPrint := TaskPrint{
		SourceURL: task.SourceURL,
		Domain:    task.Domain,
		URL:       task.URL,
		IP:        task.IP,
	}

	taskPrint.RedirectChain = task.RedirectChain
	if len(taskPrint.RedirectChain) != 0 { // src -> dst change summary
		dst := taskPrint.RedirectChain[len(taskPrint.RedirectChain)-1]
		taskPrint.DstChange = PrintDstChange(taskPrint.URL, dst)
	}
	if task.Resp != nil {
		taskPrint.Resp = PrintResp(*task.Resp)
	}
	if task.Err != nil {
		taskPrint.Err = task.Err.Error()
	}

	if task.Retry != nil && task.Retry.Retried {
		taskPrint.Retry.Retried = task.Retry.Retried
		taskPrint.Retry.RedirectChain = task.Retry.RedirectChain
		if len(taskPrint.Retry.RedirectChain) != 0 { // src -> dst change summary
			dst := taskPrint.Retry.RedirectChain[len(taskPrint.RedirectChain)-1]
			dst = strings.Split(dst, "")[len(strings.Split(dst, ""))-1]
			taskPrint.Retry.DstChange = PrintDstChange(taskPrint.URL, dst)
		}
		if task.Retry.Resp != nil {
			taskPrint.Retry.Resp = PrintResp(*task.Retry.Resp)
		}
		if task.Retry.Err != nil {
			taskPrint.Retry.Err = (*task.Retry).Err.Error()
		}
	}
	return taskPrint
}
