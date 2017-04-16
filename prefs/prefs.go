package prefs

// ////////////////////////////////////////////////////////////////////////////////// //
//                                                                                    //
//                     Copyright (c) 2009-2017 ESSENTIAL KAOS                         //
//        Essential Kaos Open Source License <https://essentialkaos.com/ekol>         //
//                                                                                    //
// ////////////////////////////////////////////////////////////////////////////////// //

import (
	"crypto"
	"fmt"
	"io/ioutil"
	"strings"

	"pkg.re/essentialkaos/ek.v8/arg"
	"pkg.re/essentialkaos/ek.v8/env"
	"pkg.re/essentialkaos/ek.v8/fsutil"
	"pkg.re/essentialkaos/ek.v8/passwd"
	"pkg.re/essentialkaos/ek.v8/timeutil"

	"gopkg.in/hlandau/passlib.v1/hash/sha2crypt"

	sshkey "github.com/yosida95/golang-sshkey"
)

// ////////////////////////////////////////////////////////////////////////////////// //

// List of supported environment variables
const (
	EV_TTL       = "TERRAFARM_TTL"
	EV_MAX_WAIT  = "TERRAFARM_MAX_WAIT"
	EV_OUTPUT    = "TERRAFARM_OUTPUT"
	EV_TEMPLATE  = "TERRAFARM_TEMPLATE"
	EV_TOKEN     = "TERRAFARM_TOKEN"
	EV_KEY       = "TERRAFARM_KEY"
	EV_REGION    = "TERRAFARM_REGION"
	EV_NODE_SIZE = "TERRAFARM_NODE_SIZE"
	EV_USER      = "TERRAFARM_USER"
	EV_PASSWORD  = "TERRAFARM_PASSWORD"
)

// List of supported preferences
const (
	TTL       = "ttl"
	MAX_WAIT  = "max-wait"
	OUTPUT    = "output"
	TEMPLATE  = "template"
	TOKEN     = "token"
	KEY       = "key"
	REGION    = "region"
	NODE_SIZE = "node-size"
	USER      = "user"
	PASSWORD  = "password"
)

// List of supported command-line arguments
const (
	ARG_TTL       = "t:ttl"
	ARG_OUTPUT    = "o:output"
	ARG_TOKEN     = "T:token"
	ARG_KEY       = "K:key"
	ARG_REGION    = "R:region"
	ARG_NODE_SIZE = "N:node-size"
	ARG_USER      = "U:user"
	ARG_PASSWORD  = "P:password"
	ARG_MAX_WAIT  = "w:max-wait"
)

// ////////////////////////////////////////////////////////////////////////////////// //

// Preferences contains farm preferences
type Preferences struct {
	TTL         int64  `json:"ttl"`
	MaxWait     int64  `json:"max_wait"`
	Output      string `json:"output"`
	Token       string `json:"token"`
	Key         string `json:"key"`
	Fingerprint string `json:"fingerprint"`
	Region      string `json:"region"`
	NodeSize    string `json:"node_size"`
	User        string `json:"user"`
	Password    string `json:"password"`
	Template    string `json:"template"`
}

// ////////////////////////////////////////////////////////////////////////////////// //

// FindAndReadPreferences read preferences from file and command-line arguments
func FindAndReadPreferences(dataDir string) (*Preferences, []error) {
	var err error

	// Create preferences width default values
	prefs := &Preferences{
		TTL:      240,
		Region:   "fra1",
		NodeSize: "16gb",
		User:     "builder",
		Password: passwd.GenPassword(18, passwd.STRENGTH_MEDIUM),
	}

	prefsFile := fsutil.ProperPath("FRS", []string{
		".terrafarm",
		"~/.terrafarm",
	})

	if prefsFile != "" {
		err = applyPreferencesFromFile(prefs, prefsFile)

		if err != nil {
			return nil, []error{err}
		}
	}

	err = applyPreferencesFromEnvironment(prefs)

	if err != nil {
		return nil, []error{err}
	}

	err = applyPreferencesFromArgs(prefs)

	if err != nil {
		return nil, []error{err}
	}

	fingerprint, err := getFingerprint(prefs.Key + ".pub")

	if err == nil {
		prefs.Fingerprint = fingerprint
	}

	errs := prefs.Validate(dataDir, true)

	if len(errs) != 0 {
		return nil, errs
	}

	return prefs, nil
}

// ////////////////////////////////////////////////////////////////////////////////// //

// applyPreferencesFromFile read arguments from file and add it to preferences struct
func applyPreferencesFromFile(prefs *Preferences, file string) error {
	data, err := ioutil.ReadFile(file)

	if err != nil {
		return err
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
		case TTL:
			prefs.TTL = timeutil.ParseDuration(propVal) / 60

			if prefs.TTL == 0 {
				return fmt.Errorf("Incorrect ttl property in %s file", file)
			}

		case MAX_WAIT, "max_wait", "maxwait":
			prefs.MaxWait = timeutil.ParseDuration(propVal) / 60

			if prefs.MaxWait == 0 {
				return fmt.Errorf("Incorrect max-wait property in %s file", file)
			}

		case OUTPUT:
			prefs.Output = propVal

		case TOKEN:
			prefs.Token = propVal

		case KEY:
			prefs.Key = propVal

		case REGION:
			prefs.Region = propVal

		case NODE_SIZE, "node_size", "nodesize":
			prefs.NodeSize = propVal

		case USER:
			prefs.User = propVal

		case TEMPLATE:
			prefs.Template = propVal

		default:
			return fmt.Errorf("Unknown property %s in %s file", propName, file)
		}
	}

	return nil
}

