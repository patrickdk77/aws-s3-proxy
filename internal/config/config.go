package config

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config represents its configurations
var (
	Config *config
	AccessLog *log.Logger
)

func init() {
	Setup()
}

type config struct { // nolint
	AwsRegion            string        // AWS_REGION
	AwsAPIEndpoint       string        // AWS_API_ENDPOINT
	S3Bucket             string        // AWS_S3_BUCKET
	S3KeyPrefix          string        // AWS_S3_KEY_PREFIX
	IndexDocument        string        // INDEX_DOCUMENT
	DirectoryListing     bool          // DIRECTORY_LISTINGS
	DirListingFormat     string        // DIRECTORY_LISTINGS_FORMAT
	DirListingCheckIndex bool          // DIRECTORY_LISTINGS_CHECK_INDEX
	HTTPCacheControl     string        // HTTP_CACHE_CONTROL (max-age=86400, no-cache ...)
	HTTPExpires          string        // HTTP_EXPIRES (Thu, 01 Dec 1994 16:00:00 GMT ...)
	BasicAuthUser        []string      // BASIC_AUTH_USER
	BasicAuthPass        []string      // BASIC_AUTH_PASS
	Port                 string        // APP_PORT
	Host                 string        // APP_HOST
	AccessLog            bool          // ACCESS_LOG
	ForwardedFor         string        // FORWARDED_FOR
	SslCert              string        // SSL_CERT_PATH
	SslKey               string        // SSL_KEY_PATH
	StripPath            string        // STRIP_PATH
	ContentEncoding      bool          // CONTENT_ENCODING
	CorsAllowOrigin      string        // CORS_ALLOW_ORIGIN
	CorsAllowMethods     string        // CORS_ALLOW_METHODS
	CorsAllowHeaders     string        // CORS_ALLOW_HEADERS
	CorsMaxAge           int64         // CORS_MAX_AGE
	HealthCheckPath      string        // HEALTHCHECK_PATH
	MetricsPath          string        // METRICS_PATH
	VersionPath          string        // VERSION_PATH
	AllPagesInDir        bool          // GET_ALL_PAGES_IN_DIR
	MaxIdleConns         int           // MAX_IDLE_CONNECTIONS
	IdleConnTimeout      time.Duration // IDLE_CONNECTION_TIMEOUT
	DisableCompression   bool          // DISABLE_COMPRESSION
	InsecureTLS          bool          // Disables TLS validation on request endpoints.
	JwtSecretKey         string        // JWT_SECRET_KEY
        JwtUserField         string        // JWT_USER_FIELD
        JwtHeader            string        // JWT_HEADER
	SPA                  bool          // SPA
	WhiteListIPRanges    []*net.IPNet  // WHITELIST_IP_RANGES is commma separated list of IP's and IP ranges. Needs parsing.
	ContentType          string        // Override default Content-Type
	ContentDisposition   string        // Override default Content-Disposition
	UsernameHeader       string        // Username Header Cf-Access-Authenticated-User-Email
}

