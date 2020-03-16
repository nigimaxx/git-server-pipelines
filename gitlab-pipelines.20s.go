package main

// <bitbar.title>GitLab Pipelines Status</bitbar.title>
// <bitbar.version>v0.1</bitbar.version>
// <bitbar.author>Niklas Mack</bitbar.author>
// <bitbar.author.github>nigimaxx</bitbar.author.github>
// <bitbar.desc>Get the Status of your GitLab Pipelines</bitbar.desc>
// <bitbar.image></bitbar.image>
// <bitbar.dependencies>golang</bitbar.dependencies>
// <bitbar.abouturl></bitbar.abouturl>

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"sort"
	"strconv"
	"time"

	"github.com/xanzy/go-gitlab"
	"gopkg.in/yaml.v2"
)

type Config struct {
	BaseURL  string   `yaml:"baseURL"`
	Token    string   `yaml:"token"`
	Projects []string `yaml:"projects"`
}

type ActiveProject struct {
	Name       string
	URL        string
	PipelineID int
	Status     string
}

var git *gitlab.Client
var icons = map[string]string{
	"success":  "🟢",
	"failed":   "🔴",
	"canceled": "🟠",
	"skipped":  "⚪️",
	"running":  "🔵",
}

func main() {
	config, err := readConfig()
	if err != nil {
		log.Fatal(err)
	}

	git = gitlab.NewClient(nil, config.Token)
	git.SetBaseURL(config.BaseURL)

	projects, err := getAllProjects()
	if err != nil {
		log.Fatal(err)
	}

	var activeProjects []ActiveProject

	for _, project := range projects {
		name := project.PathWithNamespace

		if isInList(name, config.Projects) {
			pipelines, _, _ := git.Pipelines.ListProjectPipelines(project.ID, &gitlab.ListProjectPipelinesOptions{ListOptions: gitlab.ListOptions{PerPage: 1}})

			for _, pipeline := range pipelines {
				status := pipeline.Status
				if status == "pending" {
					status = "running"
				}

				if time.Now().Sub(*pipeline.UpdatedAt) < 7*24*time.Hour {
					activeProjects = append(activeProjects, ActiveProject{name, project.WebURL, pipeline.ID, status})
				}
			}
		}
	}

	sort.Slice(activeProjects, func(i, j int) bool {
		return activeProjects[i].PipelineID > activeProjects[j].PipelineID

	})

	fmt.Println(icons[overAllStatus(activeProjects)])

	fmt.Println("---")

	for _, p := range activeProjects {
		fmt.Printf("%s %s | href=%s/pipelines\n", icons[p.Status], p.Name, p.URL)
	}
}

func getAllProjects() ([]*gitlab.Project, error) {
	var allProjects []*gitlab.Project
	page := 1

	for page > 0 {
		projects, response, err := git.Projects.ListProjects(&gitlab.ListProjectsOptions{ListOptions: gitlab.ListOptions{Page: page, PerPage: 100}})
		if err != nil {
			return nil, err
		}

		allProjects = append(allProjects, projects...)

		page, err = strconv.Atoi(response.Header.Get("X-Next-Page"))
		if err != nil {
			page = 0
		}
	}

	return allProjects, nil
}

func readConfig() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadFile(path.Join(home, "bitbar", "gitlab-config.yaml"))
	if err != nil {
		return nil, err
	}

	config := Config{}
	if err := yaml.Unmarshal([]byte(data), &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func isInList(s string, list []string) bool {
	for _, i := range list {
		if i == s {
			return true
		}
	}
	return false
}

func overAllStatus(projects []ActiveProject) string {
	status := "success"
	for _, p := range projects {
		if p.Status == "running" {
			status = p.Status
		} else if p.Status == "failed" && status != "running" {
			status = p.Status
		}
	}

	return status
}
