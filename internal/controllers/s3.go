package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/go-openapi/swag"
	"github.com/patrickdk77/aws-s3-proxy/internal/config"
	"github.com/patrickdk77/aws-s3-proxy/internal/service"
	"github.com/patrickdk77/aws-s3-proxy/internal/metrics"
)

// AwsS3 handles requests for Amazon S3
func AwsS3(w http.ResponseWriter, r *http.Request) {
	c := config.Config

	// Strip the prefix, if it's present.
	path := r.URL.Path
	if len(c.StripPath) > 0 {
		path = strings.TrimPrefix(path, c.StripPath)
	}

	// Range header
	var rangeHeader *string
	if candidate := r.Header.Get("Range"); !swag.IsZero(candidate) {
		rangeHeader = aws.String(candidate)
	}

	client := service.NewClient(r.Context(), aws.String(config.Config.AwsRegion))

	// Replace path with symlink.json
	idx := strings.Index(path, "symlink.json")
	if idx > -1 {
		replaced, err := replacePathWithSymlink(client, c.S3Bucket, c.S3KeyPrefix+path[:idx+12])
		if err != nil {
			code, message := toHTTPError(err)
			http.Error(w, message, code)
			return
		}
		path = aws.StringValue(replaced) + path[idx+12:]
	}
	// Ends with / -> listing or index.html
	if strings.HasSuffix(path, "/") {
		if c.DirectoryListing {
			if !c.DirListingCheckIndex || !client.S3exists(c.S3Bucket, c.S3KeyPrefix+path+c.IndexDocument) {
				s3listFiles(w, r, client, c.S3Bucket, c.S3KeyPrefix+path)
				return
			}
		}
		path += c.IndexDocument
	}
	// Get a S3 object
	obj, err := client.S3get(c.S3Bucket, c.S3KeyPrefix+path, rangeHeader)
	metrics.UpdateS3Reads(err, metrics.GetObjectAction, metrics.ProxySource)
	if err != nil {
		code, message := toHTTPError(err)

		if code == 404 && c.SPA && strings.Index(path, c.IndexDocument) == -1 {

			idx := strings.LastIndex(path, "/")

			if idx > -1 {
				indexPath := c.S3KeyPrefix + path[:idx+1] + c.IndexDocument

				var indexError error
				obj, indexError = client.S3get(c.S3Bucket, indexPath, rangeHeader)

				if indexError != nil {
					code, message = toHTTPError(indexError)
					http.Error(w, message, code)
					return
				}
			}
		} else {
			http.Error(w, message, code)
			return
		}
	}
	setHeadersFromAwsResponse(w, obj, c.HTTPCacheControl, c.HTTPExpires)

	_, _ = io.Copy(w, obj.Body) // nolint
}

func replacePathWithSymlink(client service.AWS, bucket, symlinkPath string) (*string, error) {
	obj, err := client.S3get(bucket, symlinkPath, nil)
	metrics.UpdateS3Reads(err, metrics.GetObjectAction, metrics.ProxySource)
	if err != nil {
		return nil, err
	}
	link := struct {
		URL string
	}{}
	buf := new(bytes.Buffer)
	if _, err = buf.ReadFrom(obj.Body); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(buf.Bytes(), &link); err != nil {
		return nil, err
	}
	return aws.String(link.URL), nil
}

func setHeadersFromAwsResponse(w http.ResponseWriter, obj *s3.GetObjectOutput, httpCacheControl, httpExpires string) {

	// Cache-Control
	if len(httpCacheControl) > 0 {
		setStrHeader(w, "Cache-Control", &httpCacheControl)
	} else {
		setStrHeader(w, "Cache-Control", obj.CacheControl)
	}
	// Expires
	if len(httpExpires) > 0 {
		setStrHeader(w, "Expires", &httpExpires)
	} else {
		setStrHeader(w, "Expires", obj.Expires)
	}
	setStrHeader(w, "Content-Encoding", obj.ContentEncoding)
	setStrHeader(w, "Content-Language", obj.ContentLanguage)

	// Fix https://github.com/patrickdk77/aws-s3-proxy/issues/20
	if len(w.Header().Get("Content-Encoding")) == 0 {
		setIntHeader(w, "Content-Length", obj.ContentLength)
	}
	setStrHeader(w, "Content-Range", obj.ContentRange)
	if config.Config.ContentType == "" {
		setStrHeader(w, "Content-Type", obj.ContentType)
	} else {
		setStrHeader(w, "Content-Type", &config.Config.ContentType)
	}
	if config.Config.ContentDisposition == "" {
		setStrHeader(w, "Content-Disposition", obj.ContentDisposition)
	} else {
		setStrHeader(w, "Content-Disposition", &config.Config.ContentDisposition)
	}
	setStrHeader(w, "ETag", obj.ETag)
	setTimeHeader(w, "Last-Modified", obj.LastModified)

	w.WriteHeader(determineHTTPStatus(obj))
}

