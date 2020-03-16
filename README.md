# GitLab Pipelines BitBar Plugin

## Requirements

- Golang
- Go Modules

## Install

```
go build -o gitlab-pipelines.20s.cgo gitlab-pipelines.20s.go
```

Move `gitlab-pipelines.20s.cgo` to your BitBar Plugins Folder

## Configuration

Put a `yaml` file in `~/bitbar/gitlab-config.yaml`

```yaml
token: { access-token }
baseURL: { api-base eg. https://gitlab.example.com/api/v4 }
projects:
  - { path_with_namespace of project }
  - test-namespace/test-project
```
