package cinp

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
)

func getLogger() *slog.Logger {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return logger
}

func TestNewURI(t *testing.T) {
	var goodList = []string{"/api/v1/", "/"}
	for _, v := range goodList {
		u, err := NewURI(v)
		if err != nil {
			t.Errorf("Unexpected error '%s'", err)
			t.FailNow()
		}
		if u.rootPath != v {
			t.Errorf("invalid root path")
			t.FailNow()
		}
	}

	var badList = []string{"/api/v1", "api/v1/", "api/v1", ""}
	for _, v := range badList {
		_, err := NewURI(v)
		if err == nil {
			t.Errorf("error missing")
			t.FailNow()
		}
	}
}

func TestURISplitBuild(t *testing.T) {
	u, err := NewURI("/api/v1/")
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}

	_, _, _, _, _, err = u.Split("/apdsf")
	if err == nil {
		t.Errorf("error missing")
		t.FailNow()
	}

	ns, model, action, idList, multi, err := u.Split("/api/v1/")
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}
	if len(ns) != 0 || model != "" || action != "" || idList != nil || multi != false {
		t.Errorf("Expected [], '', '', nil, false got %s, '%s', '%s', %s, %t", ns, model, action, idList, multi)
		t.FailNow()
	}
	r := u.Build(ns, model, action, idList)
	if r != "/api/v1/" {
		t.Errorf("Expected '/api/v1/' got '%s'", r)
		t.FailNow()
	}
	ns = nil
	r = u.Build(ns, model, action, idList)
	if r != "/api/v1/" {
		t.Errorf("Expected '/api/v1/' got '%s'", r)
		t.FailNow()
	}

	ns, model, action, idList, multi, err = u.Split("/api/v1/ns/")
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}
	if !reflect.DeepEqual(ns, []string{"ns"}) || model != "" || action != "" || idList != nil || multi != false {
		t.Errorf("Expected [ns], '', '', nil, false got %s, '%s', '%s', %s, %t", ns, model, action, idList, multi)
		t.FailNow()
	}
	r = u.Build(ns, model, action, idList)
	if r != "/api/v1/ns/" {
		t.Errorf("Expected '/api/v1/ns/' got '%s'", r)
		t.FailNow()
	}

	ns, model, action, idList, multi, err = u.Split("/api/v1/ns/model")
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}
	if !reflect.DeepEqual(ns, []string{"ns"}) || model != "model" || action != "" || idList != nil || multi != false {
		t.Errorf("Expected [ns], 'model', '', nil, false got %s, '%s', '%s', %s, %t", ns, model, action, idList, multi)
		t.FailNow()
	}
	r = u.Build(ns, model, action, idList)
	if r != "/api/v1/ns/model" {
		t.Errorf("Expected '/api/v1/ns/model' got '%s'", r)
		t.FailNow()
	}
	idList = []string{}
	r = u.Build(ns, model, action, idList)
	if r != "/api/v1/ns/model" {
		t.Errorf("Expected '/api/v1/ns/model' got '%s'", r)
		t.FailNow()
	}

	ns, model, action, idList, multi, err = u.Split("/api/v1/ns/ns2/")
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}
	if !reflect.DeepEqual(ns, []string{"ns", "ns2"}) || model != "" || action != "" || idList != nil || multi != false {
		t.Errorf("Expected [ns ns2], '', '', nil, false got %s, '%s', '%s', %s, %t", ns, model, action, idList, multi)
		t.FailNow()
	}
	r = u.Build(ns, model, action, idList)
	if r != "/api/v1/ns/ns2/" {
		t.Errorf("Expected '/api/v1/ns/ns2/' got '%s'", r)
		t.FailNow()
	}

	ns, model, action, idList, multi, err = u.Split("/api/v1/ns/ns2/model")
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}
	if !reflect.DeepEqual(ns, []string{"ns", "ns2"}) || model != "model" || action != "" || idList != nil || multi != false {
		t.Errorf("Expected [ns ns2], 'model', '', nil, false got %s, '%s', '%s', %s, %t", ns, model, action, idList, multi)
		t.FailNow()
	}
	r = u.Build(ns, model, action, idList)
	if r != "/api/v1/ns/ns2/model" {
		t.Errorf("Expected '/api/v1/ns/ns2/model' got '%s'", r)
		t.FailNow()
	}

	ns, model, action, idList, multi, err = u.Split("/api/v1/ns/model::")
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}
	if !reflect.DeepEqual(ns, []string{"ns"}) || model != "model" || action != "" || !reflect.DeepEqual(idList, []string{""}) || multi != false {
		t.Errorf("Expected [ns], 'model', '', [], false got %s, '%s', '%s', %s, %t", ns, model, action, idList, multi)
		t.FailNow()
	}
	r = u.Build(ns, model, action, idList)
	if r != "/api/v1/ns/model::" {
		t.Errorf("Expected '/api/v1/ns/model::' got '%s'", r)
		t.FailNow()
	}

	ns, model, action, idList, multi, err = u.Split("/api/v1/ns/model:ghj:")
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}
	if !reflect.DeepEqual(ns, []string{"ns"}) || model != "model" || action != "" || !reflect.DeepEqual(idList, []string{"ghj"}) || multi != false {
		t.Errorf("Expected [ns], 'model', '', [ghj], false got %s, '%s', '%s', %s, %t", ns, model, action, idList, multi)
		t.FailNow()
	}
	r = u.Build(ns, model, action, idList)
	if r != "/api/v1/ns/model:ghj:" {
		t.Errorf("Expected '/api/v1/ns/model:ghj:' got '%s'", r)
		t.FailNow()
	}

	ns, model, action, idList, multi, err = u.Split("/api/v1/ns/model:ghj:dsf:sfe:")
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}
	if !reflect.DeepEqual(ns, []string{"ns"}) || model != "model" || action != "" || !reflect.DeepEqual(idList, []string{"ghj", "dsf", "sfe"}) || multi != true {
		t.Errorf("Expected [ns], 'model', '', [ghj dsf sfe], true got %s, '%s', '%s', %s, %t", ns, model, action, idList, multi)
		t.FailNow()
	}
	r = u.Build(ns, model, action, idList)
	if r != "/api/v1/ns/model:ghj:dsf:sfe:" {
		t.Errorf("Expected '/api/v1/ns/model:ghj:dsf:sfe:' got '%s'", r)
		t.FailNow()
	}

	ns, model, action, idList, multi, err = u.Split("/api/v1/ns/model(action)")
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}
	if !reflect.DeepEqual(ns, []string{"ns"}) || model != "model" || action != "action" || idList != nil || multi != false {
		t.Errorf("Expected [ns], 'model', 'action', nil, false got %s, '%s', '%s', %s, %t", ns, model, action, idList, multi)
		t.FailNow()
	}
	r = u.Build(ns, model, action, idList)
	if r != "/api/v1/ns/model(action)" {
		t.Errorf("Expected '/api/v1/ns/model(action)' got '%s'", r)
		t.FailNow()
	}

	ns, model, action, idList, multi, err = u.Split("/api/v1/ns/model:sdf:(action)")
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}
	if !reflect.DeepEqual(ns, []string{"ns"}) || model != "model" || action != "action" || !reflect.DeepEqual(idList, []string{"sdf"}) || multi != false {
		t.Errorf("Expected [ns], 'model', 'action', [sdf], false got %s, '%s', '%s', %s, %t", ns, model, action, idList, multi)
		t.FailNow()
	}
	r = u.Build(ns, model, action, idList)
	if r != "/api/v1/ns/model:sdf:(action)" {
		t.Errorf("Expected '/api/v1/ns/model:sdf:(action)' got '%s'", r)
		t.FailNow()
	}

	ns, model, action, idList, multi, err = u.Split("/api/v1/ns/model:sdf:eed:(action)")
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}
	if !reflect.DeepEqual(ns, []string{"ns"}) || model != "model" || action != "action" || !reflect.DeepEqual(idList, []string{"sdf", "eed"}) || multi != true {
		t.Errorf("Expected [ns], 'model', 'action', [sdf eed], true got %s, '%s', '%s', %s, %t", ns, model, action, idList, multi)
		t.FailNow()
	}
	r = u.Build(ns, model, action, idList)
	if r != "/api/v1/ns/model:sdf:eed:(action)" {
		t.Errorf("Expected '/api/v1/ns/model:sdf:eed:(action)' got '%s'", r)
		t.FailNow()
	}
}

