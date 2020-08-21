# GitLab Pipelines BitBar Plugin

## Requirements

- [BitBar](https://getbitbar.com/)
- Golang
- Go Modules

## Install

```
go build -o git-server-pipelines.20s.cgo git-server-pipelines.20s.go
```

Move `git-server-pipelines.20s.cgo` to your BitBar Plugins Folder.

## Configuration

Create a `~/bitbar/gitlab-config.yaml`.

```yaml
daysUntilInactive: { only projects with pipeline builds fewer days ago are listed }
servers:
  github:
    - name: { your own name for this github server }
      token: { personal access token }
      repositories:
        - { owner/name of a repository or pattern }
        - test-owner/test-repository
        - other-test-owner/*
        - '*/*'
        #  a */* can get you into trouble with github's ratelimiting
  gitlab:
    - name: { your own name for this gitlab server }
      baseURL: { api-base e.g. https://gitlab.example.com/api/v4 }
      token: { access-token with read_api scope }
      projects:
        - { path_with_namespace of a project or pattern }
        - test-namespace/test-project
        - other-test-namespace/*
        - '*/*'
```

Pattern matching with [filepath.Match](https://golang.org/pkg/path/filepath/#Match)
