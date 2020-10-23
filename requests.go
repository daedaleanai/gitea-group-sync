package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"

	//	"reflect"
	"net"
	"net/url"
	"strings"
	"time"
)

func CheckStatusCode(res *http.Response) {

	switch {
	case 300 <= res.StatusCode && res.StatusCode < 400:
		log.Info("CheckStatusCode gitea apiKeys connection error: Redirect message")
	case 401 == res.StatusCode:
		log.Info("CheckStatusCode gitea apiKeys connection Error: Unauthorized")
	case 400 <= res.StatusCode && res.StatusCode < 500:
		log.Info("CheckStatusCode gitea apiKeys connection error: Client error")
	case 500 <= res.StatusCode && res.StatusCode < 600:
		log.Info("CheckStatusCode gitea apiKeys connection error Server error")
	}
}

func hasTimedOut(err error) bool {
	switch err := err.(type) {
	case *url.Error:
		if err, ok := err.Err.(net.Error); ok && err.Timeout() {
			return true
		}
	case net.Error:
		if err.Timeout() {
			return true
		}
	case *net.OpError:
		if err.Timeout() {
			return true
		}
	}

	errTxt := "use of closed network connection"
	if err != nil && strings.Contains(err.Error(), errTxt) {
		return true
	}

	return false
}

func RequestGet(apiKeys GiteaKeys) []byte {
	cc := &http.Client{Timeout: time.Second * 2}
	url := apiKeys.BaseUrl + apiKeys.Command + apiKeys.TokenKey[apiKeys.BruteforceTokenKey]
	res, err := cc.Get(url)
	if err != nil && hasTimedOut(err) {
		log.Fatalf("Get Request to %s failed: %v", url, err)
	}
	CheckStatusCode(res)
	b, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}
	res.Body.Close()
	return b
}

func RequestPut(apiKeys GiteaKeys) []byte {
	cc := &http.Client{Timeout: time.Second * 2}
	url := apiKeys.BaseUrl + apiKeys.Command + apiKeys.TokenKey[apiKeys.BruteforceTokenKey]
	request, err := http.NewRequest("PUT", url, nil)
	res, err := cc.Do(request)
	CheckStatusCode(res)
	if err != nil && hasTimedOut(err) {
		log.Fatalf("PUT Request to %s failed: %v", url, err)
	}
	b, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}
	res.Body.Close()
	return b
}

func RequestDel(apiKeys GiteaKeys) []byte {

	cc := &http.Client{Timeout: time.Second * 2}
	url := apiKeys.BaseUrl + apiKeys.Command + apiKeys.TokenKey[apiKeys.BruteforceTokenKey]
	request, err := http.NewRequest("DELETE", url, nil)
	res, err := cc.Do(request)
	CheckStatusCode(res)
	if err != nil && hasTimedOut(err) {
		log.Fatalf("DELETE Request to %s failed: %v", url, err)
	}
	b, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}
	res.Body.Close()
	return b
}

func RequestSearchResults(ApiKeys GiteaKeys) SearchResults {

	b := RequestGet(ApiKeys)

	var f SearchResults
	jsonErr := json.Unmarshal(b, &f)
	if jsonErr != nil {
		log.Fatal(jsonErr)
	}
	return f
}

func RequestUsersList(ApiKeys GiteaKeys) (map[string]Account, int) {

	b := RequestGet(ApiKeys)
	var Account_u = make(map[string]Account)

	var f []Account
	jsonErr := json.Unmarshal(b, &f)
	if jsonErr != nil {
		log.Errorf("Error unmarshaling json response: %v", jsonErr)
		if ApiKeys.BruteforceTokenKey == len(ApiKeys.TokenKey)-1 {
			log.Fatal("Token key is unsuitable, call to system administrator ")
		} else {
			log.Error("Can't get UsersList try another token key")
		}
		if ApiKeys.BruteforceTokenKey < len(ApiKeys.TokenKey)-1 {
			ApiKeys.BruteforceTokenKey++
			log.Debugf("BruteforceTokenKey=%d", ApiKeys.BruteforceTokenKey)
			Account_u, ApiKeys.BruteforceTokenKey = RequestUsersList(ApiKeys)
		}
	}

	for i := 0; i < len(f); i++ {
		Account_u[f[i].Login] = Account{
			//			Email:     f[i].Email,
			Id:        f[i].Id,
			Full_name: f[i].Full_name,
			Login:     f[i].Login,
		}
	}
	return Account_u, ApiKeys.BruteforceTokenKey
}

func RequestOrganizationList(apiKeys GiteaKeys) []Organization {

	b := RequestGet(apiKeys)

	var f []Organization
	jsonErr := json.Unmarshal(b, &f)
	if jsonErr != nil {
		log.Fatalf("Please check setting GITEA_TOKEN, GITEA_URL. Error unmarshaling JSON: %v", jsonErr)
	}
	return f
}

func RequestTeamList(apiKeys GiteaKeys) []Team {

	b := RequestGet(apiKeys)

	var f []Team
	jsonErr := json.Unmarshal(b, &f)
	if jsonErr != nil {
		log.Fatalf("Please check setting GITEA_TOKEN, GITEA_URL. Error unmarshaling JSON: %v", jsonErr)
	}
	return f
}

func parseJson(b []byte) interface{} {
	var f interface{}
	jsonErr := json.Unmarshal(b, &f)
	if jsonErr != nil {
		log.Fatalf("Please check setting GITEA_TOKEN, GITEA_URL. Error unmarshaling JSON: %v", jsonErr)
	}
	m := f.(interface{})
	return m
}

func parseJsonArray(b []byte) []interface{} {
	var f interface{}
	jsonErr := json.Unmarshal(b, &f)
	if jsonErr != nil {
		log.Fatalf("Please check setting GITEA_TOKEN, GITEA_URL. Error unmarshaling JSON: %v", jsonErr)
	}
	m := f.([]interface{})
	return m
}
