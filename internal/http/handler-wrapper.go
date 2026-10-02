package http

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/patrickdk77/aws-s3-proxy/internal/config"
)

const preflightHeader = "Access-Control-Request-Method"

type ReqInfo struct {
	stime     time.Time
	method    string
	proto     string
	uri       string
	ip        string
	port      string
	status    int
	size      int64
	referer   string
	userAgent string
	user      string
	host      string
}

// WrapHandler wraps every handlers
func WrapHandler(handler func(w http.ResponseWriter, r *http.Request)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := config.Config

		addr := getIP(r)
		rawIP, rawPort, _ := net.SplitHostPort(r.RemoteAddr)
		clientIP, clientPort, err := net.SplitHostPort(addr)
		if err != nil {
			clientIP, clientPort, err = net.SplitHostPort(addr)
			if err != nil {
				clientIP, clientPort, err = net.SplitHostPort(net.JoinHostPort(addr, rawPort))
				if err != nil {
					clientIP = rawIP
					clientPort = rawPort
				}
			}
		}

		ri := &ReqInfo{
			stime:     time.Now(),
			method:    r.Method,
			uri:       r.URL.String(),
			proto:     r.Proto,
			ip:        clientIP,
			port:      clientPort,
			size:      0,
			status:    0,
			referer:   r.Header.Get("Referer"),
			userAgent: r.Header.Get("User-Agent"),
			host:      r.Host,
			user:      "-",
		}

		// WhiteListIPs
		if len(c.WhiteListIPRanges) > 0 {
			found := false
			for _, whiteListIPRange := range c.WhiteListIPRanges {
				ip := net.ParseIP(clientIP)
				found = whiteListIPRange.Contains(ip)
				if found {
					break
				}
			}
			if !found {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				ri.status = http.StatusUnauthorized
				accessLog(ri)
				return
			}
		}

		// CORS
		if (len(c.CorsAllowOrigin) > 0) &&
			(len(c.CorsAllowMethods) > 0) &&
			(len(c.CorsAllowHeaders) > 0) &&
			(c.CorsMaxAge > 0) {
			w.Header().Set("Access-Control-Allow-Origin", c.CorsAllowOrigin)
			w.Header().Set("Access-Control-Allow-Methods", c.CorsAllowMethods)
			w.Header().Set("Access-Control-Allow-Headers", c.CorsAllowHeaders)
			w.Header().Set("Access-Control-Max-Age", strconv.FormatInt(c.CorsMaxAge, 10))
			if r.Method == http.MethodOptions &&
				r.Header.Get(preflightHeader) != "" {
				w.WriteHeader(http.StatusNoContent)
				ri.status = http.StatusNoContent
				accessLog(ri)
				return
			}
		}
		if len(c.UsernameHeader) > 0 && len(r.Header.Get(c.UsernameHeader)) > 0 {
			ri.user = r.Header.Get(c.UsernameHeader)
		}
		// BasicAuth
		if (len(c.BasicAuthUser) > 0) && (len(c.BasicAuthPass) > 0) &&
			!auth(r, c.BasicAuthUser, c.BasicAuthPass, ri) {
			w.Header().Set("WWW-Authenticate", `Basic realm="REALM"`)
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			ri.status = http.StatusUnauthorized
			accessLog(ri)
			return
		}
		// Auth with JWT
		if (len(c.JwtUserField) > 0 || len(c.JwtSecretKey) > 0) && !isValidJwt(r, ri) {
			w.Header().Set("WWW-Authenticate", `Basic realm="REALM"`)
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			ri.status = http.StatusUnauthorized
			accessLog(ri)
			return
		}
		// Content-Encoding
		encoding := ""
		if c.ContentEncoding {
			w.Header().Add("Vary", "Accept-Encoding")
			accept, _ := header(r, "Accept-Encoding")
			for _, e := range splitCsvLine(accept) {
				if e == "gzip" || e == "deflate" {
					encoding = e
					break
				}
			}
		}
		// Handle HTTP requests
		writer := &custom{Writer: w, ResponseWriter: w,
			status: http.StatusOK, encoding: encoding}
		handler(writer, r)
		_ = writer.Close()

		ri.status = writer.status
		ri.size = writer.Written
		accessLog(ri)
	})
}

// getIP gets a requests IP address by reading off the forwarded-for
// header (for proxies) and falls back to use the remote address.
func getIP(r *http.Request) string {
	retIP := r.RemoteAddr
	if len(config.Config.ForwardedFor) > 0 {
		values := r.Header.Values(config.Config.ForwardedFor)
		list := strings.Split(strings.Join(values, ","), ",")
		for i := len(list) - 1; i >= 0; i-- {
			address := strings.TrimSpace(list[i])
			if address != "" {
				return address
			}
		}
	}
	return retIP
}

func auth(r *http.Request, authUser, authPass []string, ri *ReqInfo) bool {
	if username, password, ok := r.BasicAuth(); ok {
		n := min(len(authUser), len(authPass))
		for i := 0; i < n; i++ {
			u := subtle.ConstantTimeCompare(
				[]byte(username), []byte(authUser[i]))
			p := subtle.ConstantTimeCompare(
				[]byte(password), []byte(authPass[i]))
			if u&p == 1 {
				ri.user = authUser[i]
				return true
			}
		}
	}
	return false
}

func header(r *http.Request, key string) (string, bool) {
	if r.Header == nil {
		return "", false
	}
	if candidate := r.Header[key]; len(candidate) > 0 {
		return candidate[0], true
	}
	return "", false
}

func splitCsvLine(data string) []string {
	splitted := strings.Split(data, ",")
	parsed := make([]string, len(splitted))
	for i, val := range splitted {
		parsed[i] = strings.TrimSpace(val)
	}
	return parsed
}

func isValidJwt(r *http.Request, ri *ReqInfo) bool {
	value := len(config.Config.JwtSecretKey) == 0
	reqToken := r.Header.Get("Authorization")
	if len(config.Config.JwtHeader) > 0 {
		reqToken = r.Header.Get(config.Config.JwtHeader)
	} else {
		splitToken := strings.Split(reqToken, "Bearer")
		if len(splitToken) != 2 {
			// Error: Bearer token not in proper format
			return value
		}
		reqToken = strings.TrimSpace(splitToken[1])
	}
	if len(reqToken) < 1 {
		return value
	}
	token, err := jwt.Parse(reqToken, func(t *jwt.Token) (interface{}, error) {
		secretKey := config.Config.JwtSecretKey
		return []byte(secretKey), nil
	})
	if token == nil {
		return value
	}
	claims, _ := token.Claims.(jwt.MapClaims)
	if len(config.Config.JwtUserField) > 0 {
		if user, ok := claims[config.Config.JwtUserField].(string); ok {
			ri.user = user
		}
	}
	if value {
		return true
	}
	if err != nil {
		return false
	}
	return token.Valid
}

func accessLog(ri *ReqInfo) {
	if config.Config.AccessLog && config.Config.HealthCheckPath != ri.uri {
		if ri.referer == "" {
			ri.referer = "-"
		}
		if ri.userAgent == "" {
			ri.userAgent = "-"
		}
		if ri.host == "" {
			ri.host = "-"
		}
		config.AccessLog.Printf("%s %s - %s [%s] \"%s %s %s\" %d %d \"%s\" \"%s\" %.3f",
			ri.host, ri.ip, ri.user,
			ri.stime.Format("2006-01-02 15:04:05 -0000"),
			ri.method, ri.uri, ri.proto,
			ri.status, ri.size, ri.referer, ri.userAgent,
			time.Since(ri.stime).Seconds())
	}
}
