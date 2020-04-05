# GitLab Pipelines BitBar Plugin

## Requirements

- [BitBar](https://getbitbar.com/)
- Golang
- Go Modules

## Install

```
go build -o gitlab-pipelines.20s.cgo gitlab-pipelines.20s.go
```

Move `gitlab-pipelines.20s.cgo` to your BitBar Plugins Folder.

## Configuration

Create a `~/bitbar/gitlab-config.yaml`.

```yaml
token: { access-token }
baseURL: { api-base e.g. https://gitlab.example.com/api/v4 }
projects:
  - { path_with_namespace of project or pattern }
  - test-namespace/test-project
  - other-test-namespace/*
  - '*/*'
```

Pattern matching with [filepath.Match](https://golang.org/pkg/path/filepath/#Match)
