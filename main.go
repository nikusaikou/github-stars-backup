package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"

	yaml "gopkg.in/yaml.v2"
)

type RepoOwner struct {
	Name string `json:"login"`
	URL  string `json:"html_url"`
}
type RepoData struct {
	Name        string    `json:"name"`
	URL         string    `json:"html_url"`
	Description string    `json:"description"`
	Language    string    `json:"language"`
	Topics      []string  `json:"topics"`
	Owner       RepoOwner `json:"owner"`
}

type RepoYAML struct {
	Name        string `yaml:"repo name"`
	URL         string `yaml:"url"`
	Description string `yaml:"description"`
	Owner       string `yaml:"author"`
	OwnerURL    string `yaml:"author's github"`
	Language    string `yaml:"language"`
	Topics      string `yaml:"tags"`
}

func main() {
	var (
		username     string
		savePath     string
		maxRepo      string
		useGoroutine string
		allRepos     []RepoData
		err          error
	)
	flag.StringVar(&username, "u", "", "your github username")
	flag.StringVar(&savePath, "p", "", "your save path")
	flag.StringVar(&maxRepo, "m", "100", "the maxinum number of your starred repos")
	flag.StringVar(&useGoroutine, "g", "true", "use goroutine or not(true/false default: true)")
	flag.Parse()

	if username == "" {
		log.Println("please input your github username.")
		return
	}
	max, err := strconv.Atoi(maxRepo)
	if err != nil {
		log.Fatalf("change \"maxRepo\" type error: %v", err)
		return
	}
	if useGoroutine == "false" {
		allRepos, err = getStarredRepo(username, max)
	} else {
		allRepos, err = getReposGoroutine(username, max)
	}
	if err != nil {
		log.Fatalf("get repos error: %v", err)
		return
	}

	var repos []RepoYAML
	for _, v := range allRepos {
		repo := RepoYAML{
			Name:        v.Name,
			URL:         v.URL,
			Description: v.Description,
			Owner:       v.Owner.Name,
			OwnerURL:    v.Owner.URL,
			Language:    v.Language,
		}
		for k, t := range v.Topics {
			repo.Topics += t
			if k < len(v.Topics)-1 {
				repo.Topics += ", "
			}
		}
		repos = append(repos, repo)
	}
	yamlData, err := yaml.Marshal(repos)
	if err != nil {
		log.Fatalf("parse yaml error: %v", err)
	}
	var path string
	if savePath != "" {
		if _, err := os.Stat(savePath); errors.Is(err, os.ErrNotExist) {
			err := os.Mkdir(savePath, 0700)
			if err != nil {
				log.Fatalf("create dir error: %v", err)
			}
		}
		path = savePath + "/stars.yaml"
	} else {
		path = "stars.yaml"
	}
	err = ioutil.WriteFile(path, yamlData, 0644)
	if err != nil {
		log.Fatalf("write file error: %v", err)
		return
	}

	fmt.Printf("You successfully backed up %d starred repos\n", len(allRepos))
}

func getStarredRepo(username string, maxRepo int) ([]RepoData, error) {
	var (
		repoCnt  int
		pageIdx  = 1
		allRepos []RepoData
	)

	for {
		resp, err := http.Get(fmt.Sprintf("https://api.github.com/users/%s/starred?page=%d&per_page=%d", username, pageIdx, 30))
		if err != nil {
			return nil, fmt.Errorf("get stars error: %w", err)
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read body erorr: %w", err)
		}

		var jsonData []RepoData
		if err := json.Unmarshal(body, &jsonData); err != nil {
			return nil, fmt.Errorf("json unmarshal error: %w", err)
		}
		repoCnt += len(jsonData)
		allRepos = append(allRepos, jsonData...)
		if repoCnt%maxRepo == 0 || len(jsonData) == 0 {
			break
		}
		pageIdx++
	}
	return allRepos, nil
}

func getReposGoroutine(username string, maxRepo int) ([]RepoData, error) {
	var (
		repoCnt  int
		allRepos []RepoData
		ch       = make(chan struct{}, 5)
		gerrors  = make(chan error)
		wg       sync.WaitGroup
		mu       sync.Mutex
		done     = make(chan struct{})
		// pageIdx  = 1
	)

	for i := 1; i < maxRepo/30+2; i++ {
		ch <- struct{}{}
		wg.Add(1)
		go func(pageIdx int) {
			defer func() {
				wg.Done()
				<-ch
			}()
			resp, err := http.Get(fmt.Sprintf("https://api.github.com/users/%s/starred?page=%d&per_page=%d", username, pageIdx, 30))
			if err != nil {
				gerrors <- err
			}
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				gerrors <- err
			}
			var jsonData []RepoData
			if err := json.Unmarshal(body, &jsonData); err != nil {
				gerrors <- err
			}

			mu.Lock()
			repoCnt += len(jsonData)
			allRepos = append(allRepos, jsonData...)
			mu.Unlock()
		}(i)
	}
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case err := <-gerrors:
		return nil, err
	case <-done:
		break
	}

	return allRepos, nil
}