func setStrHeader(w http.ResponseWriter, key string, value *string) {
	if value != nil && len(*value) > 0 {
		w.Header().Add(key, *value)
	}
}

func setIntHeader(w http.ResponseWriter, key string, value *int64) {
	if value != nil && *value > 0 {
		w.Header().Add(key, strconv.FormatInt(*value, 10))
	}
}

func setTimeHeader(w http.ResponseWriter, key string, value *time.Time) {
	if value != nil && !reflect.DeepEqual(*value, time.Time{}) {
		w.Header().Add(key, value.UTC().Format(http.TimeFormat))
	}
}

func s3listFiles(w http.ResponseWriter, r *http.Request, client service.AWS, bucket, prefix string) {
	prefix = strings.TrimPrefix(prefix, "/")

	result, err := client.S3listObjects(bucket, prefix)
	metrics.UpdateS3Reads(err, metrics.ListObjectAction, metrics.ProxySource)
	if err != nil {
		code, message := toHTTPError(err)
		http.Error(w, message, code)
		return
	}
	files := convertToMaps(result, prefix)

	// Output as a HTML
	if strings.EqualFold(config.Config.DirListingFormat, "html") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintln(w, toHTML(files))
		return
	}
	if strings.EqualFold(config.Config.DirListingFormat, "apache") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintln(w, toApache(prefix, files))
		return
	}
	if strings.EqualFold(config.Config.DirListingFormat, "shtml") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintln(w, toSimpleHTML(files))
		return
	}

	// Output as a JSON
	jsonBytes, merr := json.Marshal(files)
	if merr != nil {
		http.Error(w, merr.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = fmt.Fprintln(w, string(jsonBytes))
}

func convertToMaps(s3output *s3.ListObjectsOutput, prefix string) (s3objects) {
	var candidates s3objects

	// Prefixes
	for _, obj := range s3output.CommonPrefixes {
		candidate := strings.TrimPrefix(aws.StringValue(obj.Prefix), prefix)
		if len(candidate) == 0 {
			continue
		}
		candidates = append(candidates, s3item{candidate, -1, time.Time{}})
	}
	// Contents
	for _, obj := range s3output.Contents {
		candidate := strings.TrimPrefix(aws.StringValue(obj.Key), prefix)
		if len(candidate) == 0 {
			continue
		}
		candidates = append(candidates, s3item{candidate, *obj.Size, *obj.LastModified})
	}
	// Sort
	sort.Sort(s3objects(candidates))

	return candidates
}

func toHTML(files s3objects) string {
	html := "<!DOCTYPE html><html><body><ul>"
	for _, file := range files {
		html += "<li><a href=\"" + file.file + "\">" + file.file + "</a>"
		if !file.updatedAt.IsZero() {
			html += " " + file.updatedAt.Format(time.RFC3339)
		}
		html += "</li>"
	}
	return html + "</ul></body></html>"
}

func toApache(prefix string, files s3objects) string {
	html := "<!DOCTYPE html><html><head><title>Index of " + prefix + "</title></head>"
	html += "<body><h1>Index of " + prefix + "</h1><pre><table><tr><th>Name</th><th>Last Modified</th><th>Size</th></tr>"
	for _, file := range files {
		html += "<tr><td><a href=\"" + file.file + "\">" + file.file + "</a></td>"
		if !file.updatedAt.IsZero()  {
			html += "<td>" + file.updatedAt.Format(time.RFC3339) + "</td>"
		} else {
			html += "<td>-</td>"
		}
		if file.size >=0 {
			fsizeMod := ""
		        if file.size > 2000 {
		        	fsizeMod="k"
		        	file.size /= 1024
			}
		        if file.size > 2000 {
		        	fsizeMod="M"
		        	file.size /= 1024
			}
		        if file.size > 2000 {
		        	fsizeMod="G"
		        	file.size /= 1024
			}
			html += "<td>" + strconv.FormatInt(file.size,10) + fsizeMod + "</td>"
		} else {
			html += "<td>-</td>"
		}
		html += "</tr>"
	}
	return html + "</table><hr></pre></body></html>"
}

func toSimpleHTML(files s3objects) string {

	html := "<!DOCTYPE html><html><body>"
	for _, file := range files {
		html += "<a href=\"" + file.file + "\">" + file.file + "</a><br>"
	}
	return html + "</body></html>"
}
