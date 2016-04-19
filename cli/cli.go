package cli

// ////////////////////////////////////////////////////////////////////////////////// //
//                                                                                    //
//                     Copyright (c) 2009-2016 Essential Kaos                         //
//      Essential Kaos Open Source License <http://essentialkaos.com/ekol?en>         //
//                                                                                    //
// ////////////////////////////////////////////////////////////////////////////////// //

import (
	"bufio"
	"crypto"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"pkg.re/essentialkaos/ek.v1/arg"
	"pkg.re/essentialkaos/ek.v1/env"
	"pkg.re/essentialkaos/ek.v1/fmtc"
	"pkg.re/essentialkaos/ek.v1/fmtutil"
	"pkg.re/essentialkaos/ek.v1/fsutil"
	"pkg.re/essentialkaos/ek.v1/path"
	"pkg.re/essentialkaos/ek.v1/timeutil"
	"pkg.re/essentialkaos/ek.v1/usage"

	"gopkg.in/hlandau/passlib.v1/hash/sha2crypt"

	sshkey "github.com/yosida95/golang-sshkey"
)

// ////////////////////////////////////////////////////////////////////////////////// //

const (
	APP  = "Terrafarm"
	VER  = "0.3.1"
	DESC = "Utility for working with terraform based rpmbuilder farm"
)

const (
	ARG_TTL       = "t:ttl"
	ARG_OUTPUT    = "o:output"
	ARG_TOKEN     = "T:token"
	ARG_KEY       = "K:key"
	ARG_REGION    = "R:region"
	ARG_NODE_SIZE = "N:node-size"
	ARG_USER      = "U:user"
	ARG_PASSWORD  = "P:password"
	ARG_DEBUG     = "D:debug"
	ARG_MONITOR   = "m:monitor"
	ARG_NO_COLOR  = "nc:no-color"
	ARG_HELP      = "h:help"
	ARG_VER       = "v:version"
)

const (
	CMD_CREATE  = "create"
	CMD_DESTROY = "destroy"
	CMD_SHOW    = "show"
	CMD_PLAN    = "plan"
)

// SRC_DIR is path to directory with terrafarm sources
const SRC_DIR = "github.com/essentialkaos/terrafarm"

// ////////////////////////////////////////////////////////////////////////////////// //

// argMap is map with supported command-line arguments
var argMap = arg.Map{
	ARG_TTL:       &arg.V{},
	ARG_OUTPUT:    &arg.V{},
	ARG_TOKEN:     &arg.V{},
	ARG_KEY:       &arg.V{},
	ARG_REGION:    &arg.V{},
	ARG_NODE_SIZE: &arg.V{},
	ARG_USER:      &arg.V{},
	ARG_DEBUG:     &arg.V{Type: arg.BOOL},
	ARG_MONITOR:   &arg.V{Type: arg.INT},
	ARG_NO_COLOR:  &arg.V{Type: arg.BOOL},
	ARG_HELP:      &arg.V{Type: arg.BOOL, Alias: "u:usage"},
	ARG_VER:       &arg.V{Type: arg.BOOL, Alias: "ver"},
}

// depList is slice with dependencies required by terrafarm
var depList = []string{
	"terraform",
	"terraform-provider-digitalocean",
	"terraform-provisioner-file",
	"terraform-provisioner-remote-exec",
}

// envMap is map with environment variables
var envMap = env.Get()

// ////////////////////////////////////////////////////////////////////////////////// //

func Init() {
	runtime.GOMAXPROCS(1)

	args, errs := arg.Parse(argMap)

	if len(errs) != 0 {
		fmtc.NewLine()

		for _, err := range errs {
			fmtc.Printf("{r}%v{!}\n", err)
		}

		os.Exit(1)
	}

	if arg.GetB(ARG_NO_COLOR) {
		fmtc.DisableColors = true
	}

	if arg.GetB(ARG_VER) {
		showAbout()
		return
	}

	if arg.GetB(ARG_HELP) {
		showUsage()
		return
	}

	if !arg.Has(ARG_MONITOR) && len(args) == 0 {
		showUsage()
		return
	}

	checkEnv()
	checkDeps()

	if arg.Has(ARG_MONITOR) {
		startMonitor()
	} else {
		cmd := args[0]
		processCommand(cmd)
	}
}

// checkEnv check system environment
func checkEnv() {
	if envMap["GOPATH"] == "" {
		fmtc.Println("{r}GOPATH must be set to valid path{!}")
		os.Exit(1)
	}

	srcDir := getSrcDir()

	if !fsutil.CheckPerms("DRW", srcDir) {
		fmtc.Printf("{r}Source directory %s is not accessible{!}\n", srcDir)
		os.Exit(1)
	}
}

