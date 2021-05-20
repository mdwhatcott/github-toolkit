package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/go-github/v35/github"
	"golang.org/x/oauth2"
)

var Version = "dev"

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("VERSION:", Version)

	var root string
	var user string
	var dryRun bool

	flag.StringVar(&root, "root", os.Getenv("CODEPATH"), "The $GOPATH-style root directory.")
	flag.StringVar(&user, "user", os.Getenv("GITHUB_USER"), "The authenticated user--will ignore repos not on this user's account.")
	flag.BoolVar(&dryRun, "dry-run", false, "When set, list repos to be cloned, but don't actually clone.")
	flag.Parse()

	_, err := os.Stat(root)
	if err == os.ErrNotExist {
		log.Fatalf("supplied root [%s] is not valid: %s", root, err)
	}

	token := os.Getenv("GITHUB_PERSONAL_ACCESS_TOKEN")
	if len(token) == 0 {
		log.Fatalln("no github access token in env:GITHUB_PERSONAL_ACCESS_TOKEN")
	}
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	authClient := oauth2.NewClient(context.Background(), tokenSource)
	client := github.NewClient(authClient)

	work := make(chan string)
	wait := new(sync.WaitGroup)
	for x := 0; x < 10; x++ {
		wait.Add(1)
		go worker(x, root, work, wait, dryRun)
	}

	for page := 1; page > 0; {
		list, response, err := client.Repositories.List(context.Background(),
			"", // when blank, query for 'authenticated user'.
			&github.RepositoryListOptions{ListOptions: github.ListOptions{Page: page}},
		)
		if err != nil {
			log.Fatalln(err)
		}

		for _, repo := range list {
			name := repo.GetFullName()
			if strings.HasPrefix(name, user+"/") {
				work <- name
			}
		}

		_ = response.Body.Close()
		page = response.NextPage
	}

	close(work)
	wait.Wait()
}

func worker(id int, root string, input chan string, waiter *sync.WaitGroup, dryRun bool) {
	defer waiter.Done()

	log.Println("[INFO] starting worker:", id)
	if dryRun {
		log.Printf("[INFO] worker %d operating in dry-run mode.", id)
	}

	for name := range input {
		nameParts := strings.Split(name, "/")
		source := fmt.Sprintf("git@github.com:%s.git", name)
		target := filepath.Join(root, "src", "github.com", nameParts[0], nameParts[1])
		_, err := os.Stat(target)
		if os.IsNotExist(err) {
			log.Printf("[INFO] worker %d: git clone %s %s", id, source, target)
			if dryRun {
				continue
			}
			command := exec.Command("git", "clone", source, target)
			command.Stderr = command.Stdout
			err := command.Run()
			if err != nil {
				log.Printf("[WARN] worker %d: clone err for %s: %s", id, name, err)
			}
		}
	}
}
