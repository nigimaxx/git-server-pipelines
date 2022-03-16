package main

// <bitbar.title>GitLab Pipelines Status</bitbar.title>
// <bitbar.version>v0.3</bitbar.version>
// <bitbar.author>Niklas Mack</bitbar.author>
// <bitbar.author.github>nigimaxx</bitbar.author.github>
// <bitbar.desc>Get the Status of your GitLab Pipelines</bitbar.desc>
// <bitbar.image></bitbar.image>
// <bitbar.dependencies>golang</bitbar.dependencies>
// <bitbar.abouturl></bitbar.abouturl>

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/google/go-github/v32/github"
	"github.com/xanzy/go-gitlab"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v2"
)

// Config is the struct that represents the yaml config
type Config struct {
	DaysUntilInactive int          `yaml:"daysUntilInactive"`
	Servers           ServerConfig `yaml:"servers"`
}

// ServerConfig is the config for the github and gitlab servers
type ServerConfig struct {
	Github []GithubServerConfig `yaml:"github"`
	Gitlab []GitlabServerConfig `yaml:"gitlab"`
}

// GithubServerConfig is the config for the github servers
type GithubServerConfig struct {
	Name         string   `yaml:"name"`
	Token        string   `yaml:"token"`
	Repositories []string `yaml:"repositories"`
}

// GitlabServerConfig is the config for the gitlab servers
type GitlabServerConfig struct {
	Name     string   `yaml:"name"`
	BaseURL  string   `yaml:"baseURL"`
	Token    string   `yaml:"token"`
	Projects []string `yaml:"projects"`
}

// ActiveProject is project or repository which had a recent build activity
type ActiveProject struct {
	Name       string
	URL        string
	Status     string
	ServerName string
	UpdatedAt  time.Time
}

// GithubServer is a configured github client, its name and a context
type GithubServer struct {
	Client *github.Client
	Name   string
	Ctx    context.Context
}

// GitlabServer is a configured gitlab client and its name
type GitlabServer struct {
	Client *gitlab.Client
	Name   string
}

var gitlabIcons = map[string]string{
	"created":  "🟣",
	"pending":  "🟡",
	"running":  "🔵",
	"success":  "🟢",
	"canceled": "🟠",
	"failed":   "🔴",
	"skipped":  "⚪️",
	"manual":   "⚫️",
}

var githubIcons = map[string]string{
	"queued":          "🟡",
	"in_progress":     "🔵",
	"success":         "🟢",
	"cancelled":       "🟠",
	"failure":         "🔴",
	"skipped":         "⚪️",
	"action_required": "⚫️",
	"timed_out":       "🟤",
}

var config *Config

func main() {
	var err error
	config, err = readConfig()
	if err != nil {
		log.Fatal(err)
	}

	errCh := make(chan error)
	wgDone := make(chan bool)
	var (
		wg             sync.WaitGroup
		activeProjects []ActiveProject
	)

	for _, s := range config.Servers.Gitlab {
		git, err := gitlab.NewClient(s.Token, gitlab.WithBaseURL(s.BaseURL))
		if err != nil {
			log.Fatal(err)
		}

		server := GitlabServer{Client: git, Name: s.Name}

		projects, err := server.GetGitlabProjects()
		if err != nil {
			log.Fatal(err)
		}

		for _, project := range projects {
			name := project.PathWithNamespace

			if isInList(name, s.Projects) {
				wg.Add(1)

				go func(id int, name string) {
					defer wg.Done()

					pipeline, err := server.GetActivePipeline(id, name)
					if err != nil {
						errCh <- err
						return
					}

					if pipeline != nil {
						activeProjects = append(activeProjects, *pipeline)
					}
				}(project.ID, name)
			}
		}
	}

	for _, s := range config.Servers.Github {
		ctx := context.Background()

		tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: s.Token})
		tokenClient := oauth2.NewClient(ctx, tokenSource)

		githubClient := github.NewClient(tokenClient)
		server := GithubServer{Client: githubClient, Name: s.Name, Ctx: ctx}

		repos, err := server.GetGithubRepos()
		if err != nil {
			log.Fatal(err)
		}

		for _, r := range repos {
			var (
				owner = *r.Owner.Login
				name  = *r.Name
			)

			if isInList(fmt.Sprintf("%s/%s", owner, name), s.Repositories) {
				wg.Add(1)

				go func(owner, name string) {
					defer wg.Done()

					workflow, err := server.GetActiveWorkflow(owner, name)
					if err != nil {
						errCh <- err
						return
					}

					if workflow != nil {
						activeProjects = append(activeProjects, *workflow)
					}
				}(owner, name)
			}
		}
	}

	go func() {
		wg.Wait()
		close(wgDone)
	}()

	select {
	case <-wgDone:
	case err := <-errCh:
		close(errCh)
		log.Fatal(err)
	}

	sort.Slice(activeProjects, func(i, j int) bool {
		return activeProjects[i].UpdatedAt.After(activeProjects[j].UpdatedAt)
	})

	fmt.Println(overAllStatus(activeProjects))

	fmt.Println("---")

	for _, p := range activeProjects {
		fmt.Printf("%s %s (%s) | href=%s\n", p.Status, p.Name, p.ServerName, p.URL)
	}

	fmt.Println("---")
	fmt.Println("Refresh | refresh=true")
}

