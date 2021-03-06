//: ----------------------------------------------------------------------------
//: Copyright (C) 2017 Verizon.  All Rights Reserved.
//:
//:   Licensed under the Apache License, Version 2.0 (the "License");
//:   you may not use this file except in compliance with the License.
//:   You may obtain a copy of the License at
//:
//:       http://www.apache.org/licenses/LICENSE-2.0
//:
//:   Unless required by applicable law or agreed to in writing, software
//:   distributed under the License is distributed on an "AS IS" BASIS,
//:   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//:   See the License for the specific language governing permissions and
//:   limitations under the License.
//:
//: ----------------------------------------------------------------------------
package main

import (
	"errors"
	"fmt"
	"github.com/parnurzeal/gorequest"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

///////////////////////////// CLI ENTRYPOINT //////////////////////////////////

func LoadDefaultConfigOrExit(http *gorequest.SuperAgent) *Config {
	pth := defaultConfigPath()
	errout := []error{}

	_, err := os.Stat(pth)

	if os.IsNotExist(err) {
		errout = append(errout, errors.New("No config file existed at "+pth+". You need to `nelson login` before running other commands."))
	}

	x, parsed := readConfigFile(pth)

	if x != nil {
		errout = append(errout, errors.New("Unable to read configuration file at '"+pth+"'. Reported error was: "+x.Error()))
	}

	ve := parsed.Validate()

	if ve != nil {
		errout = append(errout, ve...) // TIM: wtf golang, ... means "expand these as vararg function application"
	}

	// if there are errors loading the config, assume its an expired
	// token and try to regenerate the configuration
	if errout != nil {
		// configuration file does not exist
		if err != nil {
			bailout(errout)
		}

		if len(ve) > 0 {
			// retry the login based on information we know
			x := attemptConfigRefresh(http, parsed)
			// if that didnt help, then bail out and report the issue to the user.
			if x != nil {
				errout = append(errout, x...)
				bailout(errout)
			}
			_, contents := readConfigFile(pth)
			return contents
		}
	}
	// if regular loading of the config worked, then
	// just go with that! #happypath
	return parsed
}

func attemptConfigRefresh(http *gorequest.SuperAgent, existing *Config) []error {
	errout := []error{}
	var ghToken string = os.Getenv("GITHUB_TOKEN")
	e, u := hostFromUri(existing.Endpoint)
	if e != nil {
		return []error{e}
	}
	if len([]rune(ghToken)) == 0 {
		errout = append(errout, errors.New("Environment GITHUB_TOKEN variable not defined. "))
		bailout(errout)
		// return []error{errout}
	}
	fmt.Println("Attempted token refresh...")
	return Login(http, ghToken, u, false)
}

func bailout(errors []error) {
	fmt.Println("🚫")
	fmt.Println("Encountered an unexpected problem(s) loading the configuration file: ")
	PrintTerminalErrors(errors)
	os.Exit(1)
}

////////////////////////////// CONFIG YAML ///////////////////////////////////

type Config struct {
	Endpoint      string `yaml:endpoint`
	ConfigSession `yaml:"session"`
}

type ConfigSession struct {
	Token     string `yaml:"token"`
	ExpiresAt int64  `yaml:"expires_at"`
}

func (c *Config) GetAuthCookie() *http.Cookie {
	expire := time.Now().AddDate(0, 0, 1)
	cookie := &http.Cookie{
		Name:       "nelson.session",
		Value:      c.ConfigSession.Token,
		Path:       "/",
		Domain:     "nelson.yourcompany.com",
		Expires:    expire,
		RawExpires: expire.Format(time.UnixDate),
		MaxAge:     86400,
		Secure:     true,
		HttpOnly:   false,
	}

	return cookie
}

func generateConfigYaml(s Session, url string) string {
	temp := &Config{
		Endpoint: url,
		ConfigSession: ConfigSession{
			Token:     s.SessionToken,
			ExpiresAt: s.ExpiresAt,
		},
	}

	d, err := yaml.Marshal(&temp)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	return "---\n" + string(d)
}

func parseConfigYaml(yamlAsBytes []byte) *Config {
	temp := &Config{}
	err := yaml.Unmarshal(yamlAsBytes, &temp)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	return temp
}

func (c *Config) Validate() []error {
	// check that the token has not expired
	errs := []error{}

	if c.ConfigSession.ExpiresAt <= currentTimeMillis() {
		errs = append(errs, errors.New("Your session has expired. Please 'nelson login' again to reactivate your session."))
	}
	return errs
}

/////////////////////////////// CONFIG I/O ////////////////////////////////////

func defaultConfigPath() string {
	targetDir := os.Getenv("HOME") + "/.nelson"
	os.Mkdir(targetDir, 0775)
	return targetDir + "/config.yml"
}

// returns Unit, no error handling. YOLO
func writeConfigFile(s Session, url string, configPath string) {
	yamlConfig := generateConfigYaml(s, url)

	err := ioutil.WriteFile(configPath, []byte(yamlConfig), 0755)
	if err != nil {
		panic(err)
	}
}

func readConfigFile(configPath string) (error, *Config) {
	b, err := ioutil.ReadFile(configPath)
	return err, parseConfigYaml(b) // TIM: parsing never fails, right? ;-)
}
