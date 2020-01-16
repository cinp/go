package cinp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// CInP client struct
type CInP struct {
	host      string
	uri       *URI
	proxy     string
	authID    string
	authToken string
}

// InvalidSession is a error that is returned when the AuthId and AuthToken do not specifiy a valid session
type InvalidSession struct{}

func (e *InvalidSession) Error() string { return "Invalid Session" }

// NotAuthorized is a error that is returned when the Session is not Authorized to make the request
type NotAuthorized struct{}

func (e *NotAuthorized) Error() string { return "Not Authorized" }

// NotFound is a error that is returned when the request referes to a namespace/model/object/action that does not exist
type NotFound struct{}

func (e *NotFound) Error() string { return "Not Found" }

// InvalidRequest is a error that is returned when the request is Invalid
type InvalidRequest struct {
	msg string
}

func (e *InvalidRequest) Error() string { return fmt.Sprintf("Invalid Request: '%s'", e.msg) }

// ServerError is a error that is returned when the request causes a ServerError
type ServerError struct {
	msg   string
	trace string // should be []string ?
}

func (e *ServerError) Error() string {
	if e.trace != "" {
		return fmt.Sprintf("Server Error: '%s' at '%s'", e.msg, e.trace)
	} else {
		return fmt.Sprintf("Server Error: '%s'", e.msg)
	}
}

// NewCInP creates a new cinp instance
func NewCInP(host string, rootPath string, proxy string) (*CInP, error) {
	if !(strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://")) {
		return nil, errors.New("host does not start with http(s)://")
	}

	if strings.HasSuffix(host, "/") {
		return nil, errors.New("host name must not end with '/'")
	}

	u, err := NewURI(rootPath)
	if err != nil {
		return nil, err
	}

	c := new(CInP)
	c.host = host
	c.uri = u
	c.proxy = proxy
	c.authID = ""
	c.authToken = ""

	return c, nil
}

// SetAuth sets the auth info, set authId to "" to clear
func (c *CInP) SetAuth(authID string, authToken string) {
	c.authID = authID
	c.authToken = authToken
}

func (c *CInP) request(verb string, uri string, data map[string]interface{}, headers map[string]string) (int, map[string]interface{}, map[string]string, error) {
	var body []byte

	if data != nil {
		var err error
		body, err = json.Marshal(data)
		if err != nil {
			return 0, nil, nil, err
		}
	}

	client := http.Client{
		Timeout: time.Second * 30,
	}

	req, err := http.NewRequest(verb, c.host+uri, bytes.NewBuffer(body))
	if err != nil {
		return 0, nil, nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("User-Agent", "golang CInP client")
	req.Header.Set("Accepts", "application/json")
	req.Header.Set("Accept-Charset", "utf-8")
	req.Header.Set("CInP-Version", "0.9")
	if c.authID != "" {
		req.Header.Set("Auth-Id", c.authID)
		req.Header.Set("Auth-Token", c.authToken)
	}
	req.Header.Set("Content-Type", "application/json;charset=utf-8")

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}

	switch res.StatusCode {
	case 401:
		return 0, nil, nil, &InvalidSession{}
	case 403:
		return 0, nil, nil, &NotAuthorized{}
	case 404:
		return 0, nil, nil, &NotFound{}
	case 200, 201, 202, 400, 500:
	default:
		return 0, nil, nil, fmt.Errorf("HTTP Code '%d' unhandled", res.StatusCode)
	}

	var resultData map[string]interface{}
	resultHeaders := make(map[string]string)
	// So some 400 and 500 responses might not be JSON encoded, other than saving the
	// body to the side in the case of Decode error, not sure what else to do.  When a
	// solution is found, see cinp/python/client.py for how to deal with the not JSON
	// responses

	err = json.NewDecoder(res.Body).Decode(&resultData)
	if err != nil && err.Error() != "EOF" {
		return 0, nil, nil, fmt.Errorf("Unable to parse response '%s'", err)
	}

	for _, v := range []string{"Position", "Count", "Total", "Type", "Multi-Object", "Object-Id", "verb"} {
		resultHeaders[v] = res.Header.Get(v)
	}

	if res.StatusCode == 400 {
		message, ok := resultData["message"]
		if ok {
			return 0, nil, nil, &InvalidRequest{message.(string)}
		}
		return 0, nil, nil, &InvalidRequest{fmt.Sprintf("%v", resultData)}
	}

	if res.StatusCode == 500 {
		if _, ok := resultData["message"]; ok {
			if _, ok := resultData["trace"]; ok {
				return 0, nil, nil, &ServerError{resultData["message"].(string), fmt.Sprintf("%v", resultData["trace"])}
			}
			return 0, nil, nil, &ServerError{resultData["message"].(string), ""}
		}
		return 0, nil, nil, &ServerError{fmt.Sprintf("%v", resultData), ""}
	}

	return res.StatusCode, resultData, resultHeaders, nil
}

