package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
	"gopkg.in/ldap.v3"
	"gopkg.in/yaml.v2"
)

var (
	configFlag   = flag.String("config", "config.yaml", "Specify YAML Configuration File")
	logLevelFlag = flag.String("loglevel", "INFO", "Minimum Log Level to display")
)

func addUsersToTeam(apiKeys GiteaKeys, users []Account, team int) bool {

	for i := 0; i < len(users); i++ {

		fullusername := url.PathEscape(fmt.Sprintf("%s", users[i].FullName))
		apiKeys.Command = "/api/v1/users/search?q=" + fullusername + "&access_token="
		foundUsers := requestSearchResults(apiKeys)

		for j := 0; j < len(foundUsers.Data); j++ {

			if strings.EqualFold(users[i].Login, foundUsers.Data[j].Login) {
				apiKeys.Command = "/api/v1/teams/" + fmt.Sprintf("%d", team) + "/members/" + foundUsers.Data[j].Login + "?access_token="
				error := requestPut(apiKeys)
				if len(error) > 0 {
					log.Errorln("Error (Team does not exist or Not Found User) :", parseJSON(error).(map[string]interface{})["message"])
				}
			}
		}
	}
	return true
}

func delUsersFromTeam(apiKeys GiteaKeys, Users []Account, team int) bool {

	for i := 0; i < len(Users); i++ {

		apiKeys.Command = "/api/v1/users/search?uid=" + fmt.Sprintf("%d", Users[i].ID) + "&access_token="

		foundUser := requestSearchResults(apiKeys)

		apiKeys.Command = "/api/v1/teams/" + fmt.Sprintf("%d", team) + "/members/" + foundUser.Data[0].Login + "?access_token="
		requestDel(apiKeys)
	}
	return true
}

func main() {
	// Parse flags of programm
	flag.Parse()
	logLevel, err := log.ParseLevel(*logLevelFlag)
	if err != nil {
		log.Fatalf("Loglevel %s not understood: %v", *logLevelFlag, err)
	}
	log.SetLevel(logLevel)
	mainJob() // First run for check settings

	var repTime string
	if len(os.Getenv("REP_TIME")) == 0 {

	} else {
		repTime = os.Getenv("REP_TIME")
	}

	c := cron.New()
	c.AddFunc(repTime, mainJob)
	c.Start()
	log.Debugf("Cron entries: %v", c.Entries())
	for true {
		time.Sleep(100 * time.Second)
	}
}

// This Function parses the enviroment for application specific variables and returns a Config struct.
// Used for setting all required settings in the application
func importEnvVars() Config {

	// Create temporary structs for creating the final config
	envConfig := Config{}

	// ApiKeys
	envConfig.APIKeys = GiteaKeys{}
	envConfig.APIKeys.TokenKey = strings.Split(os.Getenv("GITEA_TOKEN"), ",")
	envConfig.APIKeys.BaseURL = os.Getenv("GITEA_URL")

	// LDAP Config
	envConfig.LdapURL = os.Getenv("LDAP_URL")
	envConfig.LdapBindDN = os.Getenv("BIND_DN")
	envConfig.LdapBindPassword = os.Getenv("BIND_PASSWORD")
	envConfig.LdapFilter = os.Getenv("LDAP_FILTER")
	envConfig.LdapUserSearchBase = os.Getenv("LDAP_USER_SEARCH_BASE")

	// Check TLS Settings
	if len(os.Getenv("LDAP_TLS_PORT")) > 0 {
		port, err := strconv.Atoi(os.Getenv("LDAP_TLS_PORT"))
		envConfig.LdapPort = port
		envConfig.LdapTLS = true
		log.Debugf("DialTLS:=%v:%d", envConfig.LdapURL, envConfig.LdapPort)
		if err != nil {
			log.Errorln("LDAP_TLS_PORT is invalid.")
		}
	} else {
		if len(os.Getenv("LDAP_PORT")) > 0 {
			port, err := strconv.Atoi(os.Getenv("LDAP_PORT"))
			envConfig.LdapPort = port
			envConfig.LdapTLS = false
			log.Debugf("Dial:=%v:%d", envConfig.LdapURL, envConfig.LdapPort)
			if err != nil {
				log.Errorln("LDAP_PORT is invalid.")
			}
		}
	}
	// Set defaults for user Attributes
	if len(os.Getenv("LDAP_USER_IDENTITY_ATTRIBUTE")) == 0 {
		envConfig.LdapUserIdentityAttribute = "uid"
		log.Warnln("By default LDAP_USER_IDENTITY_ATTRIBUTE = 'uid'")
	} else {
		envConfig.LdapUserIdentityAttribute = os.Getenv("LDAP_USER_IDENTITY_ATTRIBUTE")
	}

	if len(os.Getenv("LDAP_USER_FULL_NAME")) == 0 {
		envConfig.LdapUserFullName = "sn" //change to cn if you need it
		log.Warnln("By default LDAP_USER_FULL_NAME = 'sn'")
	} else {
		envConfig.LdapUserFullName = os.Getenv("LDAP_USER_FULL_NAME")
	}

	return envConfig // return the config struct for use.
}

