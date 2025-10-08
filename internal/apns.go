package internal

import (
	"crypto/md5"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/freswa/dovecot-xaps-daemon/internal/config"
	"github.com/freswa/dovecot-xaps-daemon/internal/database"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"github.com/sideshow/apns2/token"
	log "github.com/sirupsen/logrus"
)

const (
	// renew certs this duration before the certs become invalid
	renewTimeBuffer = time.Hour * 24 * 30
)

var (
	oidUid        = []int{0, 9, 2342, 19200300, 100, 1, 1}
	productionOID = []int{1, 2, 840, 113635, 100, 6, 3, 2}
	//GeoTrustCert  = "-----BEGIN CERTIFICATE-----\nMIIDVDCCAjygAwIBAgIDAjRWMA0GCSqGSIb3DQEBBQUAMEIxCzAJBgNVBAYTAlVT\nMRYwFAYDVQQKEw1HZW9UcnVzdCBJbmMuMRswGQYDVQQDExJHZW9UcnVzdCBHbG9i\nYWwgQ0EwHhcNMDIwNTIxMDQwMDAwWhcNMjIwNTIxMDQwMDAwWjBCMQswCQYDVQQG\nEwJVUzEWMBQGA1UEChMNR2VvVHJ1c3QgSW5jLjEbMBkGA1UEAxMSR2VvVHJ1c3Qg\nR2xvYmFsIENBMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA2swYYzD9\n9BcjGlZ+W988bDjkcbd4kdS8odhM+KhDtgPpTSEHCIjaWC9mOSm9BXiLnTjoBbdq\nfnGk5sRgprDvgOSJKA+eJdbtg/OtppHHmMlCGDUUna2YRpIuT8rxh0PBFpVXLVDv\niS2Aelet8u5fa9IAjbkU+BQVNdnARqN7csiRv8lVK83Qlz6cJmTM386DGXHKTubU\n1XupGc1V3sjs0l44U+VcT4wt/lAjNvxm5suOpDkZALeVAjmRCw7+OC7RHQWa9k0+\nbw8HHa8sHo9gOeL6NlMTOdReJivbPagUvTLrGAMoUgRx5aszPeE4uwc2hGKceeoW\nMPRfwCvocWvk+QIDAQABo1MwUTAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBTA\nephojYn7qwVkDBF9qn1luMrMTjAfBgNVHSMEGDAWgBTAephojYn7qwVkDBF9qn1l\nuMrMTjANBgkqhkiG9w0BAQUFAAOCAQEANeMpauUvXVSOKVCUn5kaFOSPeCpilKIn\nZ57QzxpeR+nBsqTP3UEaBU6bS+5Kb1VSsyShNwrrZHYqLizz/Tt1kL/6cdjHPTfS\ntQWVYrmm3ok9Nns4d0iXrKYgjy6myQzCsplFAMfOEVEiIuCl6rYVSAlk6l5PdPcF\nPseKUgzbFbS9bZvlxrFUaKnjaZC2mqUPuLk/IH2uSrW4nOQdtqvmlKXBx4Ot2/Un\nhw4EbNX/3aBd7YdStysVAq45pmp06drE57xNNB6pXE0zX5IJL4hmXXeXxx12E6nV\n5fEWCRE11azbJHFwLJhWC9kXtNHjUStedejV0NxPNO3CBWaAocvmMw==\n-----END CERTIFICATE-----"
)

type Apns struct {
	DelayTime            uint
	Topic                string
	CheckDelayedInterval uint
	client               *apns2.Client
	db                   *database.Database
	mapMutex             sync.Mutex
	delayedApns          map[database.Registration]time.Time
	RenewTimer           *time.Timer
}

func NewApns(cfg *config.Config, db *database.Database) (apns *Apns) {
	apns = &Apns{
		DelayTime:            cfg.Delay,
		CheckDelayedInterval: cfg.CheckInterval,
		db:                   db,
		mapMutex:             sync.Mutex{},
		delayedApns:          make(map[database.Registration]time.Time),
	}
	log.Debugln("APNS for non NewMessage events will be delayed for", time.Second*time.Duration(apns.DelayTime))

	if cfg.CertificateFileP12 != "" {
		log.Debugf("Loading Certificate at %s", "/etc/xapsd/"+cfg.CertificateFileP12)
		cert, err := certificate.FromP12File("/etc/xapsd/"+cfg.CertificateFileP12, "")
		if err != nil {
			log.Fatal("Cert Error:", err)
		}
		topic, err := topicFromCertificate(cert)
		if err != nil {
			log.Fatalln("Could not parse apns topic from certificate: ", err)
		}
		apns.Topic = topic
		apns.client = apns2.NewClient(cert).Production()
	} else if cfg.CertificateFilePem != "" {
		log.Debugf("Loading Certificate at %s", "/etc/xapsd/"+cfg.CertificateFilePem)
		certData, err := ioutil.ReadFile("/etc/xapsd/" + cfg.CertificateFilePem)
		if err != nil {
			log.Fatal("Cert Error:", err)
		}
		keyData, err := ioutil.ReadFile("/etc/xapsd/" + cfg.CertificateFilePemKey)
		if err != nil {
			log.Fatal("Key Error:", err)
		}
		cert, err := tls.X509KeyPair(certData, keyData)
		if err != nil {
			log.Fatal("Cert Error:", err)
		}
		topic, err := topicFromCertificate(cert)
		if err != nil {
			log.Fatalln("Could not parse apns topic from certificate: ", err)
		}
		apns.Topic = topic
		apns.client = apns2.NewClient(cert).Production()
	} else {
		if cfg.KeyFileKeyId == "" {
			log.Fatalln(errors.New("No KeyFileKeyId  found"))
		}
		if cfg.KeyFileTeamId == "" {
			log.Fatalln(errors.New("No KeyFileTeamId found"))
		}
		if cfg.KeyFileTopic == "" {
			log.Fatalln(errors.New("No KeyFileTopic found"))
		}
		log.Debugln("Loading Keyfile")
		authKey, err := token.AuthKeyFromFile("/etc/xapsd/" + cfg.KeyFileP8)
		if err != nil {
			log.Fatal("Token error:", err)
		}
		apnsToken := &token.Token{
			AuthKey: authKey,
			KeyID:   cfg.KeyFileKeyId,
			// TeamID from developer account (View Account -> Membership)
			TeamID: cfg.KeyFileTeamId,
		}
		apns.Topic = cfg.KeyFileTopic
		apns.client = apns2.NewTokenClient(apnsToken)
	}
	log.Debugln("Topic is", apns.Topic)

	// Get the SystemCertPool, continue with an empty pool on error
	//rootCAs, _ := x509.SystemCertPool()
	//if rootCAs == nil {
	//	rootCAs = x509.NewCertPool()
	//}
	//
	//// Append our cert to the system pool
	//if ok := rootCAs.AppendCertsFromPEM([]byte(GeoTrustCert)); !ok {
	//	log.Infoln("No certs appended, using system certs only")
	//}
	//apns.client.HTTPClient.Transport.(*http2.Transport).TLSClientConfig.RootCAs = rootCAs

	apns.createDelayedNotificationThread()
	return apns
}

