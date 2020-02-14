package main

import (
	"log"
	"os"
	"strings"

	"context"

	"github.com/digitalocean/godo"
	"golang.org/x/oauth2"

	"github.com/minio/minio-go"
)

type TokenSource struct {
	AccessToken string
}

func (t *TokenSource) Token() (*oauth2.Token, error) {
	token := &oauth2.Token{
		AccessToken: t.AccessToken,
	}
	return token, nil
}

func main() {
	accessKey := os.Getenv("SPACES_KEY")
	secKey := os.Getenv("SPACES_SECRET")
	endpoint := "sfo2.digitaloceanspaces.com"
	spaceName := "crime-map" // Space names must be globally unique
	ssl := true

	// Initiate a client using DigitalOcean Spaces.
	client, err := minio.New(endpoint, accessKey, secKey, ssl)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Setting bucket to read only public")
	policy := `{"Version": "2012-10-17","Statement": [{"Action": ["s3:GetObject"],"Effect": "Allow","Principal": {"AWS": ["*"]},"Resource": ["arn:aws:s3:::crime-map/*"],"Sid": ""}]}`
	err = client.SetBucketPolicy(spaceName, policy)
	if err.Error() != "200 OK" {
		log.Fatalln("ERROR: " + err.Error())
		return
	}

	objectName := "bundle.js"
	filePath := "./dist/bundle.js"
	contentType := "application/javascript"
	n, err := client.FPutObject(spaceName, objectName, filePath, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("Successfully uploaded %s of size %d\n", objectName, n)

	pat := os.Getenv("DO_ACCESS_TOKEN")
	if pat == "" {
		log.Fatalln("Access token required")
	}

	tokenSource := &TokenSource{
		AccessToken: pat,
	}

	oauthClient := oauth2.NewClient(oauth2.NoContext, tokenSource)
	doClient := godo.NewClient(oauthClient)
	ctx := context.TODO()

	log.Printf("Listing CDNs")
	cdnName := "crime-map"
	cdn, err := getCDN(*doClient, ctx, cdnName)
	if err != nil || cdn == nil {
		log.Fatalln("Could not find CDN: " + cdnName)
	}

	log.Printf("Flushing CDN Cache")
	flushRequest := &godo.CDNFlushCacheRequest{
		Files: []string{
			"*",
		},
	}
	_, err = doClient.CDNs.FlushCache(ctx, cdn.ID, flushRequest)
}

func getCDN(client godo.Client, ctx context.Context, cdnName string) (*godo.CDN, error) {
	opt := &godo.ListOptions{
		Page:    1,
		PerPage: 200,
	}

	cdns, _, err := client.CDNs.List(ctx, opt)
	if err != nil {
		return nil, err
	}

	for i := range cdns {
		cdn := cdns[i]
		log.Println(cdn.Origin)
		if strings.HasPrefix(cdn.Origin, cdnName + ".") {
			return &cdn, nil
		}
	}

	return nil, nil
}