func importYAMLConfig(path string) (Config, error) {
	// Open Config File
	f, err := os.Open(path)
	if err != nil {
		return Config{}, err // Aborting
	}
	defer f.Close()

	// Parse File into Config Struct
	var cfg Config
	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&cfg)
	if err != nil {
		return Config{}, err // Aborting
	}
	return cfg, nil
}

func (c Config) checkConfig() {
	if len(c.APIKeys.TokenKey) <= 0 {
		log.Errorln("GITEA_TOKEN is empty or invalid.")
	}
	if len(c.APIKeys.BaseURL) == 0 {
		log.Errorln("GITEA_URL is empty")
	}
	if len(c.LdapURL) == 0 {
		log.Errorln("LDAP_URL is empty")
	}
	if c.LdapPort <= 0 {
		log.Errorln("LDAP_TLS_PORT is invalid.")
	} else {
		log.Infof("DialTLS:=%v:%d", c.LdapURL, c.LdapPort)
	}
	if len(c.LdapBindDN) == 0 {
		log.Warnln("BIND_DN is empty")
	}
	if len(c.LdapBindPassword) == 0 {
		log.Warnln("BIND_PASSWORD is empty")
	}
	if len(c.LdapFilter) == 0 {
		log.Warnln("LDAP_FILTER is empty")
	}
	if len(c.LdapUserSearchBase) == 0 {
		log.Errorln("LDAP_USER_SEARCH_BASE is empty")
	}
	if len(c.LdapUserIdentityAttribute) == 0 {
		c.LdapUserIdentityAttribute = "uid"
		log.Warnln("By default LDAP_USER_IDENTITY_ATTRIBUTE = 'uid'")
	}
	if len(c.LdapUserFullName) == 0 {
		c.LdapUserFullName = "sn"
		log.Warnln("By default LDAP_USER_FULL_NAME = 'sn'")
	}
}