// applyPreferencesFromArgs add values from command-line arguments to preferences struct
func applyPreferencesFromArgs(prefs *Preferences) error {
	if arg.Has(ARG_TTL) {
		prefs.TTL = timeutil.ParseDuration(arg.GetS(ARG_TTL)) / 60

		if prefs.TTL == 0 {
			return fmt.Errorf("Incorrect ttl property in command-line arguments")
		}
	}

	if arg.Has(ARG_MAX_WAIT) {
		prefs.MaxWait = timeutil.ParseDuration(arg.GetS(ARG_MAX_WAIT)) / 60

		if prefs.MaxWait == 0 {
			return fmt.Errorf("Incorrect max-wait property in command-line arguments")
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

	return nil
}

func applyPreferencesFromEnvironment(prefs *Preferences) error {
	var envMap = env.Get()

	if envMap[EV_TTL] != "" {
		prefs.TTL = timeutil.ParseDuration(envMap[EV_TTL]) / 60

		if prefs.TTL == 0 {
			return fmt.Errorf("Incorrect %s property in environment variables", EV_TTL)
		}
	}

	if envMap[EV_MAX_WAIT] != "" {
		prefs.MaxWait = timeutil.ParseDuration(envMap[EV_MAX_WAIT]) / 60

		if prefs.MaxWait == 0 {
			return fmt.Errorf("Incorrect %s property in environment variables", EV_TTL)
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

	return nil
}

// getFingerprint return fingerprint for public key
func getFingerprint(key string) (string, error) {
	data, err := ioutil.ReadFile(key)

	if err != nil {
		return "", err
	}

	pubkey, err := sshkey.UnmarshalPublicKey(string(data[:]))

	if err != nil {
		return "", err
	}

	return sshkey.PrettyFingerprint(pubkey, crypto.MD5)
}

// ////////////////////////////////////////////////////////////////////////////////// //

func (p *Preferences) Validate(dataDir string, allowEmptyTemplate bool) []error {
	var errs []error

	if p.Token == "" {
		errs = append(errs, fmt.Errorf("Property token must be set"))
	}

	if len(p.Token) != 64 {
		errs = append(errs, fmt.Errorf("Property token must is misformatted"))
	}

	if p.Region == "" {
		errs = append(errs, fmt.Errorf("Property region must be set"))
	}

	if p.NodeSize == "" {
		errs = append(errs, fmt.Errorf("Property node-size must be set"))
	}

	if p.User == "" {
		errs = append(errs, fmt.Errorf("Property user must be set"))
	}

	if p.Key == "" {
		errs = append(errs, fmt.Errorf("Property key must be set"))
	} else {
		if !fsutil.IsExist(p.Key) {
			errs = append(errs, fmt.Errorf("Private key file %s does not exits", p.Key))
		}

		if !fsutil.IsReadable(p.Key) {
			errs = append(errs, fmt.Errorf("Private key file %s must be readable", p.Key))
		}

		if !fsutil.IsNonEmpty(p.Key) {
			errs = append(errs, fmt.Errorf("Private key file %s does not contain any data", p.Key))
		}

		if !fsutil.IsExist(p.Key + ".pub") {
			errs = append(errs, fmt.Errorf("Public key file %s.pub does not exits", p.Key))
		}

		if !fsutil.IsReadable(p.Key + ".pub") {
			errs = append(errs, fmt.Errorf("Public key file %s.pub must be readable", p.Key))
		}

		if !fsutil.IsNonEmpty(p.Key + ".pub") {
			errs = append(errs, fmt.Errorf("Public key file %s.pub does not contain any data", p.Key))
		}
	}

	if p.Template == "" && !allowEmptyTemplate {
		errs = append(errs, fmt.Errorf("You must define template name"))
	} else {
		templateDir := dataDir + "/" + p.Template

		if !fsutil.IsExist(templateDir) {
			errs = append(errs, fmt.Errorf("Directory with template %s is not exist", p.Template))
		} else {
			if !fsutil.IsReadable(templateDir) {
				errs = append(errs, fmt.Errorf("Directory with template %s is not readable", p.Template))
			}

			if fsutil.IsDir(templateDir) {
				if fsutil.IsEmptyDir(templateDir) {
					errs = append(errs, fmt.Errorf("Directory with template %s is empty", p.Template))
				}
			} else {
				errs = append(errs, fmt.Errorf("Target %s is not a directory", templateDir))
			}
		}
	}

	return errs
}

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
