package cinp

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

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

func TestNewClient(t *testing.T) {
	var goodHostList = []string{"http://host", "https://host"}
	var badHostList = []string{"htt://host", "http//host", "http://host/", "https://host/"}

	for _, v := range goodHostList {
		c, err := NewCInP(v, "/api/v1/", "")
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
		_, err := NewCInP(v, "/api/v1/", "")
		if err == nil {
			t.Errorf("Error Missing")
			t.FailNow()
		}
	}

	var goodPathList = []string{"/api/v1/"}
	var badPathList = []string{"api/v1/", "/api/v1", "api/v1"}
	for _, v := range goodPathList {
		_, err := NewCInP("http://host", v, "")
		if err != nil {
			t.Errorf("Unexpected error '%s'", err)
			t.FailNow()
		}
	}

	for _, v := range badPathList {
		_, err := NewCInP("http://host", v, "")
		if err == nil {
			t.Errorf("Missing Error")
			t.FailNow()
		}
	}
}

func TestSetAuth(t *testing.T) {
	c, err := NewCInP("http://host", "/api/v1/", "")
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}
	c.SetAuth("auth id", "auth token")
	if c.authID != "auth id" || c.authToken != "auth token" {
		t.Errorf("Set auth  mismatch")
		t.FailNow()
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

	c, err := NewCInP(server.URL, "/api/v1/", "")
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}

	_, _, _, err = c.request("GET", "/api/v1/ns/model", nil, nil)
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
		"Cinp-Version":   "0.9",
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

	c.SetAuth("the user", "the token")
	_, _, _, err = c.request("GET", "/api/v1/ns/model", nil, nil)
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}
	compareReqHeaders["Auth-Id"] = "the user"
	compareReqHeaders["Auth-Token"] = "the token"
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

	delete(compareReqHeaders, "Auth-Id")
	delete(compareReqHeaders, "Auth-Token")
	c.SetAuth("", "")
	_, _, _, err = c.request("GET", "/api/v1/ns/model", nil, nil)
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}

	if _, ok := reqHeaders["Auth-Id"]; ok {
		t.Errorf("Auth-Id present")
		t.FailNow()
	}
	if _, ok := reqHeaders["Auth-Token"]; ok {
		t.Errorf("Auth-Token present")
		t.FailNow()
	}

	_, _, _, err = c.request("BOB", "/api/v1/ns/model:123:(23)", nil, nil)
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

	_, _, _, err = c.request("GET", "/api", nil, map[string]string{"hdr": "val", "top": "bottom"})
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

	respData = []byte("{\"a\": \"bob\"}")
	code, resp, _, err := c.request("GET", "/api", map[string]interface{}{"stuff": "jane"}, nil)
	if err != nil {
		t.Errorf("Unexpected error '%s'", err)
		t.FailNow()
	}
	if code != 200 {
		t.Errorf("got wrong code, expected '0' got '%d'", code)
	}
	cmp := []byte("{\"stuff\":\"jane\"}")
	if bytes.Compare(reqData[0:reqDataLen], cmp) != 0 {
		t.Errorf("got wrong data body, got '%s'", reqData[0:reqDataLen])
		t.FailNow()
	}
	if !reflect.DeepEqual(resp, map[string]interface{}{"a": "bob"}) {
		t.Errorf("returned result wrong, got '%s'", resp)
		t.FailNow()
	}

	_, _, _, err = c.request("GET", "/api", map[string]interface{}{"stuff": func() {}}, nil)
	if err == nil {
		t.Errorf("error missing")
		t.FailNow()
	}
}