func TestURIExtractURI(t *testing.T) {
	u, err := NewURI("/api/v1/")
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}

	badList := [][]string{{"api/v1"}, {"/api/v2/sd/sdf:d:"}}
	for _, v := range badList {
		_, err := u.ExtractIds(v)
		if err == nil {
			t.Errorf("Missing error '%s' for '%s'", err, v)
			t.FailNow()
		}
	}

	emptyList := [][]string{{}, {"/api/v1/"}, {"/api/v1/sdf/sdf"}}
	for _, v := range emptyList {
		r, err := u.ExtractIds(v)
		if err != nil {
			t.Errorf("Unexpected error '%s' for '%s'", err, v)
			t.FailNow()
		}
		if len(r) != 0 {
			t.Errorf("Expected '[]' got '%s' for '%s'", r, v)
			t.FailNow()
		}
	}

	aList := [][]string{{"/api/v1/nbs/model:d:efef:123:"}, {"/api/v1/nbs/model:d:", "/api/v1/nbs/model:efef:", "/api/v1/nbs/model:123:"}}
	aCmp := []string{"d", "efef", "123"}
	for _, v := range aList {
		r, err := u.ExtractIds(v)
		if err != nil {
			t.Errorf("Unexpected error '%s' for '%s'", err, v)
			t.FailNow()
		}
		if !reflect.DeepEqual(r, aCmp) {
			t.Errorf("Expected '%s' got '%s' for '%s'", aCmp, r, v)
			t.FailNow()
		}
	}
}

