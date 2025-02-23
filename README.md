# googleoauth2callback

A Go package that simplifies Google OAuth2 authentication flow with a local callback server.

This package is useful when you need to:

- Authenticate with Google OAuth2 in CLI applications
- Handle OAuth2 callback flow automatically
- Store and reuse OAuth2 tokens

## Usage

### Setup Google OAuth2 Credentials

1. **Access Google Cloud Console**

   - Go to [Google Cloud Console](https://console.cloud.google.com/)
   - Select or create a new project

2. **Create OAuth2 Credentials**

   - Navigate to "APIs & Services" → "Credentials"
   - Click "Create Credentials" → "OAuth client ID"
   - Choose "Web application" as the application type
   - Enter a name and click "Create"

3. **Download and Save Credentials**

   - Click the download button (JSON) for your created credentials
   - Save the downloaded file as `credentials.json` in your project root directory
   - Add both `credentials.json` and `token.json` to your `.gitignore` file to exclude them from version control

### Example

```go
package main

import (
	"log"

	"github.com/yuya-takeyama/googleoauth2callback"
	"google.golang.org/api/drive/v3"
)

func main() {
	callback := googleoauth2callback.New(
		googleoauth2callback.WithCredentialsFile("credentials.json"),
		googleoauth2callback.WithRedirectURL("http://localhost:4567/callback"),
		googleoauth2callback.WithScopes([]string{"https://www.googleapis.com/auth/drive.readonly"}),
	)

	client, err := callback.GetClient()
	if err != nil {
		log.Fatal(err)
	}

	srv, err := drive.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		log.Fatal(err)
	}

    // ...
}
```
