package cli

// ////////////////////////////////////////////////////////////////////////////////// //
//                                                                                    //
//                     Copyright (c) 2009-2016 Essential Kaos                         //
//      Essential Kaos Open Source License <http://essentialkaos.com/ekol?en>         //
//                                                                                    //
// ////////////////////////////////////////////////////////////////////////////////// //

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"pkg.re/essentialkaos/ek.v5/arg"
	"pkg.re/essentialkaos/ek.v5/fsutil"
	"pkg.re/essentialkaos/ek.v5/passwd"
	"pkg.re/essentialkaos/ek.v5/terminal"
	"pkg.re/essentialkaos/ek.v5/timeutil"

	"gopkg.in/hlandau/passlib.v1/hash/sha2crypt"
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
		Password: passwd.GenPassword(18, passwd.STRENGTH_MEDIUM),
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
				terminal.PrintErrorMessage("Incorrect ttl property in %s file", file)
			}

		case PREFS_MAX_WAIT, "max_wait", "maxwait":
			prefs.MaxWait = timeutil.ParseDuration(propVal) / 60

			if prefs.MaxWait == 0 {
				terminal.PrintErrorMessage("Incorrect max-wait property in %s file", file)
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
			terminal.PrintWarnMessage("Unknown property %s in %s file", propName, file)
		}
	}
}

// applyPreferencesFromArgs add values from command-line arguments to preferences struct
func applyPreferencesFromArgs(prefs *Preferences) {
	if arg.Has(ARG_TTL) {
		prefs.TTL = timeutil.ParseDuration(arg.GetS(ARG_TTL)) / 60

		if prefs.TTL == 0 {
			terminal.PrintErrorMessage("Incorrect ttl property in command-line arguments")
		}
	}

	if arg.Has(ARG_MAX_WAIT) {
		prefs.MaxWait = timeutil.ParseDuration(arg.GetS(ARG_MAX_WAIT)) / 60

		if prefs.MaxWait == 0 {
			terminal.PrintErrorMessage("Incorrect max-wait property in command-line arguments")
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
			terminal.PrintErrorMessage("Incorrect %s property in environment variables", EV_TTL)
		}
	}

	if envMap[EV_MAX_WAIT] != "" {
		prefs.MaxWait = timeutil.ParseDuration(envMap[EV_MAX_WAIT]) / 60

		if prefs.MaxWait == 0 {
			terminal.PrintErrorMessage("Incorrect %s property in environment variables", EV_TTL)
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
		terminal.PrintErrorMessage("Property token must be set")
		hasErrors = true
	}

	if prefs.Region == "" {
		terminal.PrintErrorMessage("Property region must be set")
		hasErrors = true
	}

	if prefs.NodeSize == "" {
		terminal.PrintErrorMessage("Property node-size must be set")
		hasErrors = true
	}

	if prefs.User == "" {
		terminal.PrintErrorMessage("Property user must be set")
		hasErrors = true
	}

	if prefs.Key == "" {
		terminal.PrintErrorMessage("Property key must be set")
		hasErrors = true
	} else {
		if !fsutil.IsExist(prefs.Key) {
			terminal.PrintErrorMessage("Private key file %s does not exits", prefs.Key)
			hasErrors = true
		}

		if !fsutil.IsReadable(prefs.Key) {
			terminal.PrintErrorMessage("Private key file %s must be readable", prefs.Key)
			hasErrors = true
		}

		if !fsutil.IsNonEmpty(prefs.Key) {
			terminal.PrintErrorMessage("Private key file %s does not contain any data", prefs.Key)
			hasErrors = true
		}

		if !fsutil.IsExist(prefs.Key + ".pub") {
			terminal.PrintErrorMessage("Public key file %s.pub does not exits", prefs.Key)
			hasErrors = true
		}

		if !fsutil.IsReadable(prefs.Key + ".pub") {
			terminal.PrintErrorMessage("Public key file %s.pub must be readable", prefs.Key)
			hasErrors = true
		}

		if !fsutil.IsNonEmpty(prefs.Key + ".pub") {
			terminal.PrintErrorMessage("Public key file %s.pub does not contain any data", prefs.Key)
			hasErrors = true
		}
	}

	templateDir := getDataDir() + "/" + prefs.Template

	if !fsutil.IsExist(templateDir) {
		terminal.PrintErrorMessage("Directory with template %s is not exist", prefs.Template)
		hasErrors = true
	} else {
		if !fsutil.IsReadable(templateDir) {
			terminal.PrintErrorMessage("Directory with template %s is not readable", prefs.Template)
			hasErrors = true
		}

		if fsutil.IsDir(templateDir) {
			if fsutil.IsEmptyDir(templateDir) {
				terminal.PrintErrorMessage("Directory with template %s is empty", prefs.Template)
				hasErrors = true
			}
		} else {
			terminal.PrintErrorMessage("Target %s is not a directory", templateDir)
			hasErrors = true
		}
	}

	if hasErrors {
		os.Exit(1)
	}
}

// ////////////////////////////////////////////////////////////////////////////////// //

func (p *Preferences) GetVariablesData() (string, error) {
	var result string

	auth, err := sha2crypt.NewCrypter512(5000).Hash(p.Password)

	if err != nil {
		return "", err
	}

	fingerpint, err := getFingerprint(p.Key + ".pub")

	if err != nil {
		return "", err
	}

	result += fmt.Sprintf("%s = \"%s\"\n", "token", p.Token)
	result += fmt.Sprintf("%s = \"%s\"\n", "auth", auth)
	result += fmt.Sprintf("%s = \"%s\"\n", "fingerprint", fingerpint)
	result += fmt.Sprintf("%s = \"%s\"\n", "key", p.Key)
	result += fmt.Sprintf("%s = \"%s\"\n", "user", p.User)
	result += fmt.Sprintf("%s = \"%s\"\n", "password", p.Password)

	if p.Region != "" {
		result += fmt.Sprintf("%s = \"%s\"\n", "region", p.Region)
	}

	if p.NodeSize != "" {
		result += fmt.Sprintf("%s = \"%s\"\n", "node_size", p.NodeSize)
	}

	return result, nil
}

// ////////////////////////////////////////////////////////////////////////////////// //
