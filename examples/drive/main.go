package main

import (
	"context"
	"fmt"
	"log"

	"github.com/yuya-takeyama/googleoauth2callback"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

func main() {
	callback := googleoauth2callback.New(
		googleoauth2callback.WithCredentialsPath("credentials.json"),
		googleoauth2callback.WithRedirectURL("http://localhost:4567/callback"),
		googleoauth2callback.WithScopes([]string{drive.DriveReadonlyScope}),
	)

	client, err := callback.GetClient()
	if err != nil {
		log.Fatal(err)
	}

	srv, err := drive.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		log.Fatal(err)
	}

	files, err := srv.Files.List().
		Fields("nextPageToken, files(id, name)").
		PageSize(10).
		Do()
	if err != nil {
		log.Fatal(err)
	}

	if len(files.Files) == 0 {
		fmt.Println("No files found.")
	} else {
		fmt.Println("Files:")
		for _, file := range files.Files {
			fmt.Printf("%s (%s)\n", file.Name, file.Id)
		}
	}
}
