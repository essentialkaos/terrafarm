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
	"strings"

	"pkg.re/essentialkaos/ek.v2/arg"
	"pkg.re/essentialkaos/ek.v2/crypto"
	"pkg.re/essentialkaos/ek.v2/fsutil"
	"pkg.re/essentialkaos/ek.v2/timeutil"
)

// ////////////////////////////////////////////////////////////////////////////////// //

// Preferences contains farm preferences
type Preferences struct {
	TTL      int64
	MaxWait  int64
	Output   string
	Token    string
	Key      string
	Region   string
	NodeSize string
	User     string
	Password string
	Template string
}

// ////////////////////////////////////////////////////////////////////////////////// //

// findAndReadPreferences read preferences from file and command-line arguments
func findAndReadPreferences() *Preferences {
	prefs := &Preferences{
		TTL:      240,
		Region:   "fra1",
		NodeSize: "16gb",
		User:     "builder",
		Password: crypto.GenPassword(18, crypto.STRENGTH_MEDIUM),
		Template: "c6-multiarch",
	}

	prefsFile := fsutil.ProperPath("FRS", []string{
		".terrafarm",
		"~/.terrafarm",
	})

	if prefsFile != "" {
		applyPreferencesFromFile(prefs, prefsFile)
	}

	applyPreferencesFromEnvironment(prefs)
	applyPreferencesFromArgs(prefs)
	validatePreferences(prefs)

	return prefs
}

// applyPreferencesFromFile read arguments from file and add it to preferences struct
func applyPreferencesFromFile(prefs *Preferences, file string) {
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
		case PREFS_TTL:
			prefs.TTL = timeutil.ParseDuration(propVal) / 60

			if prefs.TTL == 0 {
				printError("Incorrect ttl property in %s file", file)
			}

		case PREFS_MAX_WAIT, "max_wait", "maxwait":
			prefs.MaxWait = timeutil.ParseDuration(propVal) / 60

			if prefs.MaxWait == 0 {
				printError("Incorrect max-wait property in %s file", file)
			}

		case PREFS_OUTPUT:
			prefs.Output = propVal

		case PREFS_TOKEN:
			prefs.Token = propVal

		case PREFS_KEY:
			prefs.Key = propVal

		case PREFS_REGION:
			prefs.Region = propVal

		case PREFS_NODE_SIZE, "node_size", "nodesize":
			prefs.NodeSize = propVal

		case PREFS_USER:
			prefs.User = propVal

		case PREFS_TEMPLATE:
			prefs.Template = propVal

		default:
			printWarn("Unknown property %s in %s file", propName, file)
		}
	}
}

// applyPreferencesFromArgs add values from command-line arguments to preferences struct
func applyPreferencesFromArgs(prefs *Preferences) {
	if arg.Has(ARG_TTL) {
		prefs.TTL = timeutil.ParseDuration(arg.GetS(ARG_TTL)) / 60

		if prefs.TTL == 0 {
			printError("Incorrect ttl property in command-line arguments")
		}
	}

	if arg.Has(ARG_MAX_WAIT) {
		prefs.MaxWait = timeutil.ParseDuration(arg.GetS(ARG_MAX_WAIT)) / 60

		if prefs.MaxWait == 0 {
			printError("Incorrect max-wait property in command-line arguments")
		}
	}

	if arg.Has(ARG_OUTPUT) {
		prefs.Output = arg.GetS(ARG_OUTPUT)
	}

	if arg.Has(ARG_TOKEN) {
		prefs.Token = arg.GetS(ARG_TOKEN)
	}

	if arg.Has(ARG_KEY) {
		prefs.Key = arg.GetS(ARG_KEY)
	}

	if arg.Has(ARG_REGION) {
		prefs.Region = arg.GetS(ARG_REGION)
	}

	if arg.Has(ARG_NODE_SIZE) {
		prefs.NodeSize = arg.GetS(ARG_NODE_SIZE)
	}

	if arg.Has(ARG_USER) {
		prefs.User = arg.GetS(ARG_USER)
	}

	if arg.Has(ARG_PASSWORD) {
		prefs.Password = arg.GetS(ARG_PASSWORD)
	}
}

func applyPreferencesFromEnvironment(prefs *Preferences) {
	if envMap[EV_TTL] != "" {
		prefs.TTL = timeutil.ParseDuration(envMap[EV_TTL]) / 60

		if prefs.TTL == 0 {
			printError("Incorrect %s property in environment variables", EV_TTL)
		}
	}

	if envMap[EV_MAX_WAIT] != "" {
		prefs.MaxWait = timeutil.ParseDuration(envMap[EV_MAX_WAIT]) / 60

		if prefs.MaxWait == 0 {
			printError("Incorrect %s property in environment variables", EV_TTL)
		}
	}

	if envMap[EV_OUTPUT] != "" {
		prefs.Output = envMap[EV_OUTPUT]
	}

	if envMap[EV_TOKEN] != "" {
		prefs.Token = envMap[EV_TOKEN]
	}

	if envMap[EV_KEY] != "" {
		prefs.Key = envMap[EV_KEY]
	}

	if envMap[EV_REGION] != "" {
		prefs.Region = envMap[EV_REGION]
	}

	if envMap[EV_NODE_SIZE] != "" {
		prefs.NodeSize = envMap[EV_NODE_SIZE]
	}

	if envMap[EV_USER] != "" {
		prefs.User = envMap[EV_USER]
	}

	if envMap[EV_PASSWORD] != "" {
		prefs.Password = envMap[EV_PASSWORD]
	}

	if envMap[EV_TEMPLATE] != "" {
		prefs.Template = envMap[EV_TEMPLATE]
	}
}

// validatePreferences validate basic preferences
func validatePreferences(prefs *Preferences) {
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

	templateDir := getDataDir() + "/" + prefs.Template

	if !fsutil.IsExist(templateDir) {
		printError("Directory with template %s is not exist", prefs.Template)
		hasErrors = true
	} else {
		if !fsutil.IsReadable(templateDir) {
			printError("Directory with template %s is not readable", prefs.Template)
			hasErrors = true
		}

		if fsutil.IsDir(templateDir) {
			if fsutil.IsEmptyDir(templateDir) {
				printError("Directory with template %s is empty", prefs.Template)
				hasErrors = true
			}
		} else {
			printError("Target %s is not a directory", templateDir)
			hasErrors = true
		}
	}

	if hasErrors {
		os.Exit(1)
	}
}

// ////////////////////////////////////////////////////////////////////////////////// //
