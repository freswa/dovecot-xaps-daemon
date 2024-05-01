package apple_xserver_certs

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/pem"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

func NewCerts(username string, passwordhash string) *Certificates {
	certs := &Certificates{}
	generatePrivateKeys(certs)
	body := createCertRequestBody(certs, username, passwordhash)
	response := sendRequest(body, true)
	return handleResponse(certs, response)
}

func RenewCerts(certs *Certificates, username string, passwordhash string) *Certificates {
	body := createCertRequestBody(certs, username, passwordhash)
	response := sendRequest(body, false)
	return handleResponse(certs, response)
}

func handleResponse(certs *Certificates, response []byte) *Certificates {
	responseBody, err := parseResponse(response)
	if err != nil {
		log.Fatal(err)
	}
	if responseBody.Response.Status.ErrorCode != 0 {
		log.Fatalf("Error %d while retrieving certificates:\n%+v", responseBody.Response.Status.ErrorCode, responseBody)
	}
	calendarCertDER, _ := pem.Decode([]byte(responseBody.Response.Certificates[0].Certificate))
	certs.Calendar.Certificate = make([][]byte, 1)
	certs.Calendar.Certificate[0] = calendarCertDER.Bytes
	contactCertDER, _ := pem.Decode([]byte(responseBody.Response.Certificates[1].Certificate))
	certs.Contact.Certificate = make([][]byte, 1)
	certs.Contact.Certificate[0] = contactCertDER.Bytes
	mailCertDER, _ := pem.Decode([]byte(responseBody.Response.Certificates[2].Certificate))
	certs.Mail.Certificate = make([][]byte, 1)
	certs.Mail.Certificate[0] = mailCertDER.Bytes
	mgmtCertDER, _ := pem.Decode([]byte(responseBody.Response.Certificates[3].Certificate))
	certs.Mgmt.Certificate = make([][]byte, 1)
	certs.Mgmt.Certificate[0] = mgmtCertDER.Bytes
	alertsCertDER, _ := pem.Decode([]byte(responseBody.Response.Certificates[4].Certificate))
	certs.Alerts.Certificate = make([][]byte, 1)
	certs.Alerts.Certificate[0] = alertsCertDER.Bytes

	return certs
}

func sendRequest(reqBody []byte, newCerts bool) (respBody []byte) {
	r := bytes.NewReader(reqBody)
	url := "https://identity.apple.com/pushcert/caservice/renew"
	if newCerts {
		url = "https://identity.apple.com/pushcert/caservice/new"
	}

	req, err := http.NewRequest("POST", url, r)
	if err != nil {
		log.Fatalln(err)
	}

	req.Header.Set("Content-Type", "text/x-xml-plist")
	req.Header.Set("User-Agent", "Servermgrd%20Plugin/6.0 CFNetwork/811.11 Darwin/16.7.0 (x86_64)")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-us")

	req.Close = true

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := new(tls.Dialer).DialContext(
		ctx,
		"tcp",
		req.URL.Host+":443",
	)
	if err != nil {
		log.Fatalln(err) // TODO: Handle error properly
	}
	defer func() {
		_ = conn.Close() //nolint:errcheck,gosec // Ignored on purpose
	}()

	if err := req.Write(conn); err != nil {
		log.Fatalln(err) // TODO: Handle error properly
	}

	buf, err := io.ReadAll(io.LimitReader(conn, 1<<10))
	if err != nil {
		log.Fatalln(err) // TODO: Handle error properly
	}

	const (
		cr = "\r"
		nl = "\n"
	)
	for _, ign := range []string{
		"1;: mode=block",
		"max-age=31536000;: includeSubdomains",
	} {
		buf = bytes.Replace(buf, []byte(nl+ign+cr+nl), []byte(nl), 1)
	}

	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(buf)), req)
	if err != nil {
		log.Fatalln(err) // TODO: Handle error properly
	}
	defer func() {
		_ = resp.Body.Close() //nolint:errcheck,gosec // Ignored on purpose
	}()

	respBody, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	if resp.StatusCode != 200 {
		log.Fatalf("Apple didn't return 200: %s", respBody)
	}
	return
}