// checkDeps check required dependecies
func checkDeps() {
	hasErrors := false

	for _, dep := range depList {
		if env.Which(dep) == "" {
			fmtc.Printf("{r}Can't find %s. Please install it first.{!}\n", dep)
			hasErrors = true
		}
	}

	if hasErrors {
		os.Exit(1)
	}
}

// startMonitor starts monitoring process
func startMonitor() {
	destroyTime := time.Unix(int64(arg.GetI(ARG_MONITOR)), 0)

	for {
		if !isTerrafarmActive() {
			os.Exit(0)
		}

		time.Sleep(time.Minute)

		if time.Now().Unix() <= destroyTime.Unix() {
			continue
		}

		prefs := findAndReadPrefs()
		vars, err := prefsToArgs(prefs)

		if err != nil {
			continue
		}

		vars = append(vars, "-force")

		err = execTerraformSync("destroy", vars)

		if err != nil {
			continue
		}

		os.Exit(0)
	}
}

// processCommand execute some command
func processCommand(cmd string) {
	prefs := findAndReadPrefs()

	switch cmd {
	case CMD_CREATE, "apply", "start":
		createCommand(prefs)
	case CMD_DESTROY, "delete", "stop":
		destroyCommand(prefs)
	case CMD_PLAN:
		planCommand(prefs)
	case CMD_SHOW, "info":
		showCommand()
	default:
		fmtc.Printf("{r}Unknown command %s\n", cmd)
		os.Exit(1)
	}
}

// createCommand is create command handler
func createCommand(prefs *Prefs) {
	if isTerrafarmActive() {
		fmtc.Println("{y}Terrafarm already works{!}")
		os.Exit(1)
	}

	printTerrafarmInfo(prefs)

	time.Sleep(3 * time.Second)

	vars, err := prefsToArgs(prefs)

	if err != nil {
		fmtc.Printf("{r}Can't parse prefs: %v{!}\n", err)
		os.Exit(1)
	}

	if arg.GetB(ARG_DEBUG) {
		fmtc.Printf("{s}EXEC → terraform apply %s{!}\n\n", strings.Join(vars, " "))
	}

	err = execTerraform("apply", vars)

	if err != nil {
		fmtc.Printf("{r}Error while executing terraform: %v\n{!}", err)
		os.Exit(1)
	}

	fmtutil.Separator(false)

	if prefs.Output != "" {
		fmtc.Println("Exporting info about build nodes...")

		err = exportNodeList(prefs)

		if err != nil {
			fmtc.Printf("{r}Error while exporting info: %v\n", err)
		} else {
			fmtc.Printf("{g}Info about build nodes saved as %s{!}\n", prefs.Output)
		}

		fmtutil.Separator(false)
	}

	if prefs.TTL > 0 {
		fmtc.Println("Starting monitoring process...")

		err = runMonitor(prefs.TTL)

		if err != nil {
			fmtc.Printf("{r}Error while starting monitoring process: %v\n", err)
			os.Exit(1)
		}

		fmtc.Println("{g}Monitoring process succefully started!{!}")

		fmtutil.Separator(false)
	}
}

// destroyCommand is destroy command handler
func destroyCommand(prefs *Prefs) {
	if !isTerrafarmActive() {
		fmtc.Println("{y}Terrafarm does not works, nothing to destroy{!}")
		os.Exit(1)
	}

	vars, err := prefsToArgs(prefs)

	if err != nil {
		fmtc.Printf("{r}Can't parse prefs: %v{!}\n", err)
		os.Exit(1)
	}

	fmtutil.Separator(false)

	vars = append(vars, "-force")

	if arg.GetB(ARG_DEBUG) {
		fmtc.Printf("{s}EXEC → terraform destroy %s{!}\n\n", strings.Join(vars, " "))
	}

	err = execTerraform("destroy", vars)

	if err != nil {
		fmtc.Printf("{r}Error while executing terraform: %v\n{!}", err)
		os.Exit(1)
	}

	fmtutil.Separator(false)
}

// showCommand is show command handler
func showCommand() {
	fmtutil.Separator(false)

	err := execTerraform("show", nil)

	if err != nil {
		fmtc.Printf("{r}Error while executing terraform: %v\n{!}", err)
		os.Exit(1)
	}

	fmtutil.Separator(false)
}

// planCommand is plan command handler
func planCommand(prefs *Prefs) {
	printTerrafarmInfo(prefs)

	time.Sleep(3 * time.Second)

	vars, err := prefsToArgs(prefs)

	if err != nil {
		fmtc.Printf("{r}Can't parse prefs: %v{!}\n", err)
		os.Exit(1)
	}

	if arg.GetB(ARG_DEBUG) {
		fmtc.Printf("{s}EXEC → terraform plan %s{!}\n\n", strings.Join(vars, " "))
	}

	err = execTerraform("plan", vars)

	if err != nil {
		fmtc.Printf("{r}Error while executing terraform: %v\n{!}", err)
		os.Exit(1)
	}

	fmtutil.Separator(false)
}

