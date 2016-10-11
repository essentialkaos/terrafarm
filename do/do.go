// Package do provides simple methods for checking DO preferencies
package do

// ////////////////////////////////////////////////////////////////////////////////// //
//                                                                                    //
//                     Copyright (c) 2009-2016 Essential Kaos                         //
//      Essential Kaos Open Source License <http://essentialkaos.com/ekol?en>         //
//                                                                                    //
// ////////////////////////////////////////////////////////////////////////////////// //

import (
	"fmt"
	"strconv"
	"strings"

	"pkg.re/essentialkaos/ek.v5/req"
)

// ////////////////////////////////////////////////////////////////////////////////// //

const (
	STATUS_OK     StatusCode = 0
	STATUS_NOT_OK            = 1
	STATUS_ERROR             = 2
)

// DO_API is DO API url
const DO_API = "https://api.digitalocean.com/v2"

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

type DropletsInfo struct {
	Droplets []*Droplet `json:"droplets"`
}

type Droplet struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ////////////////////////////////////////////////////////////////////////////////// //

// IsValidToken return true if token valid and account is active
func IsValidToken(token string) StatusCode {
	resp, err := req.Request{
		URL:         DO_API + "/account",
		ContentType: req.CONTENT_TYPE_JSON,
		Headers:     getAuthHeaders(token),
	}.Get()

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
	resp, err := req.Request{
		URL:         DO_API + "/account/keys",
		ContentType: req.CONTENT_TYPE_JSON,
		Headers:     getAuthHeaders(token),
	}.Get()

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
	resp, err := req.Request{
		URL:         DO_API + "/regions",
		ContentType: req.CONTENT_TYPE_JSON,
		Headers:     getAuthHeaders(token),
	}.Get()

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
	resp, err := req.Request{
		URL:         DO_API + "/sizes",
		ContentType: req.CONTENT_TYPE_JSON,
		Headers:     getAuthHeaders(token),
	}.Get()

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

// DestroyTerrafarmDroplets destroy terrafarm droplets
func DestroyTerrafarmDroplets(token string) error {
	droplets, err := GetTerrafarmDropletsList(token)

	if err != nil {
		return err
	}

	if len(droplets) == 0 {
		return nil
	}

	for dropletName, dropletID := range droplets {
		resp, err := req.Request{
			URL:         DO_API + "/droplets/" + strconv.Itoa(dropletID),
			ContentType: req.CONTENT_TYPE_JSON,
			Headers:     getAuthHeaders(token),
		}.Delete()

		if err != nil {
			return fmt.Errorf("Can't send request to DigitalOcean API: %v", err)
		}

		if resp.StatusCode != 204 {
			return fmt.Errorf(
				"Can't destroy droplet %s - DigitalOcean return status code %d",
				dropletName, resp.StatusCode,
			)
		}
	}

	return nil
}

// GetTerrafarmDropletsList return map name->id
func GetTerrafarmDropletsList(token string) (map[string]int, error) {
	var result = make(map[string]int)

	resp, err := req.Request{
		URL:         DO_API + "/droplets",
		ContentType: req.CONTENT_TYPE_JSON,

		Query: req.Query{
			"page":     "1",
			"per_page": "999",
		},

		Headers: getAuthHeaders(token),
	}.Get()

	if err != nil {
		return result, fmt.Errorf("Can't fetch droplets list from DigitalOcean API: %v", err)
	}

	dropletsInfo := &DropletsInfo{}

	err = resp.JSON(dropletsInfo)

	if err != nil {
		return result, fmt.Errorf("Can't decode DigitalOcean API response: %v", err)
	}

	for _, droplet := range dropletsInfo.Droplets {
		if strings.HasPrefix(strings.ToLower(droplet.Name), "terrafarm") {
			result[droplet.Name] = droplet.ID
		}
	}

	return result, nil
}

// ////////////////////////////////////////////////////////////////////////////////// //

func getAuthHeaders(token string) req.Headers {
	return req.Headers{"Authorization": "Bearer " + token}
}
