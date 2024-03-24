CInP client for Go
==================

Install
-------

::

  go get github.com/cinp/go


Usage Example
-------------

::
  package main

  import (
      "fmt"

      cinp "github.com/cinp/go"
  )

  func main() {
    host := "http://localhost"
    proxy := nil
    expedtedAPIVersion := "0.1"
    username := "bob"
    password := "supersecret"

    client, err = cinp.NewCInP(host, "/api/v1/", proxy)
    if err != nil {
        return nil, err
    }

    APIVersion, err := getAPIVersion("/api/v1/")
    if err != nil {
        return nil, err
    }

    if APIVersion != expedtedAPIVersion {
        return nil, fmt.Errorf("API version mismatch.  Got '%s', expected '%s'", APIVersion, expedtedAPIVersion)
    }

    args := map[string]interface{}{
            "username": username,
            "password": password,
    }
    result := ""
    err := client.call("/api/v1/Auth/Auth(login)", &args, &result)
    if err != nil {
        return nil, err
    }

    client.setHeader("Auth-Id", username)
    client.setHeader("Auth-Token", result)

    # do stuff like
    #client.get("/api/")

    args = map[string]interface{}{}
    result = ""
    err := client.call("/api/v1/Auth/Auth(logout)", &args, &result)
    if err != nil {
        return nil, err
    }
  }
