package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/yabslabs/provider/util"

	"github.com/yabslabs/provider/storage"
)

const (
	accessTokenKey = "accessToken"
	usernameKey    = "username"
	acceptType     = "application/vnd.github.wyandotte-preview+json"
	urlKey         = "url"
)

var (
	accessToken, username string
)

func flags() (string, string, string) {
	var url string
	flag.StringVar(&accessToken, accessTokenKey, "", "access token of github account")
	flag.StringVar(&username, usernameKey, "", "username of github account")
	flag.StringVar(&url, urlKey, "", "url to github repo")
	flag.Parse()

	if accessToken == "" || username == "" || url == "" {
		log.Fatal("provide username and password of backup-user and url to repo")
	}

	return accessToken, username, url
}

//main exports yabs
func main() {
	accessToken, username, url := flags()
	DoExport(accessToken, username, url)
}

func DoExport(accessToken, username, url string) {
	client := &http.Client{}

	repos, err := getRepos(client, url)
	if err != nil {
		log.Fatalf("repo-check failed: %v", err)
	}
	migrationID, err := startBackup(client, url, repos)
	if err != nil {
		log.Fatalf("create backup failed: %v", err)
	}
	if err = awaitBackup(client, url, migrationID); err != nil {
		log.Fatalf("await backup failed: %v", err)
	}

	storage := storage.NewStorage()
	if err = downloadExport(client, url, migrationID, storage); err != nil {
		log.Fatalf("download backup failed: %v", err)
	}
}

func awaitBackup(client *http.Client, url string, migrationID int) error {
	req, err := util.CreateGETRequest(fmt.Sprintf("%v/migrations/%v", url, migrationID), nil, githubRequest)
	if err != nil {
		return err
	}
	for {
		time.Sleep(1 * time.Second)
		mig := Migrations{}
		if err = util.DoRequestWithUnmarshal(client, req, &mig); err != nil {
			return err
		}
		if strings.ToLower(mig.State) == "failed" {
			return fmt.Errorf("backup failed")
		}
		if strings.ToLower(mig.State) == "exported" {
			return nil
		}
	}
}

func getRepos(client *http.Client, url string) (Repos, error) {
	req, err := util.CreateGETRequest(fmt.Sprintf("%v/repos", url), nil, githubRequest)
	if err != nil {
		return nil, err
	}
	repos := make(Repos, 0)
	if err = util.DoRequestWithUnmarshal(client, req, &repos); err != nil {
		return nil, err
	}

	return repos, nil
}

func startBackup(client *http.Client, url string, repos Repos) (int, error) {
	repoList := make([]string, 0, len(repos))
	for _, repo := range repos {
		repoList = append(repoList, repo.Name)
	}
	repositories := &Repositories{Repositories: repoList}
	req, err := util.CreatePOSTRequest(fmt.Sprintf("%v/migrations", url), repositories, githubRequest)
	if err != nil {
		return 0, err
	}
	migrations := &Migrations{}
	err = util.DoRequestWithUnmarshal(client, req, migrations)
	return migrations.ID, err
}

func downloadExport(client *http.Client, url string, migrationID int, storage storage.Storage) error {
	req, err := util.CreateGETRequest(fmt.Sprintf("%v/migrations/%v/archive", url, migrationID), nil, githubRequest)
	if err != nil {
		return err
	}
	body, err := util.DoRequest(client, req)
	if err != nil {
		return err
	}
	return storage.Save(os.TempDir()+string(os.PathSeparator)+"github", "github.bak", body)
}
