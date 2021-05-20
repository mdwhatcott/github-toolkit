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

func main() {
	var root string

	flag.StringVar(&root, "root", os.Getenv("CODEPATH"), "The $GOPATH-style root directory.")
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
		go worker(x, root, work, wait)
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
			work <- repo.GetFullName()
		}

		_ = response.Body.Close()
		page = response.NextPage
	}

	close(work)
	wait.Wait()
}

func worker(id int, root string, input chan string, waiter *sync.WaitGroup) {
	defer waiter.Done()

	log.Println("[INFO] starting worker:", id)

	for name := range input {
		nameParts := strings.Split(name, "/")
		source := fmt.Sprintf("git@github.com:%s.git", name)
		target := filepath.Join(root, "src", "github.com", nameParts[0], nameParts[1])
		_, err := os.Stat(target)
		if os.IsNotExist(err) {
			log.Printf("[INFO] worker %d: git clone %s %s", id, source, target)
			command := exec.Command("git", "clone", source, target)
			command.Stderr = command.Stdout
			err := command.Run()
			if err != nil {
				log.Printf("[WARN] worker %d: clone err for %s: %s", id, name, err)
			}
		}
	}
}
