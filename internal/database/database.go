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

package database

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"sync"
	"time"
)

var dbMutex = &sync.Mutex{}

type Registration struct {
	DeviceToken string
	AccountId   string
}

type Account struct {
	DeviceToken      string
	Mailboxes        []string
	RegistrationTime time.Time
}

func (account *Account) ContainsMailbox(mailbox string) bool {
	for _, m := range account.Mailboxes {
		if m == mailbox {
			return true
		}
	}
	return false
}

type User struct {
	Accounts map[string]Account
}

type Database struct {
	filename  string
	Users     map[string]User
	lastWrite time.Time
}

func NewDatabase(filename string) (*Database, error) {
	// check if file exists
	_, err := os.Stat(filename)
	if err != nil && os.IsNotExist(err) {
		db := &Database{filename: filename, Users: make(map[string]User)}
		err := db.write()
		if err != nil {
			return nil, err
		}
		return db, nil
	}

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	db := Database{filename: filename, Users: make(map[string]User)}
	if len(data) != 0 {
		err := json.Unmarshal(data, &db)
		if err != nil {
			return nil, err
		}
	}

	registrationCleanupTicker := time.NewTicker(time.Hour * 8)
	go func() {
		for range registrationCleanupTicker.C {
			db.cleanupRegistered()
		}
	}()

	return &db, nil
}

func (db *Database) write() error {
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(db.filename+".new", data, 0644)
	return os.Rename(db.filename+".new", db.filename)
}

func (db *Database) AddRegistration(username, accountId, deviceToken string, mailboxes []string) (err error) {
	//  mutual write access to database issue #16 xaps-plugin
	dbMutex.Lock()

	// Ensure the User exists
	if _, ok := db.Users[username]; !ok {
		db.Users[username] = User{Accounts: make(map[string]Account)}
	} else {
		log.Debugf("AddRegistration(): User %s already exists", username)
	}

	// Ensure the Account exists
	if _, ok := db.Users[username].Accounts[accountId]; !ok {
		db.Users[username].Accounts[accountId] = Account{}
	} else {
		log.Debugf("AddRegistration(): Account %s already exists", accountId)
	}

	// Set or update the Registration
	db.Users[username].Accounts[accountId] =
		Account{
			DeviceToken:      deviceToken,
			Mailboxes:        mailboxes,
			RegistrationTime: time.Now(),
		}

	log.Debugf("AddRegistration(): About to flush db to disk")
	if db.lastWrite.Before(time.Now().Add(-time.Minute * 15)) {
		err = db.write()
		db.lastWrite = time.Now()
	} else {
		log.Debugf("AddRegistration(): DB flush postponed since last write (%s) is not older than 15 minutes", db.lastWrite)
	}

	// release mutex
	dbMutex.Unlock()
	return
}

func (db *Database) DeleteIfExistRegistration(reg Registration) bool {
	dbMutex.Lock()
	for username, user := range db.Users {
		for accountId, account := range user.Accounts {
			if accountId == reg.AccountId {
				log.Infoln("Deleting " + account.DeviceToken)
				delete(user.Accounts, accountId)
				// clean up empty users
				if len(user.Accounts) == 0 {
					delete(db.Users, username)
				}
				err := db.write()
				if err != nil {
					log.Error(err)
				}
				dbMutex.Unlock()
				return true
			}
		}
	}
	dbMutex.Unlock()
	return false
}

func (db *Database) FindRegistrations(username, mailbox string) ([]Registration, error) {
	var registrations []Registration
	dbMutex.Lock()
	if user, ok := db.Users[username]; ok {
		for accountId, account := range user.Accounts {
			if account.ContainsMailbox(mailbox) {
				registrations = append(registrations,
					Registration{DeviceToken: account.DeviceToken, AccountId: accountId})
			}
		}
	}
	dbMutex.Unlock()
	return registrations, nil
}

func (db *Database) UserExists(username string) bool {
	dbMutex.Lock()
	_, ok := db.Users[username]
	dbMutex.Unlock()
	return ok
}

func (db *Database) cleanupRegistered() {
	log.Debugln("Check Database for devices not calling IMAP hook for more than 30d")
	toDelete := make([]Registration, 0)
	dbMutex.Lock()
	for _, user := range db.Users {
		for accountId, account := range user.Accounts {
			if !account.RegistrationTime.IsZero() && account.RegistrationTime.Before(time.Now().Add(-time.Hour*24*30)) {
				toDelete = append(toDelete, Registration{account.DeviceToken, accountId})
			}
		}
	}
	dbMutex.Unlock()
	for _, reg := range toDelete {
		db.DeleteIfExistRegistration(reg)
	}
}
