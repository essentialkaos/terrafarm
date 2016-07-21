// Package do provides simple methods for checking DO preferencies
package do

// ////////////////////////////////////////////////////////////////////////////////// //
//                                                                                    //
//                     Copyright (c) 2009-2016 Essential Kaos                         //
//      Essential Kaos Open Source License <http://essentialkaos.com/ekol?en>         //
//                                                                                    //
// ////////////////////////////////////////////////////////////////////////////////// //

import (
	"pkg.re/essentialkaos/ek.v3/req"
)

// ////////////////////////////////////////////////////////////////////////////////// //

const (
	STATUS_OK     StatusCode = 0
	STATUS_NOT_OK            = 1
	STATUS_ERROR             = 2
)

// DO_API is DO API url
const DO_API = "https://api.digitalocean.com/v2"

// USER_AGENT is user agent used for all requests
const USER_AGENT = "terrafarm"

// ////////////////////////////////////////////////////////////////////////////////// //

type StatusCode uint8

type Account struct {
	Status string `json:"status"`
}

type AccountInfo struct {
	Account *Account `json:"account"`
}

type KeysInfo struct {
	Keys []*Key `json:"ssh_keys"`
}

type Key struct {
	Fingerprint string `json:"fingerprint"`
}

type RegionsInfo struct {
	Regions []*Region `json:"regions"`
}

type Region struct {
	Slug string `json:"slug"`
}

type SizesInfo struct {
	Sizes []*Region `json:"sizes"`
}

type Size struct {
	Slug string `json:"slug"`
}

// ////////////////////////////////////////////////////////////////////////////////// //

// IsValidToken return true if token valid and account is active
func IsValidToken(token string) StatusCode {
	resp, err := doAPIRequest(token, "/account")

	if err != nil {
		return STATUS_ERROR
	}

	accountInfo := &AccountInfo{}

	err = resp.JSON(accountInfo)

	if err != nil {
		return STATUS_ERROR
	}

	if accountInfo.Account.Status == "active" {
		return STATUS_OK
	}

	return STATUS_NOT_OK
}

// IsFingerprintValid return tru if provate key with given fingerprint is
// present in Digital Ocean account
func IsFingerprintValid(token, fingerprint string) StatusCode {
	resp, err := doAPIRequest(token, "/account/keys")

	if err != nil {
		return STATUS_ERROR
	}

	keysInfo := &KeysInfo{}

	err = resp.JSON(keysInfo)

	if err != nil {
		return STATUS_ERROR
	}

	for _, key := range keysInfo.Keys {
		if key.Fingerprint == fingerprint {
			return STATUS_OK
		}
	}

	return STATUS_NOT_OK
}

// IsRegionValid return true if region with given slug is present
// on Digital Ocean
func IsRegionValid(token, slug string) StatusCode {
	resp, err := doAPIRequest(token, "/regions")

	if err != nil {
		return STATUS_ERROR
	}

	regionsInfo := &RegionsInfo{}

	err = resp.JSON(regionsInfo)

	if err != nil {
		return STATUS_ERROR
	}

	for _, region := range regionsInfo.Regions {
		if region.Slug == slug {
			return STATUS_OK
		}
	}

	return STATUS_NOT_OK
}

// IsSizeValid return true if size with given slug is present
// on Digital Ocean
func IsSizeValid(token, slug string) StatusCode {
	resp, err := doAPIRequest(token, "/sizes")

	if err != nil {
		return STATUS_ERROR
	}

	sizesInfo := &SizesInfo{}

	err = resp.JSON(sizesInfo)

	if err != nil {
		return STATUS_ERROR
	}

	for _, size := range sizesInfo.Sizes {
		if size.Slug == slug {
			return STATUS_OK
		}
	}

	return STATUS_NOT_OK
}

// ////////////////////////////////////////////////////////////////////////////////// //

// doAPIRequest execute request to DO API
func doAPIRequest(token, url string) (*req.Response, error) {
	return req.Request{
		URL:         DO_API + url,
		UserAgent:   USER_AGENT,
		ContentType: "application/json",
		Headers: map[string]string{
			"Authorization": "Bearer " + token,
		},
	}.Get()
}
