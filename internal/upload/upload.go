package upload

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/openzerg/nydus/internal/config"
)

type Uploader struct {
	client *s3.Client
	bucket string
	prefix string
}

func New(cfg *config.Config) *Uploader {
	if cfg.S3Endpoint == "" || cfg.S3Bucket == "" {
		return nil
	}
	creds := awscreds.NewStaticCredentialsProvider(cfg.S3AccessKey, cfg.S3SecretKey, "")
	client := s3.New(s3.Options{
		Credentials:      creds,
		Region:           cfg.S3Region,
		EndpointResolver: s3.EndpointResolverFromURL(cfg.S3Endpoint),
		UsePathStyle:     true,
	})
	log.Printf("[upload] s3 enabled: endpoint=%s bucket=%s", cfg.S3Endpoint, cfg.S3Bucket)
	return &Uploader{client: client, bucket: cfg.S3Bucket, prefix: "chat/"}
}

func (u *Uploader) Enabled() bool {
	return u != nil
}

type UploadResult struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
	MimeType string `json:"mime_type"`
	FileSize int64  `json:"file_size"`
	URL      string `json:"url"`
}

func (u *Uploader) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(100 << 20); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	chatroomID := r.FormValue("chatroom_id")
	if chatroomID == "" {
		http.Error(w, "chatroom_id required", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)
	id, _ := randomHex(12)
	key := fmt.Sprintf("%s%s/%s%s", u.prefix, chatroomID, id, ext)

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = mime.TypeByExtension(ext)
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	_, err = u.client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      aws.String(u.bucket),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String(mimeType),
	})
	if err != nil {
		log.Printf("[upload] s3 put error: %v", err)
		http.Error(w, "upload failed", http.StatusInternalServerError)
		return
	}

	result := UploadResult{
		FileID:   id,
		FileName: header.Filename,
		MimeType: mimeType,
		FileSize: header.Size,
		URL:      fmt.Sprintf("%s/%s", u.bucket, key),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (u *Uploader) ServeDownload(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/files/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	bucket := parts[0]
	key := parts[1]

	out, err := u.client.GetObject(r.Context(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer out.Body.Close()

	if out.ContentType != nil {
		w.Header().Set("Content-Type", *out.ContentType)
	}
	if out.ContentLength != nil {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", *out.ContentLength))
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	io.Copy(w, out.Body)
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
