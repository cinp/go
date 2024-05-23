package cinp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type CInPClient interface {
	SetHeader(name string, value string)
	ClearHeader(name string)
	RegisterType(uri string, objectType reflect.Type)
	Describe(ctx context.Context, uri string) (*Describe, string, error)
	List(ctx context.Context, uri string, filterName string, filterValues map[string]interface{}, position int, count int) ([]string, int, int, int, error)
	ListIds(ctx context.Context, uri string, filterName string, filterValues map[string]interface{}, chunkSize int) <-chan string
	ListObjects(ctx context.Context, uri string, objectType reflect.Type, filterName string, filterValues map[string]interface{}, chunkSize int) <-chan *Object
	Get(ctx context.Context, uri string) (*Object, error)
	Create(ctx context.Context, uri string, object Object) (*Object, error)
	Update(ctx context.Context, object Object) (*Object, error)
	UpdateMulti(ctx context.Context, uri string, values *map[string]interface{}, result *map[string]Object) error
	Delete(ctx context.Context, object Object) error
	DeleteURI(ctx context.Context, uri string) error
	Call(ctx context.Context, uri string, args *map[string]interface{}, result interface{}) error
	CallMulti(ctx context.Context, uri string, args *map[string]interface{}) (*map[string]map[string]interface{}, error)
	ExtractIds(uriList []string) ([]string, error)
	Split(uri string) ([]string, string, string, []string, bool, error)
	UpdateIDs(uri string, ids []string) (string, error)
}

