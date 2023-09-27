package common

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func getExcludedHostnames() map[string]struct{} {
	file, err := os.Open("excluded_domains.txt")
	if err != nil {
		panic("excluded_domains.txt doesn't exist!")
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	excludedDomains := make(map[string]struct{})
	for scanner.Scan() {
		domain := strings.TrimSpace(scanner.Text())
		fmt.Println(domain)
		excludedDomains[domain] = struct{}{}
	}

	return excludedDomains
}

func ReadTaskStrsFromInput(filename string) ([]string, error) {
	// This function makes sure no task strings returned by this are invalid
	excludedHostnames := getExcludedHostnames()

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	var taskStrs []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		strL := strings.Split(line, ",")
		URL := strings.TrimSpace(strL[0])
		parsedURL, err := url.Parse(URL)
		if err != nil {
			fmt.Println("Error extracting domain from TaskStrs", err)
			continue // TODO: check if this is still executed... should be handled before
		}
		if _, ok := excludedHostnames[parsedURL.Hostname()]; ok {
			continue
		}
		src := "" // program still works even if no sources are provided
		if len(strL) > 1 {
			src = strings.TrimSpace(strL[1])
		}

		taskStrs = append(taskStrs, URL+","+src)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return taskStrs, nil
}

func GroupTasks(taskStrs []string) [][][]string {
	//	1. Maintain a domain map
	domainMap := make(map[string][]string)
	for _, taskStr := range taskStrs {
		line := strings.TrimSpace(taskStr)
		strL := strings.Split(line, ",")
		URL := strings.TrimSpace(strL[0])
		parsedURL, _ := url.Parse(URL) // no err should occur here (filtered by prev func)
		domainMap[parsedURL.Hostname()] = append(domainMap[parsedURL.Hostname()], taskStr)
	}
	//	2. Subgroups
	var subGroups [][][]string
	var tempGroups [][]string
	var groupLen int
	for _, taskStrL := range domainMap {
		listLen := len(taskStrL)
		if groupLen+listLen > GlobalConfig.WorkerStress {
			subGroups = append(subGroups, tempGroups)
			tempGroups = [][]string{}
			groupLen = 0
		}
		tempGroups = append(tempGroups, taskStrL)
		groupLen += listLen
	}
	if len(tempGroups) > 0 {
		subGroups = append(subGroups, tempGroups)
	}

	return subGroups
}

func GetRandomIndex(max int) int {
	var n uint64
	_ = binary.Read(rand.Reader, binary.BigEndian, &n)
	return int(n % uint64(max))
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
		return DstChangePrint{
			Scheme: false, Hostname: false, Path: false, Query: false,
		}
	}
	oriParsed, err := url.Parse(ori)
	if err != nil {
		fmt.Println("do something") //TODO: fix this
		return DstChangePrint{
			Scheme: false, Hostname: false, Path: false, Query: false,
		}
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
		SourceURL: task.Source,
		Domain:    task.Hostname,
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
			dst := taskPrint.Retry.RedirectChain[len(taskPrint.Retry.RedirectChain)-1]
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
