// Package protonmail implements a ProtonMail API client.
package protonmail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"

	"log"
)

const Version = 3

const headerAPIVersion = "X-Pm-Apiversion"

// Human verification headers. Proton's official stacks attach these to
// every request once the user has solved a challenge.
const (
	headerHumanVerificationToken     = "x-pm-human-verification-token"
	headerHumanVerificationTokenType = "x-pm-human-verification-token-type"

	// CodeHumanVerification is returned by Proton when a client must
	// complete a captcha, recovery-email or SMS challenge before it is
	// allowed to authenticate.
	CodeHumanVerification = 9001
	// CodeStaleCaptcha signals that a previously supplied verification
	// token is no longer accepted.
	CodeStaleCaptcha = 12087

	// VerificationHost serves the captcha challenge page.
	VerificationHost = "verify.proton.me"
)

type resp struct {
	Code int
	*RawAPIError
}

func (r *resp) Err() error {
	if err := r.RawAPIError; err != nil {
		return &APIError{
			Code:                r.Code,
			Message:             err.Message,
			VerificationToken:   err.Details.HumanVerificationToken,
			VerificationMethods: err.Details.HumanVerificationMethods,
		}
	}
	return nil
}

type maybeError interface {
	Err() error
}

// ErrorDetails is the "Details" object Proton attaches to 9001 errors. It
// carries the opaque token the client must echo back after the user has
// solved the challenge.
type ErrorDetails struct {
	HumanVerificationToken   string   `json:"HumanVerificationToken"`
	HumanVerificationMethods []string `json:"HumanVerificationMethods"`
}

type RawAPIError struct {
	Message string       `json:"Error"`
	Details ErrorDetails `json:"Details"`
}

type APIError struct {
	Code    int
	Message string
	// VerificationToken is the opaque challenge token from a 9001
	// response, empty when absent.
	VerificationToken string
	// VerificationMethods lists the accepted challenge types, e.g.
	// ["captcha", "email", "sms"].
	VerificationMethods []string
}

func (err *APIError) Error() string {
	return fmt.Sprintf("[%v] %v", err.Code, err.Message)
}

// NeedsHumanVerification reports whether err is Proton's request for a
// captcha or another human challenge, or a stale verification token.
func (err *APIError) NeedsHumanVerification() bool {
	return err.Code == CodeHumanVerification || err.Code == CodeStaleCaptcha
}

// SupportsCaptcha reports whether "captcha" is among the accepted
// challenge methods.
func (err *APIError) SupportsCaptcha() bool {
	for _, m := range err.VerificationMethods {
		if m == "captcha" {
			return true
		}
	}
	return false
}

// VerificationURL builds the challenge URL to open in a browser. Returns
// false when the response carries no token or does not accept captcha, in
// which case the caller should fall back to manual instructions.
func (err *APIError) VerificationURL() (string, bool) {
	if err.VerificationToken == "" || !err.SupportsCaptcha() {
		return "", false
	}
	// The token is server-supplied and lands in a query value, so build
	// the URL rather than interpolating it.
	u := url.URL{Scheme: "https", Host: VerificationHost}
	q := url.Values{}
	q.Set("token", err.VerificationToken)
	q.Set("methods", "captcha")
	u.RawQuery = q.Encode()
	return u.String(), true
}

type Timestamp int64

func NewTimestamp(t time.Time) Timestamp {
	return Timestamp(t.Unix())
}

func (t Timestamp) Time() time.Time {
	return time.Unix(int64(t), 0)
}

// Client is a ProtonMail API client.
type Client struct {
	RootURL    string
	AppVersion string
	Debug      bool

	HTTPClient *http.Client
	ReAuth     func() error

	// HumanVerificationToken and HumanVerificationTokenType hold the
	// result of a solved captcha challenge. When both are set, they are
	// attached to every outgoing request, matching the official Proton
	// stacks.
	HumanVerificationToken     string
	HumanVerificationTokenType string

	uid         string
	accessToken string
	keyRing     openpgp.EntityList
}

// SetHumanVerificationToken records a solved challenge so subsequent
// requests carry the verification headers. Call with an empty token to
// clear them.
func (c *Client) SetHumanVerificationToken(token, tokenType string) {
	c.HumanVerificationToken = token
	c.HumanVerificationTokenType = tokenType
}

func (c *Client) setRequestAuthorization(req *http.Request) {
	if c.uid != "" && c.accessToken != "" {
		req.Header.Set("X-Pm-Uid", c.uid)
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}
}

func (c *Client) newRequest(method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, c.RootURL+path, body)
	if err != nil {
		return nil, err
	}

	if c.Debug {
		log.Printf(">> %v %v\n", req.Method, req.URL.Path)
	}

	req.Header.Set("X-Pm-Appversion", c.AppVersion)
	req.Header.Set(headerAPIVersion, strconv.Itoa(Version))
	if c.HumanVerificationToken != "" && c.HumanVerificationTokenType != "" {
		req.Header.Set(headerHumanVerificationToken, c.HumanVerificationToken)
		req.Header.Set(headerHumanVerificationTokenType, c.HumanVerificationTokenType)
	}
	c.setRequestAuthorization(req)
	return req, nil
}

func (c *Client) newJSONRequest(method, path string, body interface{}) (*http.Request, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, err
	}
	b := buf.Bytes()

	req, err := c.newRequest(method, path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}

	if c.Debug {
		log.Print(string(b))
	}

	req.Header.Set("Content-Type", "application/json")
	req.GetBody = func() (io.ReadCloser, error) {
		return ioutil.NopCloser(bytes.NewReader(b)), nil
	}
	return req, nil
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:101.0) Gecko/20100101 Firefox/101.0")

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return resp, err
	}

	// Check if access token has expired
	_, hasAuth := req.Header["Authorization"]
	canRetry := req.Body == nil || req.GetBody != nil
	if resp.StatusCode == http.StatusUnauthorized && hasAuth && c.ReAuth != nil && canRetry {
		resp.Body.Close()
		c.accessToken = ""
		if err := c.ReAuth(); err != nil {
			return resp, err
		}
		c.setRequestAuthorization(req) // Access token has changed
		if req.Body != nil {
			body, err := req.GetBody()
			if err != nil {
				return resp, err
			}
			req.Body = body
		}
		return c.do(req)
	}

	return resp, nil
}

func (c *Client) doJSON(req *http.Request, respData interface{}) error {
	req.Header.Set("Accept", "application/json")

	if respData == nil {
		respData = new(resp)
	}

	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(respData); err != nil {
		return err
	}

	if c.Debug {
		log.Printf("<< %v %v", req.Method, req.URL.Path)
		log.Printf("%#v", respData)
	}

	if maybeError, ok := respData.(maybeError); ok {
		if err := maybeError.Err(); err != nil {
			log.Printf("request failed: %v %v: %v", req.Method, req.URL.String(), err)
			return err
		}
	}
	return nil
}
