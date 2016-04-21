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

const DO_API = "https://api.digitalocean.com/v2"
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