// CInP client struct
type CInP struct {
	host         string
	uri          *URI
	proxy        string
	headers      map[string]string
	typeRegistry map[string]reflect.Type
	log          *slog.Logger
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

// InvalidRequest is a error that is returned when the request is Invalid, this needs to be updated to match the rust/python clients
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
func NewCInP(log *slog.Logger, host string, rootPath string, proxy string) (*CInP, error) {
	if !(strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://")) {
		return nil, errors.New("host does not start with http(s)://")
	}

	if strings.HasSuffix(host, "/") {
		return nil, errors.New("host name must not end with '/'")
	}

	uri, err := NewURI(rootPath)
	if err != nil {
		return nil, err
	}

	cinp := CInP{}
	cinp.host = host
	cinp.uri = uri
	cinp.proxy = proxy
	cinp.typeRegistry = map[string]reflect.Type{}
	cinp.headers = map[string]string{}
	cinp.log = log

	cinp.log.Info("New client", "host", host)

	return &cinp, nil
}

// SetHeader sets a request header
func (cinp *CInP) SetHeader(name string, value string) {
	cinp.log.Debug("Set Header", "name", name)
	cinp.headers[name] = value
}

// ClearHeader sets a request header
func (cinp *CInP) ClearHeader(name string) {
	cinp.log.Debug("Clearing Header", "name", name)
	delete(cinp.headers, name)
}

func (cinp *CInP) request(ctx context.Context, verb string, uri string, dataIn interface{}, dataOut interface{}, headers map[string]string) (int, map[string]string, error) {
	var body []byte

	cinp.log.Debug("request", "extra headers", headers)

	if dataIn != nil {
		var err error
		body, err = marshalJSON(dataIn)
		if err != nil {
			return 0, nil, err
		}
	}

	if len(body) > 500 {
		bodyCopy := make([]byte, 500)
		_ = copy(bodyCopy, body[0:500])
		cinp.log.Debug("request", slog.Any("data", append(bodyCopy, []byte("...")...)))
	} else {
		cinp.log.Debug("request", slog.Any("data", body))
	}

	client := http.Client{
		Timeout: time.Second * 30,
	}

	req, err := http.NewRequest(verb, cinp.host+uri, bytes.NewBuffer(body))
	if err != nil {
		return 0, nil, err
	}

	req = req.WithContext(ctx)

	for k, v := range cinp.headers { // this must go first so the semi-untrusted "user" dosen't mess with the important stuff
		req.Header.Set(k, v)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("User-Agent", "golang CInP client")
	req.Header.Set("Accepts", "application/json")
	req.Header.Set("Accept-Charset", "utf-8")
	req.Header.Set("CInP-Version", "1.0")
	req.Header.Set("Content-Type", "application/json;charset=utf-8")

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	cinp.log.Debug("result", slog.Int("code", res.StatusCode))

	logReader := NewReaderForLogging(500)
	bodyReader := io.TeeReader(res.Body, logReader)

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

		err = json.NewDecoder(bodyReader).Decode(&resultData)
		if err != nil && err.Error() != "EOF" {
			return 0, nil, fmt.Errorf("unable to parse response '%s' with code '%d'", err, res.StatusCode)
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

	if dataOut != nil {
		err = json.NewDecoder(bodyReader).Decode(dataOut)
		if err != nil && err.Error() != "EOF" {
			return 0, nil, fmt.Errorf("unable to parse response '%s'", err)
		}
	}

	resultHeaders := make(map[string]string)
	for _, v := range []string{"Position", "Count", "Total", "Type", "Multi-Object", "Object-Id", "verb"} {
		resultHeaders[v] = res.Header.Get(v)
	}

	cinp.log.Debug("result", "headers", resultHeaders)
	cinp.log.Debug("result", slog.Any("data", logReader.LogValue()))

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
func (cinp *CInP) Describe(ctx context.Context, uri string) (*Describe, string, error) {
	result := &Describe{}
	cinp.log.Info("DESCRIBE", "uri", uri)

	code, headers, err := cinp.request(ctx, "DESCRIBE", uri, nil, result, nil)
	if err != nil {
		return nil, "", err
	}

	if code != 200 {
		return nil, "", fmt.Errorf("unexpected HTTP Code '%d' for DESCRIBE", code)
	}

	return result, headers["Type"], nil
}

// Object used for handeling objects
type Object interface {
	GetURI() string
	SetURI(string)
}

// BaseObject is
type BaseObject struct {
	uri string `json:"-"`
}

// GetURI return the URI of the object
func (baseobject *BaseObject) GetURI() string {
	return baseobject.uri
}

// SetURI sets the URI of the object
func (baseobject *BaseObject) SetURI(uri string) {
	baseobject.uri = uri
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
func (cinp *CInP) List(ctx context.Context, uri string, filterName string, filterValues map[string]interface{}, position int, count int) ([]string, int, int, int, error) {
	result := []string{}
	if position < 0 || count < 0 {
		return nil, 0, 0, 0, fmt.Errorf("position and count must be greater than 0")
	}

	headers := map[string]string{"Position": strconv.Itoa(position), "Count": strconv.Itoa(count)}
	if filterName != "" {
		headers["Filter"] = filterName
	}

	cinp.log.Info("LIST", "uri", uri)

	code, headers, err := cinp.request(ctx, "LIST", uri, &filterValues, &result, headers)
	if err != nil {
		return nil, 0, 0, 0, err
	}

	if code != 200 {
		return nil, 0, 0, 0, fmt.Errorf("unexpected HTTP code '%d'", code)
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
func (cinp *CInP) ListIds(ctx context.Context, uri string, filterName string, filterValues map[string]interface{}, chunkSize int) <-chan string {
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
			items, position, count, total, err = cinp.List(ctx, uri, filterName, filterValues, position, chunkSize)
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
func (cinp *CInP) ListObjects(ctx context.Context, uri string, objectType reflect.Type, filterName string, filterValues map[string]interface{}, chunkSize int) <-chan *Object {
	if chunkSize < 1 { // TODO: if chunkSize > max-ids  set chunkSize = max-ids
		chunkSize = 50
	}
	ch := make(chan *Object)
	go func() {
		defer close(ch)
		var itemList []string
		var count int
		var err error
		position := 0
		total := 1
		for position < total {
			itemList, position, count, total, err = cinp.List(ctx, uri, filterName, filterValues, position, chunkSize)
			if err != nil {
				// not sure what to do with the error
				break
			}
			ids, err := cinp.ExtractIds(itemList)
			if err != nil {
				// not sure what to do with the error
				break
			}
			// TODO: be more effecient and use GetMulti
			//       My golang fu is not good enough to figure out how to make, return, pass, and iterate over a map made with refelect.Type
			//       perhaps there is another way to get it to work, for now do this very ugly get one at a time mess
			for _, id := range ids {
				object, err := cinp.Get(ctx, uri+":"+id+":")
				if err != nil {
					// not sure what to do with the error
					break
				}
				ch <- object
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
func (cinp *CInP) Get(ctx context.Context, uri string) (*Object, error) {
	var err error
	var code int
	var headers map[string]string

	cinp.log.Info("GET", "uri", uri)

	result := cinp.newObject(uri)
	if mo, ok := result.(*MappedObject); ok {
		code, headers, err = cinp.request(ctx, "GET", uri, nil, &mo.Data, nil)
	} else {
		code, headers, err = cinp.request(ctx, "GET", uri, nil, result, nil)
	}
	if err != nil {
		return nil, err
	}

	if code != 200 {
		return nil, fmt.Errorf("unexpected HTTP code '%d'", code)
	}

	if headers["Multi-Object"] == httpTrue {
		return nil, fmt.Errorf("detected multi object")
	}

	result.SetURI(uri)

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
func (cinp *CInP) Create(ctx context.Context, uri string, object Object) (*Object, error) {
	cinp.log.Info("CREATE", "uri", uri)

	code, headers, err := cinp.request(ctx, "CREATE", uri, object, object, nil)
	if err != nil {
		return nil, err
	}

	if code != 201 {
		return nil, fmt.Errorf("unexpected HTTP code '%d'", code)
	}

	_, _, _, ids, _, err := cinp.Split(headers["Object-Id"])
	if err != nil {
		return nil, err
	}

	if ids != nil && len(ids) != 1 {
		return nil, fmt.Errorf("Create did not create any/one object")
	}

	object.SetURI(headers["Object-Id"])

	return &object, nil
}

// Update sends the values of the object to be updated, if the Multi-Object header is set on the result, this will error out.
// NOTE: the updated values the server sends back will  be pushed into the object
func (cinp *CInP) Update(ctx context.Context, object Object) (*Object, error) {
	cinp.log.Info("UPDATE", "object", object.GetURI())

	code, headers, err := cinp.request(ctx, "UPDATE", object.GetURI(), object, object, nil)
	if err != nil {
		return nil, err
	}

	if code != 200 {
		return nil, fmt.Errorf("unexpected HTTP code '%d'", code)
	}

	if headers["Multi-Object"] == httpTrue {
		return nil, fmt.Errorf("detected multi object")
	}

	return &object, nil
}

// UpdateMulti update the objects with the values, forces the Muti-Object header
func (cinp *CInP) UpdateMulti(ctx context.Context, uri string, values *map[string]interface{}, result *map[string]Object) error {
	headers := map[string]string{"Multi-Object": "True"}

	cinp.log.Info("UPDATE(multi)", "uri", uri)

	code, headers, err := cinp.request(ctx, "UPDATE", uri, values, result, headers)
	if err != nil {
		return err
	}

	if code != 200 {
		return fmt.Errorf("unexpected HTTP code '%d'", code)
	}

	if headers["Multi-Object"] != httpTrue {
		return fmt.Errorf("no multi result detected")
	}

	return nil
}

// Delete the object
func (cinp *CInP) Delete(ctx context.Context, object Object) error {

	cinp.log.Info("DELETE", "object", object.GetURI())

	code, _, err := cinp.request(ctx, "DELETE", object.GetURI(), nil, nil, nil)
	if err != nil {
		return err
	}

	if code != 200 {
		return fmt.Errorf("unexpected HTTP code '%d'", code)
	}

	return nil
}

// DeleteURI the object(s) from the URI
func (cinp *CInP) DeleteURI(ctx context.Context, uri string) error {

	cinp.log.Info("DELETE", "uri", uri)

	code, _, err := cinp.request(ctx, "DELETE", uri, nil, nil, nil)
	if err != nil {
		return err
	}

	if code != 200 {
		return fmt.Errorf("unexpected HTTP code '%d'", code)
	}

	return nil
}

// Call calls an object/class method from the URI, if the Multi-Object header is set on the result, this will error out
func (cinp *CInP) Call(ctx context.Context, uri string, args *map[string]interface{}, result interface{}) error {
	cinp.log.Info("CALL", "uri", uri)

	code, headers, err := cinp.request(ctx, "CALL", uri, args, result, nil)
	if err != nil {
		return err
	}

	if code != 200 {
		return fmt.Errorf("unexpected HTTP code '%d'", code)
	}

	if headers["Multi-Object"] == httpTrue {
		return fmt.Errorf("detected multi object")
	}

	return nil
}

// CallMulti calls an object/class method from the URI, forces the Muti-Object header
func (cinp *CInP) CallMulti(ctx context.Context, uri string, args *map[string]interface{}) (*map[string]map[string]interface{}, error) {
	result := map[string]map[string]interface{}{}
	headers := map[string]string{"Multi-Object": "True"}
	cinp.log.Info("CALL(multi)", "uri", uri)
	code, headers, err := cinp.request(ctx, "CALL", uri, args, &result, headers)
	if err != nil {
		return nil, err
	}

	if code != 200 {
		return nil, fmt.Errorf("unexpected HTTP code '%d'", code)
	}

	if headers["Multi-Object"] != httpTrue {
		return nil, fmt.Errorf("no multi result detected")
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
		return nil, errors.New("root path must start and end with '/'")
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
		return nil, "", "", nil, false, fmt.Errorf("unable to parse URI '%s'", uri)
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

	if len(ids) > 0 {
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
			return nil, fmt.Errorf("unable to parse URI '%s'", v)
		}
		if groups[5] != "" {
			result = append(result, strings.Split(strings.Trim(groups[5], ":"), ":")...)
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

func marshalJSON(t interface{}) ([]byte, error) { // b/c the standard library turns on HTML escaping by default.... why?
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(t)
	return buffer.Bytes(), err
}

type ReaderForLogging struct {
	cap  int
	buff *bytes.Buffer
}

func NewReaderForLogging(cap int) *ReaderForLogging {
	return &ReaderForLogging{buff: new(bytes.Buffer), cap: cap}
}

// Write to buffer, if we are full ignore the more data
func (r *ReaderForLogging) Write(buff []byte) (int, error) {
	if r.buff.Len() >= r.cap {
		return 0, nil
	}
	toRead := min(len(buff), (r.cap - r.buff.Len()))
	r.buff.Write(buff[0:toRead])
	return toRead, nil
}

func (r *ReaderForLogging) LogValue() []byte {
	if r.buff.Len() >= r.cap {
		return append(r.buff.Bytes(), []byte("...")...)
	}
	return r.buff.Bytes()
}
