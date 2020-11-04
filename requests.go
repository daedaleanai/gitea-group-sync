package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"

	//	"reflect"
	"net"
	"net/url"
	"strings"
	"time"
)

func checkStatusCode(res *http.Response) {

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

func requestGet(apiKeys GiteaKeys, command string) []byte {
	cc := &http.Client{Timeout: time.Second * 2}
	for index := range apiKeys.TokenKey {
		url := fmt.Sprintf("%s%s%s", apiKeys.BaseURL, command, apiKeys.TokenKey[index])

		res, err := cc.Get(url)
		if err != nil && hasTimedOut(err) {
			log.Fatalf("Get Request to %s failed: %v", url, err)
		}
		checkStatusCode(res)
		if res.StatusCode == 401 {
			continue
		}
		b, readErr := ioutil.ReadAll(res.Body)
		if readErr != nil {
			log.Fatal(readErr)
		}
		res.Body.Close()
		return b
	}
	return nil
}

func requestPut(apiKeys GiteaKeys) []byte {
	cc := &http.Client{Timeout: time.Second * 2}
	url := apiKeys.BaseURL + apiKeys.Command + apiKeys.TokenKey[apiKeys.BruteforceTokenKey]
	if apiKeys.DryRun {
		log.Debugf("Would call %s", url)
		return nil
	}
	request, err := http.NewRequest("PUT", url, nil)
	res, err := cc.Do(request)
	checkStatusCode(res)
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

func requestDel(apiKeys GiteaKeys, command string) []byte {
	cc := &http.Client{Timeout: time.Second * 2}
	url := fmt.Sprintf("%s%s%s", apiKeys.BaseURL, command, apiKeys.TokenKey[apiKeys.BruteforceTokenKey])
	if apiKeys.DryRun {
		log.Debugf("Would call %s", url)
		return nil
	}
	request, err := http.NewRequest("DELETE", url, nil)
	res, err := cc.Do(request)
	checkStatusCode(res)
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

func getUserByUsername(APIKeys GiteaKeys, username string) SearchResults {

	command := fmt.Sprintf("/api/v1/users/search?q=%s&access_token=", username)

	b := requestGet(APIKeys, command)

	var f SearchResults
	jsonErr := json.Unmarshal(b, &f)
	if jsonErr != nil {
		log.Fatal(jsonErr)
	}
	return f
}

func getUserByID(APIKeys GiteaKeys, ID int) SearchResults {

	command := fmt.Sprintf("/api/v1/users/search?uid=%d&access_token=", ID)

	b := requestGet(APIKeys, command)

	var f SearchResults
	jsonErr := json.Unmarshal(b, &f)
	if jsonErr != nil {
		log.Fatal(jsonErr)
	}
	return f
}

func deleteUserFromTeam(APIKeys GiteaKeys, team int, user User) {
	command := fmt.Sprintf("/api/v1/teams/%d/members/%s?access_token=", team, user.Login)

	requestDel(APIKeys, command)
}

func requestUsersList(APIKeys GiteaKeys, team Team) map[string]Account {

	command := fmt.Sprintf("/api/v1/teams/%d/members?access_token=", team.ID)
	var AccountU = make(map[string]Account)
	var f []Account

	b := requestGet(APIKeys, command)

	jsonErr := json.Unmarshal(b, &f)
	if jsonErr != nil {
		log.Errorf("Error unmarshaling json response: %v", jsonErr)
	}

	for i := 0; i < len(f); i++ {
		AccountU[f[i].Login] = Account{
			//			Email:     f[i].Email,
			ID:       f[i].ID,
			FullName: f[i].FullName,
			Login:    f[i].Login,
		}
	}
	return AccountU
}

func requestOrganizationList(apiKeys GiteaKeys) []Organization {
	page := 1
	limit := 20

	var orgs []Organization

	for {
		command := fmt.Sprintf("/api/v1/admin/orgs?page=%d&limit=%d&access_token=", page, limit) // List all organizations

		b := requestGet(apiKeys, command)

		var f []Organization
		jsonErr := json.Unmarshal(b, &f)
		if jsonErr != nil {
			log.Fatalf("Please check setting GITEA_TOKEN, GITEA_URL. Error unmarshaling JSON: %v", jsonErr)
		}
		if len(f) == 0 {
			break
		}
		orgs = append(orgs, f...)
		page++
	}
	return orgs
}

func requestTeamList(apiKeys GiteaKeys, org Organization) []Team {
	command := fmt.Sprintf("/api/v1/orgs/%s/teams?access_token=", org.Name)

	b := requestGet(apiKeys, command)

	var f []Team
	jsonErr := json.Unmarshal(b, &f)
	if jsonErr != nil {
		log.Fatalf("Please check setting GITEA_TOKEN, GITEA_URL. Error unmarshaling JSON: %v", jsonErr)
	}
	return f
}

func parseJSON(b []byte) interface{} {
	var f interface{}
	jsonErr := json.Unmarshal(b, &f)
	if jsonErr != nil {
		log.Fatalf("Please check setting GITEA_TOKEN, GITEA_URL. Error unmarshaling JSON: %v", jsonErr)
	}
	m := f.(interface{})
	return m
}

func parseJSONArray(b []byte) []interface{} {
	var f interface{}
	jsonErr := json.Unmarshal(b, &f)
	if jsonErr != nil {
		log.Fatalf("Please check setting GITEA_TOKEN, GITEA_URL. Error unmarshaling JSON: %v", jsonErr)
	}
	m := f.([]interface{})
	return m
}
