package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// todo: dry run
// todo: docs
// todo: comments
// todo: GitHub action

//todo: add comment
var (
	BuildTime string
	BuildSHA  string
	Version   string
)

type pullRequest struct {
	Number   uint64    `json:"number"`
	ClosedAt time.Time `json:"closed_at"`

	Head struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
}

type branch struct {
	Name string `json:"name"`

	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

type executor struct {
	client *http.Client
	token  string
	http   bool
	dry    bool
}

//todo: add comment
func NewExecutor(token string) *executor {
	ex := executor{
		client: &http.Client{},
		token:  token,
	}

	return &ex
}

func (ex *executor) makeRequest(method string, url string) (res *http.Response, err error) {
	protocol := "https"
	if ex.http {
		protocol = "http"
	}

	req, err := http.NewRequest(method, protocol+"://api.github.com/"+url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "token "+ex.token)
	res, err = ex.client.Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (ex *executor) listClosedPullRequests(user string, repo string, days int) []pullRequest {
	pullRequests := make([]pullRequest, 0, 1)
	now := time.Now()
	maxAgeHours := float64(days * 24)

	if maxAgeHours < 24.0 {
		maxAgeHours = 24.0
	}

	for page := 1; ; page++ {
		res, err := ex.makeRequest("GET", "repos/"+user+"/"+repo+"/pulls?state=closed&sort=updated&direction=desc&per_page=100&page="+strconv.Itoa(page))

		if err != nil {
			log.Fatalln("Failed to get pull requests", err)
		}

		d := json.NewDecoder(res.Body)
		var prs struct {
			PullRequests []pullRequest
		}
		err = d.Decode(&prs.PullRequests)

		if err != nil {
			log.Fatalln("Failed to parse pull requests", err)
		}

		pullRequests = append(pullRequests, prs.PullRequests...)

		if len(prs.PullRequests) == 0 || len(prs.PullRequests) < 100 {
			break
		}

		lastPullRequest := prs.PullRequests[len(prs.PullRequests)-1]
		lastPullRequestAge := now.Sub(lastPullRequest.ClosedAt).Hours()
		//todo: only add pull requests < maxAgeHours?
		if lastPullRequestAge >= maxAgeHours {
			break
		}
	}

	return pullRequests
}

func (ex *executor) listUnprotectedBranches(user string, repo string) []branch {
	branches := make([]branch, 0, 1)

	for page := 1; ; page++ {
		res, err := ex.makeRequest("GET", "repos/"+user+"/"+repo+"/branches?protected=false&per_page=100&page="+strconv.Itoa(page))

		if err != nil {
			log.Fatalln("Failed to get branches", err)
		}

		d := json.NewDecoder(res.Body)
		var bs struct {
			Branches []branch
		}
		err = d.Decode(&bs.Branches)

		if err != nil {
			log.Fatalln("Failed to parse branches", err)
		}

		branches = append(branches, bs.Branches...)

		if len(bs.Branches) == 0 || len(bs.Branches) < 100 {
			break
		}
	}

	return branches
}

func (ex *executor) deleteBranches(user string, repo string, branches []string) {
	for _, branch := range branches {
		_, err := ex.makeRequest("DELETE", "repos/"+user+"/"+repo+"/git/refs/heads/"+branch)

		if err != nil {
			log.Fatalln("Failed to delete branch", branch, err)
		}

		log.Println("Deleted branch", branch)
	}
}

func getStaleBranches(branches []branch, pullRequests []pullRequest) []string {
	branchesByName := make(map[string]branch)
	staleBranches := make([]string, 0, 1)

	for _, b := range branches {
		branchesByName[b.Name] = b
	}

	for _, pr := range pullRequests {
		staleBranch, branchExists := branchesByName[pr.Head.Ref]
		if branchExists && staleBranch.Commit.SHA == pr.Head.SHA {
			staleBranches = append(staleBranches, pr.Head.Ref)
		}
	}

	return staleBranches
}

func getDays(days string) int {
	if days != "" {
		d, err := strconv.Atoi(days)
		if err == nil {
			return d
		}
	}
	return 1
}

//todo: add comment
func Run(user string, repo string, days int, ex executor) error {
	var err error

	if user == "" {
		err = errors.New("missing user")
	} else if repo == "" {
		err = errors.New("missing repo")
	} else if days < 1 {
		err = errors.New("invalid value for days (" + strconv.Itoa(days) + ")")
	}

	if err != nil {
		return err
	}

	closedPullRequests := ex.listClosedPullRequests(user, repo, days)
	unprotectedBranches := ex.listUnprotectedBranches(user, repo)
	staleBranches := getStaleBranches(unprotectedBranches, closedPullRequests)
	ex.deleteBranches(user, repo, staleBranches)
	return err
}

func main() {
	fmt.Println("github-fresh v" + Version + " " + BuildTime + " " + BuildSHA)
	var token = flag.String("token", os.Getenv("GITHUB_FRESH_TOKEN"), "GitHub API token")
	var user = flag.String("user", os.Getenv("GITHUB_FRESH_USER"), "GitHub user")
	var repo = flag.String("repo", os.Getenv("GITHUB_FRESH_REPO"), "GitHub repo")
	var days = flag.Int("days", getDays(os.Getenv("GITHUB_FRESH_DAYS")), "Max age in days of checked pull requests")
	flag.Parse()
	ex := NewExecutor(*token)
	err := Run(*user, *repo, *days, *ex)
	if err != nil {
		log.Fatalln(err)
	}
}
