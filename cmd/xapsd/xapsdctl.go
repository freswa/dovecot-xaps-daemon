//
// The MIT License (MIT)
//
// Copyright (c) 2015 Stefan Arentz <stefan@arentz.ca>
// Copyright (c) 2017 Frederik Schwan <frederik dot schwan at linux dot com>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
//

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"

	"github.com/freswa/dovecot-xaps-daemon/internal/config"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const Version = "0.6"

var configPath = flag.String("configPath", "./configs/xapsd/", `Add an additional path to lookup the config file in`)
var configName = flag.String("configName", "", `Set a different configName (without extension) than the default "xapsd"`)

var setLogLevel = flag.String("setLogLevel", "warn", `Set the loglevel to either trace, debug, error, fatal, info, panic or warn`)
var setDatabaseFile = flag.String("setDatabaseFile", "", `Sets the location of the file database file. Xapsd creates a json file to store the registration persistent on disk.`)
var setSocketPath = flag.String("setSocketPath", "", `Sets the location of the socket xapsd uses a socket to allow dovecot to connect.`)
var setCheckInterval = flag.Uint("setCheckInterval", 0, `This sets the interval to check for delayed messages.`)
var setDelay = flag.Uint("setDelay", 0, `Set the time how long notifications for not-new messages should be delayed until they are sent.`)
var setAppleID = flag.String("setAppleID", "", `Set a valid Apple ID to retrieve certificates from Apple`)
var setApplePassword = flag.String("setApplePassword", "", `Set the correct Apple ID Password. The password will be saved as hach value`)

func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func hashPassword(cleartext string) string {
	hash := sha256.New()
	hash.Write([]byte(cleartext))
	sha256_hash := hex.EncodeToString(hash.Sum(nil))
	return sha256_hash

}
func main() {
	flag.Parse()
	config.ParseConfig(*configName, *configPath)
	config := config.GetOptions()
	lvl, err := log.ParseLevel(config.LogLevel)
	if err != nil {
		log.Fatal(err)
	}
	log.SetLevel(lvl)

	if isFlagPassed("setLogLevel") {
		viper.Set(`LogLevel`, setLogLevel)
	}
	if isFlagPassed("setDatabaseFile") {
		viper.Set(`DatabaseFile`, setSocketPath)
	}
	if isFlagPassed("setSocketPath") {
		viper.Set(`SocketPath`, setSocketPath)
	}
	if isFlagPassed("setCheckInterval") {
		viper.Set(`CheckInterval`, setCheckInterval)
	}
	if isFlagPassed("setDelay") {
		viper.Set(`Delay`, setDelay)
	}
	if isFlagPassed("setAppleID") {
		viper.Set(`AppleID`, setAppleID)
	}
	if isFlagPassed("setApplePassword") {
		viper.Set(`AppleIdHashedPassword`, hashPassword(*setApplePassword))
	}

	err = viper.WriteConfig()
	if err != nil {
		log.Fatal(err)
	}
}
