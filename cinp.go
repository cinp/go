package cinp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// CInP client struct
type CInP struct {
	host         string
	uri          *URI
	proxy        string
	authID       string
	authToken    string
	typeRegistry map[string]reflect.Type
}

const httpTrue = "True"

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
	}
	return fmt.Sprintf("Server Error: '%s'", e.msg)
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

	cinp := CInP{}
	cinp.host = host
	cinp.uri = u
	cinp.proxy = proxy
	cinp.authID = ""
	cinp.authToken = ""
	cinp.typeRegistry = map[string]reflect.Type{}

	return &cinp, nil
}

// SetAuth sets the auth info, set authId to "" to clear
func (cinp *CInP) SetAuth(authID string, authToken string) {
	cinp.authID = authID
	cinp.authToken = authToken
}

// IsAuthenticated return true if the authID is set
func (cinp *CInP) IsAuthenticated() bool {
	return cinp.authID != ""
}

func (cinp *CInP) request(verb string, uri string, data *map[string]interface{}, result interface{}, headers map[string]string) (int, map[string]string, error) {
	var body []byte

	if data != nil {
		var err error
		body, err = json.Marshal(data)
		if err != nil {
			return 0, nil, err
		}
	}

	client := http.Client{
		Timeout: time.Second * 30,
	}

	req, err := http.NewRequest(verb, cinp.host+uri, bytes.NewBuffer(body))
	if err != nil {
		return 0, nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("User-Agent", "golang CInP client")
	req.Header.Set("Accepts", "application/json")
	req.Header.Set("Accept-Charset", "utf-8")
	req.Header.Set("CInP-Version", "1.0")
	if cinp.authID != "" {
		req.Header.Set("Auth-Id", cinp.authID)
		req.Header.Set("Auth-Token", cinp.authToken)
	}
	req.Header.Set("Content-Type", "application/json;charset=utf-8")

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	switch res.StatusCode {
	case 401:
		return 0, nil, &InvalidSession{}
	case 403:
		return 0, nil, &NotAuthorized{}
	case 404:
		return 0, nil, &NotFound{}
	case 200, 201, 202, 400, 500:
	default:
		return 0, nil, fmt.Errorf("HTTP Code '%d' unhandled", res.StatusCode)
	}

	if res.StatusCode == 400 || res.StatusCode == 500 {
		// So some 400 and 500 responses might not be JSON encoded, other than saving the
		// body to the side in the case of Decode error, not sure what else to do.  When a
		// solution is found, see cinp/python/client.py for how to deal with the not JSON
		// responses

		var resultData map[string]interface{}

		err = json.NewDecoder(res.Body).Decode(&resultData)
		if err != nil && err.Error() != "EOF" {
			return 0, nil, fmt.Errorf("Unable to parse response '%s' with code '%d'", err, res.StatusCode)
		}

		if res.StatusCode == 400 {
			message, ok := resultData["message"]
			if ok {
				return 0, nil, &InvalidRequest{message.(string)}
			}
			return 0, nil, &InvalidRequest{fmt.Sprintf("%v", resultData)}
		}

		// HTTP 500
		if _, ok := resultData["message"]; ok {
			if _, ok := resultData["trace"]; ok {
				return 0, nil, &ServerError{resultData["message"].(string), fmt.Sprintf("%v", resultData["trace"])}
			}
			return 0, nil, &ServerError{resultData["message"].(string), ""}
		}
		return 0, nil, &ServerError{fmt.Sprintf("%v", resultData), ""}
	}

	resultHeaders := make(map[string]string)

	err = json.NewDecoder(res.Body).Decode(result)
	if err != nil && err.Error() != "EOF" {
		return 0, nil, fmt.Errorf("Unable to parse response '%s'", err)
	}

	for _, v := range []string{"Position", "Count", "Total", "Type", "Multi-Object", "Object-Id", "verb"} {
		resultHeaders[v] = res.Header.Get(v)
	}

	return res.StatusCode, resultHeaders, nil
}

// FieldParamater defines a Field or Paramater from the describe
type FieldParamater struct {
	Name           string        `json:"name"`
	Doc            string        `json:"doc"`
	Path           string        `json:"path"`
	Type           string        `json:"type"`
	Length         int           `json:"length"`
	URI            string        `json:"uri"`
	AllowedSchemes []string      `json:"allowed_schemes"`
	Choices        []interface{} `json:"choices"`
	IsArray        bool          `json:"is_array"`
	Default        interface{}   `json:"default"`
	Mode           string        `json:"mode"`
	Required       bool          `json:"required"`
}

// Describe struct definiation
type Describe struct {
	Name string `json:"name"`
	Doc  string `json:"doc"`
	Path string `json:"path"`
	// Namespace
	APIVersion  string   `json:"api-version"`
	MultiURIMax int      `json:"multi-uri-max"`
	Namespaces  []string `json:"namespaces"`
	Models      []string `json:"models"`
	// Model
	Constants         map[string]string           `json:"constants"`
	Fields            []FieldParamater            `json:"fields"`
	Actions           []string                    `json:"actions"`
	NotAllowedMethods []string                    `json:"not-allowed-methods"`
	ListFilters       map[string][]FieldParamater `json:"list-filters"`
	// Actions
	ReturnType FieldParamater   `json:"return-type"`
	Static     bool             `json:"static"`
	Paramaters []FieldParamater `json:"paramaters"`
}

// Describe the URI
func (cinp *CInP) Describe(uri string) (*Describe, string, error) {
	result := &Describe{}
	code, headers, err := cinp.request("DESCRIBE", uri, nil, result, nil)
	if err != nil {
		return nil, "", err
	}

	if code != 200 {
		return nil, "", fmt.Errorf("Unexpected HTTP Code '%d' for DESCRIBE", code)
	}

	return result, headers["Type"], nil
}

// Object used for handeling objects
type Object interface {
	GetID() string
	SetID(string)
	AsMap(bool) *map[string]interface{}
}

// BaseObject is
type BaseObject struct {
	id string
}

// GetID return the id of the object
func (baseobject *BaseObject) GetID() string {
	return baseobject.id
}

// SetID set the id of the object
func (baseobject *BaseObject) SetID(id string) {
	baseobject.id = id
}

// MappedObject for generic Object Manipluation
type MappedObject struct {
	BaseObject
	Data map[string]interface{}
}

// AsMap exports the Object's Data as a map
func (mo *MappedObject) AsMap(isCreate bool) *map[string]interface{} {
	return &mo.Data
}

// MappedObjectType is the type used for the MappedObject which is used if a uri is not found in the type table
var MappedObjectType = reflect.TypeOf((*MappedObject)(nil)).Elem()

// RegisterType registeres the type to use for a url
func (cinp *CInP) RegisterType(uri string, objectType reflect.Type) {
	object := reflect.New(objectType).Interface()
	_, ok := object.(Object)
	if !ok {
		panic(fmt.Sprintf("%v does not implement Object", objectType))
	}

	cinp.typeRegistry[uri] = objectType
}

func (cinp *CInP) objectType(uri string) reflect.Type {
	offset := strings.IndexByte(uri, ':')
	if offset != -1 {
		uri = uri[:offset]
	}

	objectType, ok := cinp.typeRegistry[uri]
	if !ok {
		return MappedObjectType
	}

	return objectType
}

func (cinp *CInP) newObject(uri string) Object {
	objectType := cinp.objectType(uri)

	return reflect.New(objectType).Interface().(Object)
}

// List objects
func (cinp *CInP) List(uri string, filterName string, filterValues map[string]interface{}, position int, count int) ([]string, int, int, int, error) {
	result := []string{}
	if position < 0 || count < 0 {
		return nil, 0, 0, 0, fmt.Errorf("Position and count must be greater than 0")
	}

	headers := map[string]string{"Position": strconv.Itoa(position), "Count": strconv.Itoa(count)}
	if filterName != "" {
		headers["Filter"] = filterName
	}

	code, headers, err := cinp.request("LIST", uri, &filterValues, &result, headers)
	if err != nil {
		return nil, 0, 0, 0, err
	}

	if code != 200 {
		return nil, 0, 0, 0, fmt.Errorf("Unexpected HTTP code '%d'", code)
	}
	var total int
	if position, err = strconv.Atoi(headers["Position"]); err != nil {
		return nil, 0, 0, 0, err
	}
	if count, err = strconv.Atoi(headers["Count"]); err != nil {
		return nil, 0, 0, 0, err
	}
	if total, err = strconv.Atoi(headers["Total"]); err != nil {
		return nil, 0, 0, 0, err
	}

	return result, position, count, total, nil
}

// ListIds List Objects and return in a channel
func (cinp *CInP) ListIds(uri string, filterName string, filterValues map[string]interface{}, chunkSize int) <-chan string {
	if chunkSize < 1 {
		chunkSize = 50
	}
	ch := make(chan string)
	go func() {
		defer close(ch)
		var items []string
		var count int
		var err error
		position := 0
		total := 1
		for position < total {
			items, position, count, total, err = cinp.List(uri, filterName, filterValues, position, chunkSize)
			if err != nil {
				// not sure what to do with the error
				break
			}
			for _, v := range items {
				ch <- v
			}
			position += count
		}
	}()
	return ch
}

// ListObjects List Objects and return in a channel
func (cinp *CInP) ListObjects(uri string, objectType reflect.Type, filterName string, filterValues map[string]interface{}, chunkSize int) <-chan Object {
	if chunkSize < 1 { // TODO: if chunkSize > max-ids  set chunkSize = max-ids
		chunkSize = 50
	}
	ch := make(chan Object)
	go func() {
		defer close(ch)
		var itemList []string
		var count int
		var err error
		position := 0
		total := 1
		for position < total {
			itemList, position, count, total, err = cinp.List(uri, filterName, filterValues, position, chunkSize)
			if err != nil {
				// not sure what to do with the error
				break
			}
			ids, err := cinp.ExtractIds(itemList)
			if err != nil {
				// not sure what to do with the error
				break
			}
			// TODO: bemore effecient and use GetMulti
			//       My golang fu is not good enough to figure out how to make, return, pass, and iterate over a map made with refelect.Type
			//       perhaps there is another way to get it to work, for now do this very ugly get one at a time mess
			for _, id := range ids {
				object, err := cinp.Get(uri + ":" + id + ":")
				if err != nil {
					// not sure what to do with the error
					break
				}
				ch <- *object
			}
			// fmt.Println(ids)
			// objList, err := cinp.GetMulti(uri + ":" + strings.Join(ids, ":") + ":")
			// fmt.Println(err)
			// if err != nil {
			// 	// not sure what to do with the error
			// 	break
			// }
			// for _, v := range *objList {
			// 	ch <- v
			// }
			position += count
		}
	}()
	return ch
}

// Get gets an object from the URI, if the Multi-Object header is set on the result, this will error out
func (cinp *CInP) Get(uri string) (*Object, error) {
	var err error
	var code int
	var headers map[string]string

	result := cinp.newObject(uri)
	if mo, ok := result.(*MappedObject); ok {
		code, headers, err = cinp.request("GET", uri, nil, &mo.Data, nil)
	} else {
		code, headers, err = cinp.request("GET", uri, nil, result, nil)
	}
	if err != nil {
		return nil, err
	}

	if code != 200 {
		return nil, fmt.Errorf("Unexpected HTTP code '%d'", code)
	}

	if headers["Multi-Object"] == httpTrue {
		return nil, fmt.Errorf("Detected multi object")
	}

	result.SetID(uri)

	return &result, nil
}

// GetMulti get objects from the URI, forces the Muti-Object header
// func (cinp *CInP) GetMulti(uri string) (*map[string]Object, error) {
// 	headers := map[string]string{"Multi-Object": "True"}
// 	mapType := reflect.MapOf(reflect.TypeOf(""), cinp.objectType(uri))
// 	result := reflect.MakeMap(mapType).Interface()
// 	code, headers, err := cinp.request("GET", uri, nil, result, headers)
// 	fmt.Printf("3  %+v\n", result)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	if code != 200 {
// 		return nil, fmt.Errorf("Unexpected HTTP code '%d'", code)
// 	}
//
// 	if headers["Multi-Object"] != httpTrue {
// 		return nil, fmt.Errorf("None Multi result detected")
// 	}
//
// 	//return result.(map[string]Object), nil
// 	return &map[string]Object{}, nil
// }

// Create an object with the values
func (cinp *CInP) Create(uri string, object Object) error {
	values := object.AsMap(true)
	code, headers, err := cinp.request("CREATE", uri, values, object, nil)
	if err != nil {
		return err
	}

	if code != 201 {
		return fmt.Errorf("Unexpected HTTP code '%d'", code)
	}

	_, _, _, ids, _, err := cinp.Split(headers["Object-Id"])
	if err != nil {
		return err
	}

	if ids != nil && len(ids) != 1 {
		return fmt.Errorf("Create did not create any/one object")
	}

	object.SetID(headers["Object-Id"])

	return nil
}

// Update sends the values of the object to be updated, if the Multi-Object header is set on the result, this will error out.
// NOTE: the updated values the server sends back will  be pushed into the object
func (cinp *CInP) Update(object Object, fieldList []string) error {
	values := object.AsMap(false)
	if fieldList != nil {
	top:
		for k := range *values {
			for _, v := range fieldList {
				if k == v {
					continue top
				}
			}
			delete(*values, k)
		}
	}
	code, headers, err := cinp.request("UPDATE", object.GetID(), values, object, nil)
	if err != nil {
		return err
	}

	if code != 200 {
		return fmt.Errorf("Unexpected HTTP code '%d'", code)
	}

	if headers["Multi-Object"] == httpTrue {
		return fmt.Errorf("Detected multi object")
	}

	return nil
}

// UpdateMulti update the objects with the values, forces the Muti-Object header
func (cinp *CInP) UpdateMulti(uri string, values *map[string]interface{}, result *map[string]Object) error {
	headers := map[string]string{"Multi-Object": "True"}
	code, headers, err := cinp.request("UPDATE", uri, values, result, headers)
	if err != nil {
		return err
	}

	if code != 200 {
		return fmt.Errorf("Unexpected HTTP code '%d'", code)
	}

	if headers["Multi-Object"] != httpTrue {
		return fmt.Errorf("None Multi result detected")
	}

	return nil
}

// Delete the object
func (cinp *CInP) Delete(object Object) error {
	code, _, err := cinp.request("DELETE", object.GetID(), nil, nil, nil)
	if err != nil {
		return err
	}

	if code != 200 {
		return fmt.Errorf("Unexpected HTTP code '%d'", code)
	}

	return nil
}

// DeleteURI the object(s) from the URI
func (cinp *CInP) DeleteURI(uri string) error {
	code, _, err := cinp.request("DELETE", uri, nil, nil, nil)
	if err != nil {
		return err
	}

	if code != 200 {
		return fmt.Errorf("Unexpected HTTP code '%d'", code)
	}

	return nil
}

// Call calls an object/class method from the URI, if the Multi-Object header is set on the result, this will error out
func (cinp *CInP) Call(uri string, args *map[string]interface{}, result interface{}) error {
	code, headers, err := cinp.request("CALL", uri, args, result, nil)
	if err != nil {
		return err
	}

	if code != 200 {
		return fmt.Errorf("Unexpected HTTP code '%d'", code)
	}

	if headers["Multi-Object"] == httpTrue {
		return fmt.Errorf("Detected multi object")
	}

	return nil
}

// CallMulti calls an object/class method from the URI, forces the Muti-Object header
func (cinp *CInP) CallMulti(uri string, args *map[string]interface{}) (*map[string]map[string]interface{}, error) {
	result := map[string]map[string]interface{}{}
	headers := map[string]string{"Multi-Object": "True"}
	code, headers, err := cinp.request("CALL", uri, args, &result, headers)
	if err != nil {
		return nil, err
	}

	if code != 200 {
		return nil, fmt.Errorf("Unexpected HTTP code '%d'", code)
	}

	if headers["Multi-Object"] != httpTrue {
		return nil, fmt.Errorf("None Multi result detected")
	}

	return &result, nil
}

// ExtractIds extract the id(s) from a list of URI
func (cinp *CInP) ExtractIds(uriList []string) ([]string, error) {
	return cinp.uri.ExtractIds(uriList)
}

// Split the uri into it's parts
func (cinp *CInP) Split(uri string) ([]string, string, string, []string, bool, error) {
	return cinp.uri.Split(uri)
}

// UpdateIDs update/set the id(s) in the uri
func (cinp *CInP) UpdateIDs(uri string, ids []string) (string, error) {
	return cinp.uri.UpdateIDs(uri, ids)
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

	var ids []string
	var multi bool
	if groups[5] != "" {
		ids = strings.Split(strings.Trim(groups[5], ":"), ":")
		multi = len(ids) > 1
	} else {
		ids = nil // ids = [] is an empty list of ids, where nil means the list is not even present
		multi = false
	}

	var action string
	if groups[7] != "" {
		action = groups[7][1 : len(groups[7])-1]
	}

	return namespaceList, groups[4], action, ids, multi, nil
}

// Build constructs a URI from the paramaters, NOTE: if model is "", ids and action are skiped
func (u *URI) Build(namespace []string, model string, action string, ids []string) string {
	result := u.rootPath

	if len(namespace) > 0 {
		result += strings.Join(namespace, "/") + "/"
	}

	if model == "" {
		return result
	}

	result += model

	if ids != nil && len(ids) > 0 {
		result += ":" + strings.Join(ids, ":") + ":"
	}

	if action != "" {
		result += "(" + action + ")"
	}

	return result
}

// ExtractIds extract the id(s) from a list of URI
func (u *URI) ExtractIds(uriList []string) ([]string, error) {
	result := make([]string, 0)
	for _, v := range uriList {
		groups := u.uriRegex.FindStringSubmatch(v)
		if len(groups) < 8 {
			return nil, fmt.Errorf("Unable to parse URI '%s'", v)
		}
		if groups[5] != "" {
			for _, v := range strings.Split(strings.Trim(groups[5], ":"), ":") {
				result = append(result, v)
			}
		}
	}

	return result, nil
}

// UpdateIDs update/set the id(s) in the uri
func (u *URI) UpdateIDs(uri string, ids []string) (string, error) {
	ns, model, action, _, _, err := u.Split(uri)
	if err != nil {
		return "", err
	}

	return u.Build(ns, model, action, ids), nil
}