// Describe structu definiation
type Describe struct {
	Type string
	Name string `json:"name"`
	Doc  string `json:"doc"`
	Path string `json:"path"`
	// Field/Paramater
	// Length         int         `length`
	// URI            string      `uri`
	// AllowedSchemes []string    `allowed_schemes`
	// Choices        []string    `choices`
	// IsArray        bool        `is_array`
	// Default        interface{} `default`
	// Mode           string  `mode`
	// Required       bool    `required`
	// Namespace
	APIVersion  string   `json:"api-version"`
	MultiURIMax int      `json:"multi-uri-max"`
	Namespaces  []string `json:"namespaces"`
	Models      []string `json:"models"`
	// Model
	Constants         []string `json:constants`
	Fields            []string `json:fields`
	Actions           []string `json:actions`
	NotAllowedMethods []string `json:not-allowed-metods`
	ListFilters       []string `json:list-filters`
	// Actions
	// ReturnType string   `return-type`
	// Static     bool     `static`
	// Paramaters []string `paramaters`
}

// Describe the URI
func (c *CInP) Describe(uri string) (*Describe, error) {
	code, data, headers, err := c.request("DESCRIBE", uri, nil, nil)
	if err != nil {
		return nil, err
	}

	if code != 200 {
		return nil, fmt.Errorf("Unexpected HTTP Code '%d' for DESCRIBE", code)
	}

	result := &Describe{Type: headers["Type"]}

	if v, ok := data["name"]; ok {
		result.Name = v.(string)
	}

	if v, ok := data["doc"]; ok {
		result.Doc = v.(string)
	}

	if v, ok := data["path"]; ok {
		result.Path = v.(string)
	}

	if v, ok := data["api-version"]; ok {
		result.APIVersion = v.(string)
	}

	if v, ok := data["multi-uri-max"]; ok {
		result.MultiURIMax = int(v.(float64))
	}

	if v, ok := data["namespaces"]; ok {
		item := v.([]interface{})
		result.Namespaces = make([]string, len(item))
		for k := range item {
			result.Namespaces[k] = item[k].(string)
		}
	}

	return result, nil
}

// URI is for parsing and checking CNiP URIs
type URI struct {
	rootPath string
	uriRegex *regexp.Regexp
}

// NewURI creates and initilizes a URI instance
func NewURI(rootPath string) (*URI, error) {
	if rootPath == "" || rootPath[len(rootPath)-1] != '/' || rootPath[0] != '/' {
		return nil, errors.New("rootPath must start and end with '/'")
	}

	r, err := regexp.Compile("^(" + rootPath + ")(([a-zA-Z0-9\\-_.!~*]+/)*)([a-zA-Z0-9\\-_.!~*]+)?(:([a-zA-Z0-9\\-_.!~*\\']*:)*)?(\\([a-zA-Z0-9\\-_.!~*]+\\))?$")
	if err != nil {
		return nil, err
	}

	u := new(URI)
	u.rootPath = rootPath
	u.uriRegex = r

	return u, nil
}

// Split the uri into it's parts
func (u *URI) Split(uri string) ([]string, string, string, []string, bool, error) {
	groups := u.uriRegex.FindStringSubmatch(uri)
	if len(groups) < 8 {
		return nil, "", "", nil, false, fmt.Errorf("Unable to parse URI '%s'", uri)
	}

	//( _, root, namespace, _, model, rec_id, _, action ) = groups
	if groups[1] != u.rootPath {
		return nil, "", "", nil, false, errors.New("URI does not start in the rootPath")
	}

	var namespaceList []string
	if groups[2] != "" {
		namespaceList = strings.Split(strings.Trim(groups[2], "/"), "/")
	}

	var idList []string
	var multi bool
	if groups[5] != "" {
		idList = strings.Split(strings.Trim(groups[5], ":"), ":")
		multi = len(idList) > 1
	} else {
		idList = nil // id_list = [] is an empty list of ids, where nil means the list is not even present
		multi = false
	}

	var action string
	if groups[7] != "" {
		action = groups[7][1 : len(groups[7])-1]
	}

	return namespaceList, groups[4], action, idList, multi, nil
}

// Build  constructs a URI from the paramaters, NOTE: if model is "", idList and action are skiped
func (u *URI) Build(namespace []string, model string, action string, idList []string) string {
	result := u.rootPath

	if len(namespace) > 0 {
		result += strings.Join(namespace, "/") + "/"
	}

	if model == "" {
		return result
	}

	result += model

	if idList != nil && len(idList) > 0 {
		result += ":" + strings.Join(idList, ":") + ":"
	}

	if action != "" {
		result += "(" + action + ")"
	}

	return result
}