func TestNewClient(t *testing.T) {
	var goodHostList = []string{"http://host", "https://host"}
	var badHostList = []string{"htt://host", "http//host", "http://host/", "https://host/"}

	for _, v := range goodHostList {
		c, err := NewCInP(getLogger(), v, "/api/v1/", "")
		if err != nil {
			t.Errorf("Unexpected error '%s'", err)
			t.FailNow()
		}
		if c.host != v {
			t.Errorf("invalid host")
			t.FailNow()
		}
	}

	for _, v := range badHostList {
		_, err := NewCInP(getLogger(), v, "/api/v1/", "")
		if err == nil {
			t.Errorf("Error Missing")
			t.FailNow()
		}
	}

	var goodPathList = []string{"/api/v1/"}
	var badPathList = []string{"api/v1/", "/api/v1", "api/v1"}
	for _, v := range goodPathList {
		_, err := NewCInP(getLogger(), "http://host", v, "")
		if err != nil {
			t.Errorf("Unexpected error '%s'", err)
			t.FailNow()
		}
	}

	for _, v := range badPathList {
		_, err := NewCInP(getLogger(), "http://host", v, "")
		if err == nil {
			t.Errorf("Missing Error")
			t.FailNow()
		}
	}
}

func TestRequest(t *testing.T) {
	var reqURL string
	var reqMethod string
	var reqData []byte
	var reqDataLen int
	var reqHeaders map[string][]string
	var respData []byte
	var respHeaders map[string][]string

	handler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		reqURL = req.URL.String()
		reqMethod = req.Method
		reqData = make([]byte, 128)
		reqDataLen, _ = req.Body.Read(reqData)
		req.Body.Close()
		reqHeaders = req.Header
		req.Header = respHeaders
		rw.Write(respData)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	c, err := NewCInP(getLogger(), server.URL, "/api/v1/", "")
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}

	data := map[string]interface{}{}
	_, _, err = c.request(context.TODO(), "GET", "/api/v1/ns/model", nil, &data, nil)
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}
	if reqURL != "/api/v1/ns/model" {
		t.Errorf("Expected URI '/api/v1/ns/model' got '%s'", reqURL)
		t.FailNow()
	}
	if reqMethod != "GET" {
		t.Errorf("Expected Method 'GET' got '%s'", reqMethod)
		t.FailNow()
	}
	if reqDataLen != 0 {
		t.Errorf("Expected No Data got %s", reqData)
		t.FailNow()
	}
	var compareReqHeaders = map[string]string{
		"Cinp-Version":   "1.0",
		"Content-Type":   "application/json;charset=utf-8",
		"User-Agent":     "golang CInP client",
		"Accepts":        "application/json",
		"Accept-Charset": "utf-8",
	}
	for k, v := range compareReqHeaders {
		if reqHeaders[k][0] != v {
			t.Errorf("Invalid Header '%s' expected '%s' got '%s'", k, v, reqHeaders[k])
			t.FailNow()
		}
	}

	for k, v := range compareReqHeaders {
		_, ok := reqHeaders[k]
		if !ok {
			t.Errorf("Missing Header '%s'", k)
			t.FailNow()
		}
		if reqHeaders[k][0] != v {
			t.Errorf("Invalid Header '%s' expected '%s' got '%s'", k, v, reqHeaders[k])
			t.FailNow()
		}
	}

	_, _, err = c.request(context.TODO(), "BOB", "/api/v1/ns/model:123:(23)", nil, &data, nil)
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}
	if reqURL != "/api/v1/ns/model:123:(23)" {
		t.Errorf("Expected URI '/api/v1/ns/model:123:(23)' got '%s'", reqURL)
		t.FailNow()
	}
	if reqMethod != "BOB" {
		t.Errorf("Expected Method 'BOB' got '%s'", reqMethod)
		t.FailNow()
	}
	if reqDataLen != 0 {
		t.Errorf("Expected No Data got %s", reqData)
		t.FailNow()
	}

	_, _, err = c.request(context.TODO(), "GET", "/api", nil, &data, map[string]string{"hdr": "val", "top": "bottom"})
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}
	compareReqHeaders["Hdr"] = "val"
	compareReqHeaders["Top"] = "bottom"
	for k, v := range compareReqHeaders {
		_, ok := reqHeaders[k]
		if !ok {
			t.Errorf("Missing Header '%s'", k)
			t.FailNow()
		}
		if reqHeaders[k][0] != v {
			t.Errorf("Invalid Header '%s' expected '%s' got '%s'", k, v, reqHeaders[k])
			t.FailNow()
		}
	}
	delete(compareReqHeaders, "Hdr")
	delete(compareReqHeaders, "Top")

	respDataOut := map[string]interface{}{}
	respData = []byte("{\"a\": \"bob\"}")
	code, _, err := c.request(context.TODO(), "GET", "/api", &map[string]interface{}{"stuff": "jane"}, &respDataOut, nil)
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}
	if code != 200 {
		t.Errorf("got wrong code, expected '0' got '%d'", code)
	}
	cmp := []byte("{\"stuff\":\"jane\"}\n")
	if !bytes.Equal(reqData[0:reqDataLen], cmp) {
		t.Errorf("got wrong data body, got '%s' exptected '%s'", reqData[0:reqDataLen], cmp)
		t.FailNow()
	}
	if !reflect.DeepEqual(respDataOut, map[string]interface{}{"a": "bob"}) {
		t.Errorf("returned result wrong, got '%s'", respDataOut)
		t.FailNow()
	}

	_, _, err = c.request(context.TODO(), "GET", "/api", &map[string]interface{}{"stuff": func() {}}, &data, nil)
	if err == nil {
		t.Errorf("error missing")
		t.FailNow()
	}
}