func (apns *Apns) createDelayedNotificationThread() {
	delayedNotificationTicker := time.NewTicker(time.Second * time.Duration(apns.CheckDelayedInterval))
	go func() {
		for range delayedNotificationTicker.C {
			apns.checkDelayed()
		}
	}()
}

func (apns *Apns) checkDelayed() {
	log.Debugln("Checking all delayed APNS")
	var sendNow []database.Registration
	apns.mapMutex.Lock()
	for reg, t := range apns.delayedApns {
		log.Debugln("Registration", reg.AccountId, "/", reg.DeviceToken, "has been waiting for", time.Since(t))
		if time.Since(t) > time.Second*time.Duration(apns.DelayTime) {
			sendNow = append(sendNow, reg)
			delete(apns.delayedApns, reg)
		}
	}
	apns.mapMutex.Unlock()
	for _, reg := range sendNow {
		apns.SendNotification(reg, false, "")
	}
}

func (apns *Apns) SendNotification(registration database.Registration, delayed bool, mailbox string) {
	apns.mapMutex.Lock()
	if delayed {
		apns.delayedApns[registration] = time.Now()
		apns.mapMutex.Unlock()
		return
	} else {
		delete(apns.delayedApns, registration)
		apns.mapMutex.Unlock()
	}
	log.Debugln("Sending notification to", registration.AccountId, "/", registration.DeviceToken)

	notification := &apns2.Notification{}
	notification.DeviceToken = registration.DeviceToken
	notification.Topic = apns.Topic
	composedPayload := []byte(`{"aps":{`)
	composedPayload = append(composedPayload, []byte(`"account-id":"`+registration.AccountId+`"`)...)
	if mailbox != "" {
		hash := md5.Sum([]byte(mailbox))
		mailbox_hash := hex.EncodeToString(hash[:])
		composedPayload = append(composedPayload, []byte(`, "m":"`+mailbox_hash+`"`)...)
	}
	composedPayload = append(composedPayload, []byte(`}}`)...)
	notification.Payload = composedPayload
	notification.PushType = apns2.PushTypeBackground
	notification.Expiration = time.Now().Add(24 * time.Hour)
	// set the apns-priority
	//notification.Priority = apns2.PriorityLow

	if log.IsLevelEnabled(log.DebugLevel) {
		dbgstr, _ := notification.MarshalJSON()
		log.Debugf("Sending: %s", dbgstr)
	}
	res, err := apns.client.Push(notification)

	if err != nil {
		log.Fatal("Error:", err)
	}

	switch res.StatusCode {
	case http.StatusOK:
		log.Debugln("Apple returned 200 for notification to", registration.AccountId, "/", registration.DeviceToken)
	case 410:
		// The device token is inactive for the specified topic.
		log.Infoln("Apple returned 410 for notification to", registration.AccountId, "/", registration.DeviceToken)
		apns.db.DeleteIfExistRegistration(registration)
	default:
		log.Errorf("Apple returned a non-200 HTTP status: %v %v %v\n", res.StatusCode, res.ApnsID, res.Reason)
	}
}

func topicFromCertificate(tlsCert tls.Certificate) (string, error) {
	if len(tlsCert.Certificate) > 1 {
		return "", errors.New("found multiple certificates in the cert file - only one is allowed")
	}

	cert, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		log.Fatalln("Could not parse certificate: ", err)
	}

	if len(cert.Subject.Names) == 0 {
		return "", errors.New("Subject.Names is empty")
	}

	if !cert.Subject.Names[0].Type.Equal(oidUid) {
		return "", errors.New("did not find a Subject.Names[0] with type 0.9.2342.19200300.100.1.1")
	}

	return cert.Subject.Names[0].Value.(string), nil
}
