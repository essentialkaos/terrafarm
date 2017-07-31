package terraform

// ////////////////////////////////////////////////////////////////////////////////// //
//                                                                                    //
//                     Copyright (c) 2009-2017 ESSENTIAL KAOS                         //
//        Essential Kaos Open Source License <https://essentialkaos.com/ekol>         //
//                                                                                    //
// ////////////////////////////////////////////////////////////////////////////////// //

import (
	"pkg.re/essentialkaos/ek.v9/jsonutil"
)

// ////////////////////////////////////////////////////////////////////////////////// //

// TFState contains terraform state
type TFState struct {
	Modules []*TFModule `json:"modules"`
}

// TFModule is terraform module
type TFModule struct {
	Resources map[string]*TFResource `json:"resources"`
}

// TFResource is  terraform resource
type TFResource struct {
	Type string          `json:"type"`
	Info *TFResourceInfo `json:"primary"`
}

// TFResourceInfo contains info about terraform resource
type TFResourceInfo struct {
	ID         string                `json:"id"`
	Attributes *TFResourceAttributes `json:"attributes"`
}

// TFResourceAttributes contains terraform resource attributes
type TFResourceAttributes struct {
	ID     string `json:"id"`
	IP     string `json:"ipv4_address"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// ////////////////////////////////////////////////////////////////////////////////// //

// ReadState read and parse terraform state file
func ReadState(file string) (*TFState, error) {
	state := &TFState{}

	err := jsonutil.DecodeFile(file, state)

	if err != nil {
		return nil, err
	}

	return state, nil
}