// Setup configurations with environment variables
func Setup() {
	region := os.Getenv("AWS_REGION")
	if len(region) == 0 {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	port := os.Getenv("APP_PORT")
	if len(port) == 0 {
		port = "80"
	}
	indexDocument := os.Getenv("INDEX_DOCUMENT")
	if len(indexDocument) == 0 {
		indexDocument = "index.html"
	}
	directoryListings := false
	if b, err := strconv.ParseBool(os.Getenv("DIRECTORY_LISTINGS")); err == nil {
		directoryListings = b
	}
	directoryListingsCheckIndex := false
	if b, err := strconv.ParseBool(os.Getenv("DIRECTORY_LISTINGS_CHECK_INDEX")); err == nil {
		directoryListingsCheckIndex = b
	}
	accessLog := false
	if b, err := strconv.ParseBool(os.Getenv("ACCESS_LOG")); err == nil {
		accessLog = b
	}
	contentEncoding := true
	if b, err := strconv.ParseBool(os.Getenv("CONTENT_ENCODING")); err == nil {
		contentEncoding = b
	}
	corsMaxAge := int64(600)
	if i, err := strconv.ParseInt(os.Getenv("CORS_MAX_AGE"), 10, 64); err == nil {
		corsMaxAge = i
	}
	allPagesInDir := false
	if b, err := strconv.ParseBool(os.Getenv("GET_ALL_PAGES_IN_DIR")); err == nil {
		allPagesInDir = b
	}
	maxIdleConns := 150
	if b, err := strconv.ParseInt(os.Getenv("MAX_IDLE_CONNECTIONS"), 10, 16); err == nil {
		maxIdleConns = int(b)
	}
	idleConnTimeout := time.Duration(10) * time.Second
	if b, err := strconv.ParseInt(os.Getenv("IDLE_CONNECTION_TIMEOUT"), 10, 64); err == nil {
		idleConnTimeout = time.Duration(b) * time.Second
	}
	disableCompression := true
	if b, err := strconv.ParseBool(os.Getenv("DISABLE_COMPRESSION")); err == nil {
		disableCompression = b
	}
	insecureTLS := false
	if b, err := strconv.ParseBool(os.Getenv("INSECURE_TLS")); err == nil {
		insecureTLS = b
	}
	SPA := false
	if b, err := strconv.ParseBool(os.Getenv("SPA")); err == nil {
		SPA = b
	}
	whiteListIPRanges := []*net.IPNet{}
	var err error
	if whiteListIPRangesStr := os.Getenv("WHITELIST_IP_RANGES"); len(whiteListIPRangesStr) != 0 {
		whiteListIPRangesTemp := strings.Split(whiteListIPRangesStr, ",")
		whiteListIPRanges, err = createIPNets(whiteListIPRangesTemp)
		if err != nil {
			log.Fatalf("%v", err)
		}
	}
        usernames := []string{}
	username := os.Getenv("BASIC_AUTH_USER")
	if username != "" {
		usernames = strings.Split(username, " ")
	}
        passwords := []string{}
	password := os.Getenv("BASIC_AUTH_PASS")
	if password != "" {
		passwords = strings.Split(password, " ")
	}

	Config = &config{
		AwsRegion:            region,
		AwsAPIEndpoint:       os.Getenv("AWS_API_ENDPOINT"),
		S3Bucket:             os.Getenv("AWS_S3_BUCKET"),
		S3KeyPrefix:          os.Getenv("AWS_S3_KEY_PREFIX"),
		IndexDocument:        indexDocument,
		DirectoryListing:     directoryListings,
		DirListingCheckIndex: directoryListingsCheckIndex,
		DirListingFormat:     os.Getenv("DIRECTORY_LISTINGS_FORMAT"),
		HTTPCacheControl:     os.Getenv("HTTP_CACHE_CONTROL"),
		HTTPExpires:          os.Getenv("HTTP_EXPIRES"),
		BasicAuthUser:        usernames,
		BasicAuthPass:        passwords,
		Port:                 port,
		Host:                 os.Getenv("APP_HOST"),
		AccessLog:            accessLog,
		ForwardedFor:         os.Getenv("FORWARDED_FOR"),
		SslCert:              os.Getenv("SSL_CERT_PATH"),
		SslKey:               os.Getenv("SSL_KEY_PATH"),
		StripPath:            os.Getenv("STRIP_PATH"),
		ContentEncoding:      contentEncoding,
		CorsAllowOrigin:      os.Getenv("CORS_ALLOW_ORIGIN"),
		CorsAllowMethods:     os.Getenv("CORS_ALLOW_METHODS"),
		CorsAllowHeaders:     os.Getenv("CORS_ALLOW_HEADERS"),
		CorsMaxAge:           corsMaxAge,
		HealthCheckPath:      os.Getenv("HEALTHCHECK_PATH"),
		MetricsPath:          os.Getenv("METRICS_PATH"),
		VersionPath:          os.Getenv("VERSION_PATH"),
		AllPagesInDir:        allPagesInDir,
		MaxIdleConns:         maxIdleConns,
		IdleConnTimeout:      idleConnTimeout,
		DisableCompression:   disableCompression,
		InsecureTLS:          insecureTLS,
		JwtSecretKey:         os.Getenv("JWT_SECRET_KEY"),
		JwtUserField:         os.Getenv("JWT_USER_FIELD"),
		JwtHeader:            os.Getenv("JWT_HEADER"),
		SPA:                  SPA,
		WhiteListIPRanges:    whiteListIPRanges,
		ContentType:          os.Getenv("CONTENT_TYPE"),
		ContentDisposition:   os.Getenv("CONTENT_DISPOSITION"),
		UsernameHeader:       os.Getenv("USERNAME_HEADER"),
	}

	// Proxy
	log.Printf("[config] Proxy to %v", Config.S3Bucket)
	log.Printf("[config] AWS Region: %v", Config.AwsRegion)

	// TLS pem files
	if (len(Config.SslCert) > 0) && (len(Config.SslKey) > 0) {
		log.Print("[config] TLS enabled.")
	}
	// Basic authentication
	if (len(Config.BasicAuthUser) > 0) && (len(Config.BasicAuthPass) > 0) {
		log.Printf("[config] Basic authentication: %s", Config.BasicAuthUser)
	}
	// CORS
	if (len(Config.CorsAllowOrigin) > 0) && (Config.CorsMaxAge > 0) {
		log.Printf("[config] CORS enabled: %s", Config.CorsAllowOrigin)
	}
	// WhiteListIPRanges
	if len(Config.WhiteListIPRanges) > 0 {
		log.Printf("[config] WhiteListIPRanges enabled: %v", Config.WhiteListIPRanges)
	}
	if Config.AccessLog {
		AccessLog = log.New(os.Stdout, "",0)
	}
}

func createIPNets(src []string) ([]*net.IPNet, error) {
	whiteListIPRanges := make([]*net.IPNet, 0, len(src))
	for _, whiteListIPRange := range src {
		if !strings.Contains(whiteListIPRange, "/") {
			// Make range from single IP
			whiteListIPRange = fmt.Sprintf("%s/32", whiteListIPRange)
		}
		_, subnet, err := net.ParseCIDR(whiteListIPRange)
		if err != nil {
			return nil, fmt.Errorf("[config] invalid IP range '%s' in WHITELIST_IP_RANGES: %w", whiteListIPRange, err)
		}
		whiteListIPRanges = append(whiteListIPRanges, subnet)
	}
	return whiteListIPRanges, nil
}