func TestLogging(t *testing.T) {
	var respData []byte
	var reqData []byte
	var reqDataLen int

	handler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		reqData = make([]byte, 40960)
		reqDataLen, _ = req.Body.Read(reqData)
		req.Body.Close()
		written, err := rw.Write(respData)
		if err != nil {
			t.Errorf("Unexpected error '%s'", err)
			t.FailNow()
		}
		if written != len(respData) {
			t.Errorf("Not all bytes were written, expected %d, wrote %d", len(respData), written)
			t.FailNow()
		}
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	c, err := NewCInP(getLogger(), server.URL, "/api/v1/", "")
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}

	respDataOut := map[string]interface{}{}
	respData = []byte("{")
	reqDataIn := map[int]interface{}{}
	for i := 1; i <= 1000; i++ {
		reqDataIn[i] = "This is a Bunch of filler data"
		respData = append(respData, []byte("\"a\": \"Even More filler data\", ")...)
	}
	respData = append(respData, []byte("\"end\": \"The End\"}")...)
	// just making sure the logging doesn't lock up on very large requests and responses
	code, _, err := c.request(context.TODO(), "GET", "/api", &reqDataIn, &respDataOut, nil)
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}
	if code != 200 {
		t.Errorf("got wrong code, expected '0' got '%d'", code)
		t.FailNow()
	}

	cmp, err := marshalJSON(reqDataIn)
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}
	if !bytes.Equal(reqData[0:reqDataLen], cmp[0:reqDataLen]) {
		t.Errorf("got wrong data body, got '%s' exptected '%s'", reqData[0:reqDataLen], cmp[0:reqDataLen])
		t.FailNow()
	}

	if !reflect.DeepEqual(respDataOut, map[string]interface{}{"a": "Even More filler data", "end": "The End"}) {
		t.Errorf("returned result wrong, got '%s'", respDataOut)
		t.FailNow()
	}
}
