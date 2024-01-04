package common

import (
	"bufio"
	"bytes"
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

// getExcludedHostnames reads all excluded domains ( judici.com )
//
//	refer to the email on Sept 27th
func ReadExcludedHostnames() map[string]struct{} {
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

// ReadTaskStrsFromInput reads all tasks from the input file
//
//	Each task: f"{url}, {source(wiki article)}" (sources are [optional])
//	This function makes sure that all returned tasks strings are valid.
//	Caller (main) is responsible for error handling
func ReadTaskStrsFromInput(filename string) ([]string, error) {
	excludedHostnames := ReadExcludedHostnames()

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
			continue
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

// GroupTasks group all input tasks into subgroups
//
//	No hostnames are handled by multiple workers, this ensures maximum politeness
//	All workers can handle >= 1 hostnames, but at most N tasks
//	This function only return [][][]string, caller (main) is responsible for creating workers
func GroupTasks(taskStrs []string) [][][]string {
	//	1. Maintain a hostname map (hostname -> list of taskStrs)
	domainMap := make(map[string]map[string][]string, 0)
	for _, taskStr := range taskStrs {
		line := strings.TrimSpace(taskStr)
		strL := strings.Split(line, ",")
		URL := strings.TrimSpace(strL[0])
		source := ""
		if len(strL) > 1 {
			source = strings.TrimSpace(strL[1])
		}
		parsedURL, _ := url.Parse(URL) // no err should occur here (filtered by prev func)
		if _, ok := domainMap[parsedURL.Hostname()]; !ok {
			domainMap[parsedURL.Hostname()] = make(map[string][]string, 0)
		}
		if _, ok := domainMap[parsedURL.Hostname()][URL]; !ok {
			domainMap[parsedURL.Hostname()][URL] = make([]string, 0)
		}
		domainMap[parsedURL.Hostname()][URL] = append(domainMap[parsedURL.Hostname()][URL], source)
	}
	//	2. Group hostnames together, all workers can handle >= 1 hostnames, but at most N tasks
	var subGroups [][][]string
	var tempGroups [][]string
	var groupLen int
	for _, taskStrM := range domainMap {
		listLen := len(taskStrM)
		if groupLen+listLen > GlobalConfig.WorkerStress {
			subGroups = append(subGroups, tempGroups)
			tempGroups = [][]string{}
			groupLen = 0
		}
		taskStrL := make([]string, len(taskStrM))
		idx := 0
		for k, v := range taskStrM {
			taskStrL[idx] = k + "," + strings.Join(v, " ")
			idx++
		} // convert from map to string, use map for prev dedup
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

// PrintResp is a helper for db logging, same as all other func PrintSomething
//
//	mongodb cannot dereference pointers, so all pointed values are dereferenced by this
func PrintResp(resp *http.Response) RespPrint {
	if resp == nil {
		return RespPrint{}
	}
	defer resp.Body.Close()

	storableResp := RespPrint{
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
	}

	if eTag, ok := resp.Header["Etag"]; ok {
		storableResp.ETag = strings.Trim(eTag[0], "\"")
	}

	var dynamicBuf bytes.Buffer
	chunkSize := 32 * 1024
	for {
		chunkBuf := make([]byte, chunkSize)

		n, err := io.ReadAtLeast(resp.Body, chunkBuf, chunkSize)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				dynamicBuf.Write(chunkBuf[:n]) // Write the last partial chunk
				break
			}
			fmt.Println(resp.Request.URL, "PrintResp err io.Reader")
			break
		}

		dynamicBuf.Write(chunkBuf[:n])

		if dynamicBuf.Len() >= GlobalConfig.ESelfTagBuffLen {
			break
		}
	}

	storableResp.ESelfTag = computeETag(dynamicBuf.Bytes())
	storableResp.Size = dynamicBuf.Len()

	return storableResp
}

func PrintErr(err error) string {
	if err == nil {
		return ""
	} else {
		return err.Error()
	}
}

func PrintDstChange(ori string, redirectChain []string) DstChangePrint {
	var dstURL string
	if len(redirectChain) > 0 {
		dst := redirectChain[len(redirectChain)-1]
		dstL := strings.Split(dst, " ")
		dstURL = dstL[len(dstL)-1]
	} else {
		return DstChangePrint{
			Scheme: false, Hostname: false, Path: false, Query: false,
		}
	}

	oriParsed, err := url.Parse(ori)
	if err != nil {
		fmt.Printf("parsing ori url %s\n", ori) //TODO: fix this
		return DstChangePrint{
			Scheme: false, Hostname: false, Path: false, Query: false,
		}
	}
	dstParsed, err := url.Parse(dstURL)
	if err != nil {
		fmt.Printf("parsing dst url %s\n", dstURL) //TODO: fix this
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

//func PrintTask(task TaskPrint) TaskPrint {
//	return task
//}
