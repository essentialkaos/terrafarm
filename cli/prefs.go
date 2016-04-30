package cli

// ////////////////////////////////////////////////////////////////////////////////// //
//                                                                                    //
//                     Copyright (c) 2009-2016 Essential Kaos                         //
//      Essential Kaos Open Source License <http://essentialkaos.com/ekol?en>         //
//                                                                                    //
// ////////////////////////////////////////////////////////////////////////////////// //

import (
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"pkg.re/essentialkaos/ek.v1/arg"
	"pkg.re/essentialkaos/ek.v1/crypto"
	"pkg.re/essentialkaos/ek.v1/fsutil"
)

// ////////////////////////////////////////////////////////////////////////////////// //

type Prefs struct {
	TTL      int64
	Output   string
	Token    string
	Key      string
	Region   string
	NodeSize string
	User     string
	Password string
	Farm     string
}

// ////////////////////////////////////////////////////////////////////////////////// //

// findAndReadPrefs read prefs from file and command-line arguments
func findAndReadPrefs() *Prefs {
	prefs := &Prefs{
		Password: crypto.GenPassword(18, crypto.STRENGTH_MEDIUM),
	}

	prefsFile := fsutil.ProperPath("FRS", []string{
		".terrafarm",
		"~/.terrafarm",
	})

	if prefsFile != "" {
		applyPrefsFromFile(prefs, prefsFile)
	}

	applyPrefsFromArgs(prefs)
	validatePrefs(prefs)

	return prefs
}

// applyPrefsFromFile read arguments from file and add it to prefs struct
func applyPrefsFromFile(prefs *Prefs, file string) {
	data, err := ioutil.ReadFile(file)

	if err != nil {
		return
	}

	for _, prop := range strings.Split(string(data), "\n") {
		prop = strings.TrimSpace(prop)

		if prop == "" {
			continue
		}

		propSlice := strings.Split(prop, ":")

		if len(propSlice) < 2 {
			continue
		}

		propName := propSlice[0]
		propVal := strings.TrimSpace(strings.Join(propSlice[1:], ":"))

		switch strings.ToLower(propName) {
		case "ttl":
			prefs.TTL = parseTTL(propVal)

			if prefs.TTL == -1 {
				printError("Can't parse ttl property in %s file", file)
			}

		case "output":
			prefs.Output = propVal

		case "token":
			prefs.Token = propVal

		case "key":
			prefs.Key = propVal

		case "region":
			prefs.Region = propVal

		case "node_size", "node-size":
			prefs.NodeSize = propVal

		case "user":
			prefs.User = propVal

		case "farm":
			prefs.Farm = propVal

		default:
			printWarn("Unknown property %s in %s file", propName, file)
		}
	}
}

// applyPrefsFromArgs add values from command-line arguments to prefs struct
func applyPrefsFromArgs(prefs *Prefs) {
	if arg.Has(ARG_TTL) {
		prefs.TTL = parseTTL(arg.GetS(ARG_TTL))

		if prefs.TTL == -1 {
			printError("Can't parse ttl property from command-line arguments")
		}
	}

	if arg.GetS(ARG_OUTPUT) != "" {
		prefs.Output = arg.GetS(ARG_OUTPUT)
	}

	if arg.GetS(ARG_TOKEN) != "" {
		prefs.Token = arg.GetS(ARG_TOKEN)
	}

	if arg.GetS(ARG_KEY) != "" {
		prefs.Key = arg.GetS(ARG_KEY)
	}

	if arg.GetS(ARG_REGION) != "" {
		prefs.Region = arg.GetS(ARG_REGION)
	}

	if arg.GetS(ARG_NODE_SIZE) != "" {
		prefs.NodeSize = arg.GetS(ARG_NODE_SIZE)
	}

	if arg.GetS(ARG_USER) != "" {
		prefs.User = arg.GetS(ARG_USER)
	}

	if arg.GetS(ARG_PASSWORD) != "" {
		prefs.Password = arg.GetS(ARG_PASSWORD)
	}

	if arg.GetS(ARG_FARM) != "" {
		prefs.Farm = arg.GetS(ARG_FARM)
	}
}

// parseTTL parse ttl string and return as minutes
func parseTTL(ttl string) int64 {
	var ttlVal int64
	var mult int64
	var err error

	switch {
	case strings.HasSuffix(ttl, "d"):
		ttlVal, err = strconv.ParseInt(strings.TrimRight(ttl, "d"), 10, 64)
		mult = 1440

	case strings.HasSuffix(ttl, "h"):
		ttlVal, err = strconv.ParseInt(strings.TrimRight(ttl, "h"), 10, 64)
		mult = 60

	case strings.HasSuffix(ttl, "m"):
		ttlVal, err = strconv.ParseInt(strings.TrimRight(ttl, "m"), 10, 64)
		mult = 1

	default:
		ttlVal, err = strconv.ParseInt(ttl, 10, 64)
		mult = 1
	}

	if err != nil {
		return -1
	}

	return ttlVal * mult
}

// validatePrefs validate basic preferencies
func validatePrefs(prefs *Prefs) {
	hasErrors := false

	if prefs.Token == "" {
		printError("Property token must be set")
		hasErrors = true
	}

	if prefs.Region == "" {
		printError("Property region must be set")
		hasErrors = true
	}

	if prefs.NodeSize == "" {
		printError("Property node-size must be set")
		hasErrors = true
	}

	if prefs.User == "" {
		printError("Property user must be set")
		hasErrors = true
	}

	if prefs.Key == "" {
		printError("Property key must be set")
		hasErrors = true
	} else {
		if !fsutil.IsExist(prefs.Key) {
			printError("Private key file %s does not exits", prefs.Key)
			hasErrors = true
		}

		if !fsutil.IsReadable(prefs.Key) {
			printError("Private key file %s must be readable", prefs.Key)
			hasErrors = true
		}

		if !fsutil.IsNonEmpty(prefs.Key) {
			printError("Private key file %s does not contain any data", prefs.Key)
			hasErrors = true
		}

		if !fsutil.IsExist(prefs.Key + ".pub") {
			printError("Public key file %s.pub does not exits", prefs.Key)
			hasErrors = true
		}

		if !fsutil.IsReadable(prefs.Key + ".pub") {
			printError("Public key file %s.pub must be readable", prefs.Key)
			hasErrors = true
		}

		if !fsutil.IsNonEmpty(prefs.Key + ".pub") {
			printError("Public key file %s.pub does not contain any data", prefs.Key)
			hasErrors = true
		}
	}

	farmDataDir := getDataDir() + "/" + prefs.Farm

	if !fsutil.IsExist(farmDataDir) {
		printError("Directory with farm data %s is not exist", farmDataDir)
		hasErrors = true
	} else {
		if !fsutil.IsReadable(farmDataDir) {
			printError("Directory with farm data %s is not readable", farmDataDir)
			hasErrors = true
		}

		if fsutil.IsDir(farmDataDir) {
			if fsutil.IsEmptyDir(farmDataDir) {
				printError("Directory with farm data %s is empty", farmDataDir)
				hasErrors = true
			}
		} else {
			printError("Target %s is not a directory", farmDataDir)
			hasErrors = true
		}
	}

	if hasErrors {
		os.Exit(1)
	}
}

// ////////////////////////////////////////////////////////////////////////////////// //
