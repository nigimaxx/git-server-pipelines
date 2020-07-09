package main

// <bitbar.title>GitLab Pipelines Status</bitbar.title>
// <bitbar.version>v0.2</bitbar.version>
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
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/xanzy/go-gitlab"
	"gopkg.in/yaml.v2"
)

type Config struct {
	DaysUntilInactive int            `yaml:"daysUntilInactive"`
	Servers           []ServerConfig `yaml:"servers"`
}

type ServerConfig struct {
	Name     string   `yaml:"name"`
	BaseURL  string   `yaml:"baseURL"`
	Token    string   `yaml:"token"`
	Projects []string `yaml:"projects"`
}

type ActiveProject struct {
	Name       string
	URL        string
	Status     string
	ServerName string
	UpdatedAt  time.Time
}

var icons = map[string]string{
	"created":  "🟣",
	"pending":  "🟡",
	"running":  "🔵",
	"success":  "🟢",
	"canceled": "🟠",
	"failed":   "🔴",
	"skipped":  "⚪️",
	"manual":   "⚫️",
}

func main() {
	config, err := readConfig()
	if err != nil {
		log.Fatal(err)
	}

	var activeProjects []ActiveProject

	for _, server := range config.Servers {
		git := gitlab.NewClient(nil, server.Token)
		git.SetBaseURL(server.BaseURL)

		projects, err := getAllProjects(git)
		if err != nil {
			log.Fatal(err)
		}

		for _, project := range projects {
			name := project.PathWithNamespace

			if isInList(name, server.Projects) {
				pipelines, _, _ := git.Pipelines.ListProjectPipelines(project.ID, &gitlab.ListProjectPipelinesOptions{ListOptions: gitlab.ListOptions{PerPage: 1}})

				for _, pipeline := range pipelines {
					status := pipeline.Status
					if status == "pending" {
						status = "running"
					}

					if time.Now().Sub(*pipeline.UpdatedAt) < time.Duration(config.DaysUntilInactive)*24*time.Hour {
						activeProjects = append(activeProjects, ActiveProject{name, pipeline.WebURL, status, server.Name, *pipeline.UpdatedAt})
					}
				}
			}
		}
	}

	sort.Slice(activeProjects, func(i, j int) bool {
		return activeProjects[i].UpdatedAt.After(activeProjects[j].UpdatedAt)
	})

	fmt.Println(icons[overAllStatus(activeProjects)])

	fmt.Println("---")

	for _, p := range activeProjects {
		fmt.Printf("%s %s (%s) | href=%s\n", icons[p.Status], p.Name, p.ServerName, p.URL)
	}
}

func getAllProjects(git *gitlab.Client) ([]*gitlab.Project, error) {
	var allProjects []*gitlab.Project
	page := 1

	for page > 0 {
		projects, response, err := git.Projects.ListProjects(&gitlab.ListProjectsOptions{ListOptions: gitlab.ListOptions{Page: page, PerPage: 100}, Membership: gitlab.Bool(true)})
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
		if matched, _ := filepath.Match(i, s); matched {
			return true
		}
	}
	return false
}

func overAllStatus(projects []ActiveProject) string {
	status := "success"

	if len(projects) == 0 {
		return "skipped"
	}

	for _, p := range projects {
		if p.Status == "running" {
			status = p.Status
		} else if p.Status == "failed" && status != "running" {
			status = p.Status
		}
	}

	return status
}