// printTerrafarmInfo print info from prefs
func printTerrafarmInfo(prefs *Prefs) {
	fmtutil.Separator(false, "TERRAFARM")

	fmtc.Printf("  {*}%-16s{!} %s\n", "Token:", getMaskedToken(prefs.Token))

	fmtc.Printf("  {*}%-16s{!} %s\n", "Private Key:", prefs.Key)
	fmtc.Printf("  {*}%-16s{!} %s\n", "Public Key:", prefs.Key+".pub")

	fingerprint, _ := getFingerprint(prefs.Key + ".pub")

	fmtc.Printf("  {*}%-16s{!} %s\n", "Fingerprint:", fingerprint)

	switch {
	case prefs.TTL <= 0:
		fmtc.Printf("  {*}%-16s{!} {r}disabled{!}\n", "TTL:")
	case prefs.TTL > 360:
		fmtc.Printf("  {*}%-16s{!} {r}%s{!}\n", "TTL:", timeutil.PrettyDuration(prefs.TTL*60))
	case prefs.TTL > 120:
		fmtc.Printf("  {*}%-16s{!} {y}%s{!}\n", "TTL:", timeutil.PrettyDuration(prefs.TTL*60))
	default:
		fmtc.Printf("  {*}%-16s{!} %s\n", "TTL:", timeutil.PrettyDuration(prefs.TTL*60))
	}

	if prefs.Output != "" {
		fmtc.Printf("  {*}%-16s{!} %s\n", "Output:", prefs.Output)
	}

	if prefs.Region != "" {
		fmtc.Printf("  {*}%-16s{!} %s\n", "Region:", prefs.Region)
	}

	if prefs.NodeSize != "" {
		fmtc.Printf("  {*}%-16s{!} %s\n", "Node size:", prefs.NodeSize)
	}

	if prefs.User != "" {
		fmtc.Printf("  {*}%-16s{!} %s\n", "User:", prefs.User)
	}

	if isTerrafarmActive() {
		fmtc.Printf("  {*}%-16s{!} {g}works{!}\n", "State:")
	} else {
		fmtc.Printf("  {*}%-16s{!} {y}stopped{!}\n", "State:")
	}

	fmtutil.Separator(false)
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

// getPasswordHash return shadow file compatible hash
func getPasswordHash(password string) (string, error) {
	return sha2crypt.NewCrypter512(5000).Hash(password)
}

// getMaskedToken return first and last 8 symbols of token
func getMaskedToken(token string) string {
	if len(token) <= 16 {
		return ""
	}

	return token[:8] + "..." + token[len(token)-8:]
}

// prefsToArgs return prefs as command line arguments for terraform
func prefsToArgs(prefs *Prefs) ([]string, error) {
	auth, err := getPasswordHash(prefs.Password)

	if err != nil {
		return nil, err
	}

	fingerpint, err := getFingerprint(prefs.Key + ".pub")

	if err != nil {
		return nil, err
	}

	var vars []string

	vars = append(vars, fmtc.Sprintf("-var token=%s", prefs.Token))
	vars = append(vars, fmtc.Sprintf("-var auth=%s", auth))
	vars = append(vars, fmtc.Sprintf("-var fingerprint=%s", fingerpint))
	vars = append(vars, fmtc.Sprintf("-var key=%s", prefs.Key))
	vars = append(vars, fmtc.Sprintf("-var user=%s", prefs.User))
	vars = append(vars, fmtc.Sprintf("-var password=%s", prefs.Password))

	if prefs.Region != "" {
		vars = append(vars, fmtc.Sprintf("-var region=%s", prefs.Region))
	}

	if prefs.NodeSize != "" {
		vars = append(vars, fmtc.Sprintf("-var node_size=%s", prefs.NodeSize))
	}

	return vars, nil
}

// exec execute command
func execTerraform(command string, args []string) error {
	fsutil.Push(getDataDir())

	cmd := exec.Command("terraform", command)

	if len(args) != 0 {
		cmd.Args = append(cmd.Args, strings.Split(strings.Join(args, " "), " ")...)
	}

	r, err := cmd.StdoutPipe()

	if err != nil {
		return fmtc.Errorf("Can't redirect output: %v", err)
	}

	s := bufio.NewScanner(r)

	go func() {
		for s.Scan() {
			fmtc.Printf("  %s\n", s.Text())
		}
	}()

	err = cmd.Start()

	if err != nil {
		return fmtc.Errorf("Can't start terraform: %v", err)
	}

	err = cmd.Wait()

	if err != nil {
		return fmtc.Errorf("Can't process terraform output: %v", err)
	}

	fsutil.Pop()

	return nil
}

// execTerraformSync start terrafrom command synchronously
func execTerraformSync(command string, args []string) error {
	fsutil.Push(getDataDir())

	cmd := exec.Command("terraform", command)

	if len(args) != 0 {
		cmd.Args = append(cmd.Args, strings.Split(strings.Join(args, " "), " ")...)
	}

	err := cmd.Run()

	fsutil.Pop()

	return err
}

// isTerrafarmActive return true if terrafarm already active
func isTerrafarmActive() bool {
	stateFile := getStateFilePath()

	if !fsutil.IsExist(stateFile) {
		return false
	}

	state, err := readTFState(stateFile)

	if err != nil {
		return true
	}

	return len(state.Modules[0].Resources) != 0
}

// getDataDir return path to directory with terraform data
func getDataDir() string {
	return path.Join(getSrcDir(), "terradata")
}

// getSrcDir return path to directory with terrafarm sources
func getSrcDir() string {
	return path.Join(envMap["GOPATH"], "src", SRC_DIR)
}

// getStateFilePath return path to terraform state file
func getStateFilePath() string {
	return path.Join(getDataDir(), "terraform.tfstate")
}

// runMonitor run monitoring process
func runMonitor(ttl int64) error {
	destroyTime := time.Now().Unix() + (ttl * 60)

	cmd := exec.Command("terrafarm", "--monitor", fmtc.Sprintf("%d", destroyTime))

	return cmd.Start()
}

// exportNodeList exports info about nodes for usage in rpmbuilder
func exportNodeList(prefs *Prefs) error {
	if fsutil.IsExist(prefs.Output) {
		if fsutil.IsDir(prefs.Output) {
			return fmtc.Errorf("Output path must be path to file")
		}

		if !fsutil.IsWritable(prefs.Output) {
			return fmtc.Errorf("Output path must be path to writable file")
		}

		os.Remove(prefs.Output)
	}

	fd, err := os.OpenFile(prefs.Output, os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		return err
	}

	defer fd.Close()

	state, err := readTFState(getStateFilePath())

	if err != nil {
		return fmtc.Errorf("Can't read state file: %v", err)
	}

	for _, node := range state.Modules[0].Resources {
		nodeRec := fmtc.Sprintf(
			"%s:%s@%s",
			prefs.User,
			prefs.Password,
			node.Info.Attributes.IP,
		)

		switch {
		case strings.HasSuffix(node.Info.Attributes.Name, "-x32"):
			nodeRec += "~i386"
		case strings.HasSuffix(node.Info.Attributes.Name, "-x48"):
			nodeRec += "~i686"
		}

		fmtc.Fprintln(fd, nodeRec)
	}

	return nil
}

// ////////////////////////////////////////////////////////////////////////////////// //

// showUsage show help content
func showUsage() {
	info := usage.NewInfo("")

	info.AddCommand(CMD_CREATE, "Create and run farm droplets on DigitalOcean")
	info.AddCommand(CMD_DESTROY, "Destroy farm droplets on DigitalOcean")
	info.AddCommand(CMD_SHOW, "Show info about droplets in farm")
	info.AddCommand(CMD_PLAN, "Show execution plan")

	info.AddOption(ARG_TTL, "Max farm TTL (Time To Live)", "ttl")
	info.AddOption(ARG_OUTPUT, "Path to output file with access credentials", "file")
	info.AddOption(ARG_TOKEN, "DigitalOcean token", "token")
	info.AddOption(ARG_KEY, "Path to private key", "key-file")
	info.AddOption(ARG_REGION, "DigitalOcean region", "region")
	info.AddOption(ARG_NODE_SIZE, "Droplet size on DigitalOcean", "size")
	info.AddOption(ARG_USER, "Build node user name", "username")
	info.AddOption(ARG_NO_COLOR, "Disable colors in output")
	info.AddOption(ARG_HELP, "Show this help message")
	info.AddOption(ARG_VER, "Show version")

	info.AddExample(CMD_PLAN, "Show build plan")
	info.AddExample(CMD_PLAN+" --node-size 8gb --ttl 3h", "Show build plan with redefined node size and TTL")
	info.AddExample(CMD_CREATE+" --ttl 45m", "Run farm with 45 min TTL")
	info.AddExample(CMD_DESTROY, "Destory all farm nodes")
	info.AddExample(CMD_SHOW, "Show info about build nodes")

	info.Render()
}

// showAbout show info about utility
func showAbout() {
	about := &usage.About{
		App:     APP,
		Version: VER,
		Desc:    DESC,
		Year:    2006,
		Owner:   "ESSENTIAL KAOS",
		License: "Essential Kaos Open Source License <https://essentialkaos.com/ekol?en>",
	}

	about.Render()
}