// GetActiveWorkflow gets the latest workflow of a github repository
func (server *GithubServer) GetActiveWorkflow(owner, name string) (*ActiveProject, error) {
	runs, _, err := server.Client.Actions.ListRepositoryWorkflowRuns(
		server.Ctx,
		owner,
		name,
		&github.ListWorkflowRunsOptions{ListOptions: github.ListOptions{PerPage: 1, Page: 1}},
	)
	if err != nil {
		return nil, err
	}

	if len(runs.WorkflowRuns) > 0 {
		run := runs.WorkflowRuns[0]
		status := *run.Status
		if status == "completed" {
			status = *run.Conclusion
		}

		if time.Now().Sub(run.UpdatedAt.Time) < time.Duration(config.DaysUntilInactive)*24*time.Hour {
			return &ActiveProject{
				Name:       fmt.Sprintf("%s/%s", owner, name),
				URL:        *run.HTMLURL,
				Status:     githubIcons[status],
				ServerName: server.Name,
				UpdatedAt:  run.UpdatedAt.Time,
			}, nil
		}
	}

	return nil, nil
}

// GetGithubRepos gets all github repositoris that a user can access
func (server *GithubServer) GetGithubRepos() ([]*github.Repository, error) {
	options := &github.RepositoryListOptions{ListOptions: github.ListOptions{PerPage: 100}}

	var allRepos []*github.Repository

	for {
		repos, response, err := server.Client.Repositories.List(server.Ctx, "", options)
		if err != nil {
			return nil, err
		}

		allRepos = append(allRepos, repos...)

		if response.NextPage == 0 {
			break
		}

		options.Page = response.NextPage
	}

	return allRepos, nil
}

// GetActivePipeline gets the latest pipeline build of a gitlab project
func (server *GitlabServer) GetActivePipeline(id int, name string) (*ActiveProject, error) {
	pipelines, _, err := server.Client.Pipelines.ListProjectPipelines(id, &gitlab.ListProjectPipelinesOptions{ListOptions: gitlab.ListOptions{PerPage: 1}})
	if err != nil {
		return nil, err
	}

	if len(pipelines) > 0 {
		pipeline := pipelines[0]

		if time.Now().Sub(*pipeline.UpdatedAt) < time.Duration(config.DaysUntilInactive)*24*time.Hour {
			return &ActiveProject{
				Name:       name,
				URL:        pipeline.WebURL,
				Status:     gitlabIcons[pipeline.Status],
				ServerName: server.Name,
				UpdatedAt:  *pipeline.UpdatedAt,
			}, nil
		}
	}

	return nil, nil
}

// GetGitlabProjects gets all gitlab projects that a user can access
func (server *GitlabServer) GetGitlabProjects() ([]*gitlab.Project, error) {
	var allProjects []*gitlab.Project
	page := 1

	for page > 0 {
		projects, response, err := server.Client.Projects.ListProjects(&gitlab.ListProjectsOptions{ListOptions: gitlab.ListOptions{Page: page, PerPage: 100}, Membership: gitlab.Bool(true)})
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

	data, err := ioutil.ReadFile(path.Join(home, "projects", "private", "bitbar-config", "gitlab-config.yaml"))
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
	status := "🟢"

	if len(projects) == 0 {
		return "⚪️"
	}

	for _, p := range projects {
		if iconWeight(p.Status) > iconWeight(status) {
			status = p.Status
		}
	}

	return status
}

func iconWeight(icon string) int {
	switch icon {
	case "🟢":
		return 1
	case "🔴":
		return 2
	case "🔵":
		return 3
	default:
		return 0
	}
}