func mainJob() {

	//------------------------------
	//  Check and Set input settings
	//------------------------------
	var cfg Config

	cfg, importErr := importYAMLConfig(*configFlag)
	if importErr != nil {
		log.Warnln("Fallback: Importing Settings from Enviroment Variables ")
		cfg = importEnvVars()
	} else {
		log.Debugln("Successfully imported YAML Config from %s", *configFlag)
		log.Debugf("%+v", cfg)
	}
	// Checks Config
	cfg.checkConfig()
	log.Debugln("Checked config elements")

	// Prepare LDAP Connection
	var l *ldap.Conn
	var err error
	if cfg.LdapTLS {
		l, err = ldap.DialTLS("tcp", fmt.Sprintf("%s:%d", cfg.LdapURL, cfg.LdapPort), &tls.Config{InsecureSkipVerify: true})
	} else {
		l, err = ldap.Dial("tcp", fmt.Sprintf("%s:%d", cfg.LdapURL, cfg.LdapPort))
	}

	if err != nil {
		log.Fatalf("Error connecting to LDAP server: %v", err)
	}
	defer l.Close()

	err = l.Bind(cfg.LdapBindDN, cfg.LdapBindPassword)
	if err != nil {
		log.Fatalf("Error binding to LDAP server: %v", err)
	}
	page := 1
	cfg.APIKeys.BruteforceTokenKey = 0
	cfg.APIKeys.Command = "/api/v1/admin/orgs?page=" + fmt.Sprintf("%d", page) + "&limit=20&access_token=" // List all organizations
	organizationList := requestOrganizationList(cfg.APIKeys)

	log.Debugf("%d organizations were found on the server: %s", len(organizationList), cfg.APIKeys.BaseURL)

	for 0 < len(organizationList) {

		for i := 0; i < len(organizationList); i++ {

			log.Debugln(organizationList)

			log.Debugf("Begin an organization review: OrganizationName= %v, OrganizationId= %d \n", organizationList[i].Name, organizationList[i].ID)

			cfg.APIKeys.Command = "/api/v1/orgs/" + organizationList[i].Name + "/teams?access_token="
			teamList := requestTeamList(cfg.APIKeys)
			log.Debugf("%d teams were found in %s organization", len(teamList), organizationList[i].Name)
			log.Debugf("Skip synchronization in the Owners team")
			cfg.APIKeys.BruteforceTokenKey = 0

			for j := 1; j < len(teamList); j++ {

				// preparing request to ldap server
				filter := fmt.Sprintf(cfg.LdapFilter, teamList[j].Name)
				searchRequest := ldap.NewSearchRequest(
					cfg.LdapUserSearchBase, // The base dn to search
					ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
					filter, // The filter to apply
					[]string{"cn", "uid", "mailPrimaryAddress, sn", cfg.LdapUserIdentityAttribute}, // A list attributes to retrieve
					nil,
				)
				// make request to ldap server
				sr, err := l.Search(searchRequest)
				if err != nil {
					log.Fatal(err)
				}
				AccountsLdap := make(map[string]Account)
				AccountsGitea := make(map[string]Account)
				var addUserToTeamList, delUserToTeamlist []Account
				if len(sr.Entries) > 0 {
					log.Infof("The LDAP %s has %d users corresponding to team %s", cfg.LdapURL, len(sr.Entries), teamList[j].Name)
					for _, entry := range sr.Entries {

						AccountsLdap[entry.GetAttributeValue(cfg.LdapUserIdentityAttribute)] = Account{
							FullName: entry.GetAttributeValue(cfg.LdapUserFullName),
							Login:    entry.GetAttributeValue(cfg.LdapUserIdentityAttribute),
						}
					}

					cfg.APIKeys.Command = "/api/v1/teams/" + fmt.Sprintf("%d", teamList[j].ID) + "/members?access_token="
					AccountsGitea, cfg.APIKeys.BruteforceTokenKey = requestUsersList(cfg.APIKeys)
					log.Infof("The gitea %s has %d users corresponding to team %s Teamid=%d", cfg.APIKeys.BaseURL, len(AccountsGitea), teamList[j].Name, teamList[j].ID)

					for k, v := range AccountsLdap {
						if AccountsGitea[k].Login != v.Login {
							addUserToTeamList = append(addUserToTeamList, v)
						}
					}
					log.Debugf("can be added users list %v", addUserToTeamList)
					addUsersToTeam(cfg.APIKeys, addUserToTeamList, teamList[j].ID)

					for k, v := range AccountsGitea {
						if AccountsLdap[k].Login != v.Login {
							delUserToTeamlist = append(delUserToTeamlist, v)
						}
					}
					log.Debugf("must be del users list %v", delUserToTeamlist)
					delUsersFromTeam(cfg.APIKeys, delUserToTeamlist, teamList[j].ID)

				} else {
					log.Infof("The LDAP %s found no users corresponding to team %s", cfg.LdapURL, teamList[j].Name)
				}
			}
		}

		page++
		cfg.APIKeys.BruteforceTokenKey = 0
		cfg.APIKeys.Command = "/api/v1/admin/orgs?page=" + fmt.Sprintf("%d", page) + "&limit=20&access_token=" // List all organizations
		organizationList = requestOrganizationList(cfg.APIKeys)
		log.Debugf("%d organizations were found on the server: %s", len(organizationList), cfg.APIKeys.BaseURL)
	}
}
