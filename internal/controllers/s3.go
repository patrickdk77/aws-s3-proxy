package controllers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-openapi/swag/typeutils"
	"github.com/karlseguin/ccache/v3"
	"github.com/patrickdk77/aws-s3-proxy/internal/config"
	"github.com/patrickdk77/aws-s3-proxy/internal/metrics"
	"github.com/patrickdk77/aws-s3-proxy/internal/service"
)

var (
	httpCache     *ccache.Cache[cachedResponse]
	cacheOnce     sync.Once
	NewClientFunc = service.NewClient
)

type cachedResponse struct {
	*s3.GetObjectOutput
	Body        []byte
	ContentType string
	Exists      bool
}

type ObjectOutput interface {
	s3.GetObjectOutput | s3.HeadObjectOutput
}

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
	if candidate := r.Header.Get("Range"); !typeutils.IsZero(candidate) {
		rangeHeader = aws.String(candidate)
	}

	client := NewClientFunc(r.Context(), aws.String(config.Config.AwsRegion))

	// Replace path with symlink.json
	idx := strings.Index(path, "symlink.json")
	if idx > -1 {
		replaced, err := replacePathWithSymlink(r, client, c.S3Bucket, c.S3KeyPrefix+path[:idx+12])
		if err != nil {
			code, message := toHTTPError(err)
			http.Error(w, message, code)
			return
		}
		path = aws.ToString(replaced) + path[idx+12:]
	}

	if c.CacheSize > 0 && c.CacheTTL > 0 && rangeHeader == nil {
		cacheOnce.Do(func() {
			httpCache = ccache.New(ccache.Configure[cachedResponse]().MaxSize(c.CacheSize))
		})
	}

	// Ends with / -> listing or index.html
	if strings.HasSuffix(path, "/") {
		if c.DirectoryListing {
			cacheKey := "IndexCache:=" + c.S3KeyPrefix + path
			var item *ccache.Item[cachedResponse]
			if httpCache != nil {
				item = httpCache.Get(cacheKey)
			}
			if item != nil && !item.Expired() {
				val := item.Value()
				if !val.Exists {
					w.Header().Set("Content-Type", val.ContentType)
					_, _ = w.Write(val.Body)
					return
				}
			} else {
				if !c.DirListingCheckIndex || !client.S3exists(r.Context(), c.S3Bucket, c.S3KeyPrefix+path+c.IndexDocument) {
					obj, err := s3listFiles(r, client, c.S3Bucket, c.S3KeyPrefix+path)
					if err != nil {
						if obj.Exists {
							http.Error(w, err.Error(), http.StatusInternalServerError)
						} else {
							code, message := toHTTPError(err)
							http.Error(w, message, code)
						}
						return
					}
					obj.Exists = false
					httpCache.Set(cacheKey, obj, c.CacheTTLIndex)
					w.Header().Set("Content-Type", obj.ContentType)
					_, _ = w.Write(obj.Body)
					return
				} else {
					obj := cachedResponse{Exists: true}
					httpCache.Set(cacheKey, obj, c.CacheTTLIndex)

				}
			}
		}
		path += c.IndexDocument
	}

	switch r.Method {
	case "GET":
		// Get a S3 object
		var obj *s3.GetObjectOutput
		var err error

		cacheKey := c.S3KeyPrefix + path
		var item *ccache.Item[cachedResponse]
		if httpCache != nil {
			item = httpCache.Get(cacheKey)
		}

		if item != nil && !item.Expired() {
			val := item.Value()
			obj = val.GetObjectOutput
			obj.Body = io.NopCloser(bytes.NewReader(val.Body))
		} else {
			obj, err = client.S3get(r.Context(), c.S3Bucket, c.S3KeyPrefix+path, rangeHeader)
			metrics.UpdateS3Reads(err, metrics.GetObjectAction, metrics.ProxySource)
			if err != nil {
				code, message := toHTTPError(err)
				if (code == 404 || code == 403) && c.SPA && !strings.Contains(path, c.IndexDocument) {
					idx := strings.LastIndex(path, "/")
					if idx > -1 {
						indexPath := c.S3KeyPrefix + path[:idx+1] + c.IndexDocument
						var indexError error
						obj, indexError = client.S3get(r.Context(), c.S3Bucket, indexPath, rangeHeader)
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
			if httpCache != nil && err == nil && aws.ToInt64(obj.ContentLength) <= c.CacheMaxFileSize {
				buf := new(bytes.Buffer)
				_, err := io.Copy(buf, obj.Body)
				if err == nil {
					obj.Body.Close()
					bodyBytes := buf.Bytes()
					obj.Body = io.NopCloser(bytes.NewReader(bodyBytes))

					// Calculate TTL
					ttl := c.CacheTTL
					if obj.CacheControl != nil {
						matches := regexp.MustCompile(`max-age=(\d+)`).FindStringSubmatch(*obj.CacheControl)
						if len(matches) == 2 {
							if seconds, err := strconv.Atoi(matches[1]); err == nil {
								if time.Duration(seconds)*time.Second < ttl {
									ttl = time.Duration(seconds) * time.Second
								}
							}
						}
					}

					httpCache.Set(cacheKey, cachedResponse{
						GetObjectOutput: obj,
						Body:            bodyBytes,
					}, ttl)
				}
			}
		}
		setHeadersFromAwsResponse(w, obj, c.HTTPCacheControl, c.HTTPExpires)
		w.WriteHeader(determineHTTPStatus(obj))
		_, _ = io.Copy(w, obj.Body) // nolint
	case "HEAD":
		// Head a S3 object
		var obj interface{}
		var err error

		cacheKey := c.S3KeyPrefix + path
		var item *ccache.Item[cachedResponse]
		if httpCache != nil {
			item = httpCache.Get(cacheKey)
		}

		if item != nil && !item.Expired() {
			val := item.Value()
			obj = val.GetObjectOutput
		} else {
			obj, err = client.S3head(r.Context(), c.S3Bucket, c.S3KeyPrefix+path, rangeHeader)
			// metrics.UpdateS3Reads(err, metrics.GetObjectAction, metrics.ProxySource)
			if err != nil {
				code, message := toHTTPError(err)
				if (code == 404 || code == 403) && c.SPA && !strings.Contains(path, c.IndexDocument) {
					idx := strings.LastIndex(path, "/")
					if idx > -1 {
						indexPath := c.S3KeyPrefix + path[:idx+1] + c.IndexDocument
						var indexError error
						obj, indexError = client.S3head(r.Context(), c.S3Bucket, indexPath, rangeHeader)
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
		}
		setHeadersFromAwsResponse(w, obj, c.HTTPCacheControl, c.HTTPExpires)
		w.WriteHeader(http.StatusOK)
	default:
		// return method not allowed, 405
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
}

func replacePathWithSymlink(r *http.Request, client service.AWS, bucket, symlinkPath string) (*string, error) {
	obj, err := client.S3get(r.Context(), bucket, symlinkPath, nil)
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

func setHeadersFromAwsResponse(w http.ResponseWriter, obj interface{}, httpCacheControl, httpExpires string) {
	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// Helper functions to get fields safely via reflection
	getString := func(fieldName string) *string {
		f := v.FieldByName(fieldName)
		if f.IsValid() && !f.IsNil() && f.Type() == reflect.TypeOf((*string)(nil)) {
			return f.Interface().(*string)
		}
		return nil
	}
	getTime := func(fieldName string) *time.Time {
		f := v.FieldByName(fieldName)
		if f.IsValid() && !f.IsNil() && f.Type() == reflect.TypeOf((*time.Time)(nil)) {
			return f.Interface().(*time.Time)
		}
		return nil
	}
	getInt64 := func(fieldName string) *int64 {
		f := v.FieldByName(fieldName)
		if f.IsValid() && !f.IsNil() && f.Type() == reflect.TypeOf((*int64)(nil)) {
			return f.Interface().(*int64)
		}
		return nil
	}

	// Cache-Control
	if len(httpCacheControl) > 0 {
		setStrHeader(w, "Cache-Control", &httpCacheControl)
	} else {
		setStrHeader(w, "Cache-Control", getString("CacheControl"))
	}
	// Expires
	if len(httpExpires) > 0 {
		setStrHeader(w, "Expires", &httpExpires)
	} else {
		// Try ExpiresString first (if it exists in generic object), then Expires
		if s := getString("ExpiresString"); s != nil {
			setStrHeader(w, "Expires", s)
		}
	}
	setStrHeader(w, "Content-Encoding", getString("ContentEncoding"))
	setStrHeader(w, "Content-Language", getString("ContentLanguage"))

	if len(w.Header().Get("Content-Encoding")) == 0 {
		setIntHeader(w, "Content-Length", getInt64("ContentLength"))
	}
	setStrHeader(w, "Content-Range", getString("ContentRange"))

	contentType := getString("ContentType")
	if config.Config.ContentType == "" {
		setStrHeader(w, "Content-Type", contentType)
	} else {
		setStrHeader(w, "Content-Type", &config.Config.ContentType)
	}

	contentDisposition := getString("ContentDisposition")
	if config.Config.ContentDisposition == "" {
		setStrHeader(w, "Content-Disposition", contentDisposition)
	} else {
		setStrHeader(w, "Content-Disposition", &config.Config.ContentDisposition)
	}
	setStrHeader(w, "ETag", getString("ETag"))
	setTimeHeader(w, "Last-Modified", getTime("LastModified"))

	// Location, rewrite to our own
	if len(w.Header().Get("Location")) > 0 {
		l, err := url.Parse(w.Header().Get("Location"))
		if err == nil && strings.Contains(l.Host, config.Config.S3Bucket) {
			path := l.RequestURI()
			setStrHeader(w, "Location", &path)
		}
	}
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

func s3listFiles(r *http.Request, client service.AWS, bucket, prefix string) (cachedResponse, error) {
	prefix = strings.TrimPrefix(prefix, "/")

	result, err := client.S3listObjects(r.Context(), bucket, prefix)
	metrics.UpdateS3Reads(err, metrics.ListObjectAction, metrics.ProxySource)
	if err != nil {
		return cachedResponse{}, err
	}
	files := convertToMaps(result, prefix)

	// Output as a HTML
	if strings.EqualFold(config.Config.DirListingFormat, "html") {
		return cachedResponse{
			Body:        []byte(toHTML(files)),
			ContentType: "text/html; charset=utf-8",
		}, nil
	}
	if strings.EqualFold(config.Config.DirListingFormat, "apache") {
		return cachedResponse{
			Body:        []byte(toApache(prefix, files)),
			ContentType: "text/html; charset=utf-8",
		}, nil
	}
	if strings.EqualFold(config.Config.DirListingFormat, "shtml") {
		return cachedResponse{
			Body:        []byte(toSimpleHTML(files)),
			ContentType: "text/html; charset=utf-8",
		}, nil
	}

	// Output as a JSON
	jsonBytes, merr := json.Marshal(files)
	if merr != nil {
		return cachedResponse{Exists: true}, merr
	}
	return cachedResponse{
		Body:        jsonBytes,
		ContentType: "application/json; charset=utf-8",
	}, nil
}

func convertToMaps(s3output *s3.ListObjectsV2Output, prefix string) s3objects {
	var candidates s3objects

	// Prefixes
	for _, obj := range s3output.CommonPrefixes {
		candidate := strings.TrimPrefix(aws.ToString(obj.Prefix), prefix)
		if len(candidate) == 0 {
			continue
		}
		candidates = append(candidates, s3item{candidate, -1, time.Time{}})
	}
	// Contents
	for _, obj := range s3output.Contents {
		candidate := strings.TrimPrefix(aws.ToString(obj.Key), prefix)
		if len(candidate) == 0 {
			continue
		}
		candidates = append(candidates, s3item{candidate, *obj.Size, *obj.LastModified})
	}
	// Sort
	sort.Sort(candidates)

	return candidates
}

func toHTML(files s3objects) string {
	html := "<!DOCTYPE html><html><head><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"></head><body><ul>"
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
	html := "<!DOCTYPE html><html><head><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>Index of " + prefix + "</title></head>"
	html += "<body><h1>Index of " + prefix + "</h1><pre><table><tr><th>Name</th><th>Last Modified</th><th>Size</th></tr>"
	for _, file := range files {
		html += "<tr><td><a href=\"" + file.file + "\">" + file.file + "</a></td>"
		if !file.updatedAt.IsZero() {
			html += "<td>" + file.updatedAt.Format(time.RFC3339) + "</td>"
		} else {
			html += "<td>-</td>"
		}
		if file.size >= 0 {
			fsizeMod := ""
			if file.size > 2000 {
				fsizeMod = "k"
				file.size /= 1024
			}
			if file.size > 2000 {
				fsizeMod = "M"
				file.size /= 1024
			}
			if file.size > 2000 {
				fsizeMod = "G"
				file.size /= 1024
			}
			html += "<td>" + strconv.FormatInt(file.size, 10) + fsizeMod + "</td>"
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
