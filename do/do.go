// Package do provides simple methods for checking DO preferencies
package do

// ////////////////////////////////////////////////////////////////////////////////// //
//                                                                                    //
//                     Copyright (c) 2009-2016 Essential Kaos                         //
//      Essential Kaos Open Source License <http://essentialkaos.com/ekol?en>         //
//                                                                                    //
// ////////////////////////////////////////////////////////////////////////////////// //

import (
	"pkg.re/essentialkaos/ek.v1/req"
)

// ////////////////////////////////////////////////////////////////////////////////// //

// DO_API is DO API url
const DO_API = "https://api.digitalocean.com/v2"

// USER_AGENT is user agent used for all requests
const USER_AGENT = "terrafarm"

// ////////////////////////////////////////////////////////////////////////////////// //

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
func IsValidToken(token string) bool {
	resp, err := doAPIRequest(token, "/account")

	if err != nil {
		return false
	}

	accountInfo := &AccountInfo{}

	err = resp.JSON(accountInfo)

	if err != nil {
		return false
	}

	return accountInfo.Account.Status == "active"
}

// IsFingerprintValid return tru if provate key with given fingerprint is
// present in Digital Ocean account
func IsFingerprintValid(token, fingerprint string) bool {
	resp, err := doAPIRequest(token, "/account/keys")

	if err != nil {
		return false
	}

	keysInfo := &KeysInfo{}

	err = resp.JSON(keysInfo)

	if err != nil {
		return false
	}

	for _, key := range keysInfo.Keys {
		if key.Fingerprint == fingerprint {
			return true
		}
	}

	return false
}

// IsRegionValid return true if region with given slug is present
// on Digital Ocean
func IsRegionValid(token, slug string) bool {
	resp, err := doAPIRequest(token, "/regions")

	if err != nil {
		return false
	}

	regionsInfo := &RegionsInfo{}

	err = resp.JSON(regionsInfo)

	if err != nil {
		return false
	}

	for _, region := range regionsInfo.Regions {
		if region.Slug == slug {
			return true
		}
	}

	return false
}

// IsSizeValid return true if size with given slug is present
// on Digital Ocean
func IsSizeValid(token, slug string) bool {
	resp, err := doAPIRequest(token, "/sizes")

	if err != nil {
		return false
	}

	sizesInfo := &SizesInfo{}

	err = resp.JSON(sizesInfo)

	if err != nil {
		return false
	}

	for _, size := range sizesInfo.Sizes {
		if size.Slug == slug {
			return true
		}
	}

	return false
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
